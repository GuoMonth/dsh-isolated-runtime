import fs from 'node:fs/promises';
import path from 'node:path';
import crypto from 'node:crypto';
import * as tar from 'tar';

export function targetFor(platform, arch) {
  const target = `${platform}-${arch === 'x64' ? 'amd64' : arch}`;
  if (!['linux-amd64', 'darwin-arm64'].includes(target)) throw new Error(`Unsupported host ${target}; use Linux x86_64 or native Apple Silicon macOS.`);
  return target;
}

export function validateRelease(release, version, target) {
  if (release.schemaVersion !== 2 || release.version !== `v${version}` || !/^[a-f0-9]{40}$/.test(release.sourceSHA)) throw new Error('Launcher/release identity mismatch');
  for (const type of ['cell', 'operator']) {
    if (!new RegExp(`^ghcr.io/guomonth/dsh-isolated-runtime-${type}@sha256:[a-f0-9]{64}$`).test(release.images?.[type])) throw new Error('Invalid image identity');
  }
  const asset = release.packages?.[target];
  if (asset?.archive !== `dsh-isolated-runtime-v${version}-${target}.tar.gz` || !/^[a-f0-9]{64}$/.test(asset.archiveSHA256)) throw new Error('Invalid archive identity');
  return asset;
}

export async function download(url, destination, expected, fetcher = fetch) {
  for (let attempt = 0; attempt < 3; attempt++) {
    let file;
    try {
      const response = await fetcher(url, {signal: AbortSignal.timeout(120_000)});
      if (!response.ok || !response.body) throw new Error(`Download HTTP ${response.status}: ${url}`);
      file = await fs.open(destination, 'w', 0o600);
      const hash = crypto.createHash('sha256'); let size = 0;
      for await (const chunk of response.body) {
        size += chunk.length;
        if (size > 64 * 1024 * 1024) throw new Error('Archive exceeds download limit');
        hash.update(chunk); await file.writeFile(chunk);
      }
      await file.close(); file = undefined;
      if (hash.digest('hex') !== expected) throw new Error('Archive checksum mismatch');
      return;
    } catch (error) {
      await file?.close(); await fs.rm(destination, {force: true});
      if (attempt === 2 || /checksum|limit/.test(error.message)) throw error;
      await new Promise(resolve => setTimeout(resolve, 1000 * (attempt + 1)));
    }
  }
}

export async function unpack(archive, destination, folder) {
  let entries = 0; let size = 0;
  const seen = new Set();
  tar.t({file: archive, sync: true, strict: true, onReadEntry(entry) {
    const name = entry.path.replace(/\/$/, '');
    if (++entries > 10_000 || (size += entry.size) > 256 * 1024 * 1024 ||
        !['File', 'Directory'].includes(entry.type) || name.includes('\\') ||
        name.split('/').some(part => ['..', '.', ''].includes(part)) ||
        !(name === folder || name.startsWith(`${folder}/`)) || seen.has(name)) {
      throw new Error(`Unsafe archive entry: ${entry.path}`);
    }
    seen.add(name);
  }});
  await tar.x({file: archive, cwd: destination, strict: true, noChmod: true, noMtime: true});
}

export async function install(release, version, target, cache, fetcher = fetch) {
  const asset = validateRelease(release, version, target);
  await fs.mkdir(cache, {recursive: true, mode: 0o700});
  // Each invocation gets a private extraction; concurrent invocations cannot
  // expose a half-installed executable. The runtime itself serializes mutations.
  const work = await fs.mkdtemp(path.join(cache, 'launch-'));
  try {
    const archive = path.join(work, asset.archive);
    const cached = path.join(cache, asset.archiveSHA256 + '.tar.gz');
    let valid = false;
    try { valid = crypto.createHash('sha256').update(await fs.readFile(cached)).digest('hex') === asset.archiveSHA256; } catch {}
    if (valid) await fs.copyFile(cached, archive);
    else {
      await download(`https://github.com/GuoMonth/dsh-isolated-runtime/releases/download/${release.version}/${asset.archive}`, archive, asset.archiveSHA256, fetcher);
      const staging = path.join(work, 'cache-copy');
      await fs.copyFile(archive, staging); await fs.rename(staging, cached);
    }
    const folder = asset.archive.replace(/\.tar\.gz$/, '');
    await unpack(archive, work, folder);
    const directory = path.join(work, folder);
    const inner = JSON.parse(await fs.readFile(path.join(directory, 'release.json'), 'utf8'));
    if (inner.version !== release.version || inner.sourceSHA !== release.sourceSHA || inner.packagePlatform !== target.replace('-', '/') ||
        JSON.stringify(inner.images) !== JSON.stringify(release.images)) throw new Error('Extracted release identity mismatch');
    await fs.chmod(path.join(directory, 'dsh-runtime'), 0o700);
    return {directory, cleanup: () => fs.rm(work, {recursive: true, force: true})};
  } catch (error) { await fs.rm(work, {recursive: true, force: true}); throw error; }
}
