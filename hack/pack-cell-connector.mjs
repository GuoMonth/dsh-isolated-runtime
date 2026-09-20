import { execFileSync } from "node:child_process";
import { mkdirSync, writeFileSync, copyFileSync } from "node:fs";
import { resolve, dirname } from "node:path";
import { fileURLToPath } from "node:url";

const root = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const output = process.argv[2];
if (!output)
  throw new Error("Usage: node hack/pack-cell-connector.mjs OUTPUT_DIRECTORY");
const run = (command, args, cwd = root) =>
  execFileSync(command, args, { cwd, encoding: "utf8" });
if (
  run("git", [
    "status",
    "--porcelain",
    "--untracked-files=all",
    "--",
    "packages/cell-connector",
    "hack/pack-cell-connector.mjs",
    "LICENSE",
  ]).trim()
)
  throw new Error("Commit connector inputs before packing");
const commit = run("git", ["rev-parse", "HEAD"]).trim();
const directory = resolve(root, "packages/cell-connector");
run("npm", ["ci", "--ignore-scripts"], directory);
run("npm", ["run", "build"], directory);
copyFileSync(resolve(root, "LICENSE"), resolve(directory, "LICENSE"));
writeFileSync(
  resolve(directory, "source.json"),
  JSON.stringify(
    { repository: "GuoMonth/dsh-isolated-runtime", commit },
    null,
    2,
  ) + "\n",
);
mkdirSync(resolve(output), { recursive: true });
process.stdout.write(
  run(
    "npm",
    [
      "pack",
      "--ignore-scripts",
      "--pack-destination",
      resolve(output),
      "--json",
    ],
    directory,
  ),
);
