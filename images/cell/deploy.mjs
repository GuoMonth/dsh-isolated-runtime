// Materialize the upstream Python runtime closure, following upstream's deploy
// route. No workspace/source paths may be needed by the final image.
import fs from 'node:fs';
import path from 'node:path';
const [source, target] = process.argv.slice(2);
if (!source || !target) throw new Error('usage: deploy.mjs SOURCE TARGET');
const modules = path.join(target, 'node_modules');
const manifest = JSON.parse(fs.readFileSync(path.join(target, 'package.json')));
for (const name of Object.keys(manifest.dependencies).sort()) {
  const destination = path.join(modules, name);
  if (fs.existsSync(destination)) continue;
  const from = path.join(source, 'python/sdk-runtime/node_modules', name);
  if (!fs.existsSync(from)) throw new Error(`upstream deploy omitted ${name}`);
  fs.cpSync(from, destination, {recursive: true, dereference: true,
    filter: p => !p.startsWith(path.join(from, 'node_modules'))});
}
function materialize(directory) {
  for (const entry of fs.readdirSync(directory, {withFileTypes: true})) {
    const file = path.join(directory, entry.name);
    if (entry.name === '.bin') { fs.rmSync(file, {recursive:true,force:true}); continue; }
    if (entry.isSymbolicLink()) {
      if (entry.name === '.bin') { fs.rmSync(file, {recursive:true, force:true}); continue; }
      const original = fs.realpathSync(file);
      fs.unlinkSync(file);
      fs.cpSync(original, file, {recursive: true, dereference: true,
        filter: p => p !== path.join(original, 'node_modules') && !p.startsWith(path.join(original, 'node_modules') + path.sep)});
    } else if (entry.isDirectory()) materialize(file);
  }
}
materialize(modules);
// The upstream legacy deploy can omit newly added transitive workspace peers
// from its explicit Python closure. Complete those from this exact source tree,
// never from an older npm release or a second version of the framework.
const workspace = new Map();
for (const file of fs.globSync(['packages/*/*/package.json','vendor/*/package.json','apps/*/package.json'],{cwd:source})) {
  const directory=path.dirname(path.join(source,file));
  const pkg=JSON.parse(fs.readFileSync(path.join(directory,'package.json')));
  workspace.set(pkg.name,directory);
}
const checked=new Set();
function complete(name) {
  if(checked.has(name))return;checked.add(name);
  const destination=path.join(modules,name);
  if(!fs.existsSync(destination) && workspace.has(name)) {
    const from=workspace.get(name);
    fs.cpSync(from,destination,{recursive:true,dereference:true,filter:p=>!p.startsWith(path.join(from,'node_modules'))});
  }
  const file=path.join(destination,'package.json');
  if(!fs.existsSync(file))return;
  const pkg=JSON.parse(fs.readFileSync(file));
  for(const dependency of Object.keys({...pkg.dependencies,...pkg.peerDependencies})) if(workspace.has(dependency)) complete(dependency);
}
for(const name of Object.keys(manifest.dependencies))complete(name);
// Use the Node entry directly; copying pnpm's shell shim would retain its
// build-time paths. This is also the entry used by upstream packed-install tests.
