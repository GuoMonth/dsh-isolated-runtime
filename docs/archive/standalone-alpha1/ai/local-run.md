# AI Installation Runbook

Scope: v0.2.0-alpha.1, Linux x86_64 and native Apple Silicon macOS. This source
branch prepares an alpha; verify that the version is actually published before
attempting to download it. Do not substitute v0.1.2 commands or assume a package
name is owned merely because it exists in npm search results.

## Authority and Boundaries

Use the selected release's bundled docs, release.json and checksums. main may
describe commands not present in the installed version. Product source:
https://github.com/GuoMonth/dsh-isolated-runtime

- Do not ask for API keys, passwords or cookies in the conversation.
- Do not run credentials or read credentials.json unless the user explicitly
  needs that local action; never relay the secret in chat, logs or an issue.
- Do not install Docker, escalate privileges, change system trust/hosts, or
  operate an existing Kubernetes context without explicit approval.
- Never run uninstall to fix a failed start. It deletes the cluster AND data.
- Do not claim a real model or Mac Docker Desktop was tested from fixture CI.
- This is a local alpha, not a production deployment or automatic upgrader.

## Preflight

1. Confirm OS/architecture and Docker availability. For npm, require Node 22+.
   Direct archive installation does not require a preinstalled Node runtime.
2. Require a running native Linux Docker engine. The reference allocation is
   4 CPUs / 16 GiB RAM, not a guaranteed minimum. Check available disk space
   and explain that images, tools and Chromium require additional downloads.
3. Read the network/source list in https://github.com/GuoMonth/dsh-isolated-runtime/blob/v0.2.0-alpha.1/docs/distribution.md. Do not disable TLS
   checks or substitute arbitrary public registry proxies when downloads fail.
4. Ask whether an existing installation or data must be preserved. Respect
   DSH_RUNTIME_HOME, XDG_STATE_HOME and legacy DSH_DEMO_HOME. Never repoint a
   new release at old state to bypass the release identity guard.
5. Prefer an explicit version, not a moving npm tag, for reproducible support.
   Inspect npm metadata and confirm it links to this repository before use.

## Install and Start

After publication, the npm entry is:

```sh
npx dsh-isolated-runtime@0.2.0-alpha.1 start --no-open
```

When using npm, replace every ./dsh-runtime prefix below with
`npx dsh-isolated-runtime@0.2.0-alpha.1`; there is no executable installed
in the user's working directory. Do not assume a global npm installation.

Without npm, download the platform archive and SHA256SUMS from the official
GitHub Release, verify the archive before extracting, then run:

```sh
./dsh-runtime doctor --json
./dsh-runtime up
./dsh-runtime status --json
```

Choose --snapshots on the first up only if requested. The storage class cannot
be switched in place. The default installation creates a dedicated kind
cluster and private kubeconfig, not resources in the user's current context.

doctor returns JSON schemaVersion 1, checks [{name,ok}], and ok. Exit 0 means
its prerequisite checks pass, 1 means failed checks, 2 means invalid arguments.
It is not a proof of all network routes, storage capacity or resource adequacy.
status returns schemaVersion 1, state and ready. States: not-installed,
stopped, running, starting-or-unhealthy, unavailable. A running Cell must have
Ready=True and observedGeneration matching metadata.generation. Unavailable
ownership/API or an unhealthy running installation exits 1. status does not
prove a browser login or a real model request succeeded.

## User Acceptance

Open the UI with ./dsh-runtime open (requires a graphical desktop). The user
privately obtains their generated login with ./dsh-runtime credentials, then
enters the model key in DSH's native onboarding. Workspace:
/var/lib/dsh/data/workspace.

Have the user request a unique file creation and read-back. Then stop, resume
with up, open again and verify the same file and session. Record the installed
release, host architecture, result and redacted errors. Do not record keys or
the generated login. Browser opening alone is not end-to-end acceptance.

## Recovery and Cleanup

On failure, inspect the error and private runtime/objects.txt locally. Collect
only needed redacted facts. Credentials, Kubernetes Secrets, kubeconfig,
browser profiles and raw private state must not be uploaded as diagnostics.

- Docker unavailable: ask the user to start it; do not silently install it.
- Port conflict: identify the owner of 18443/15556; never kill unrelated work.
- Download failure: report the failing host and checksum/HTTP error, retain
  caches and retry; no insecure TLS, arbitrary mirror or skipped checksums.
- Release mismatch: use the old release to export data, or use separate state
  after stopping its browser and forwarding. No automatic migration/downgrade.
- Stop: ./dsh-runtime stop. Data retained; resume with up.
- Delete: only after explicit informed user confirmation, run
  ./dsh-runtime uninstall --yes. All owned cluster data, including Retain PVCs,
  is destroyed. Downloaded tools and archives remain cached.

## Suggested User Prompt

Read this version's AI installation runbook and help me run DSH Isolated Runtime
locally. Check prerequisites first, preserve existing data, and explain any
required system changes before making them. Let me enter credentials privately.
Verify readiness, then guide me through file write/read and stop/resume checks.
Do not uninstall or delete data without my explicit confirmation.
