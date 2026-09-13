# Local Installation

Target release: **v0.2.0-alpha.1**. This branch prepares the alpha; the commands
below that download it become available only after publication. Existing
v0.1.2 archives retain their original commands.

## Prerequisites

Linux x86_64 or Apple Silicon macOS with a native arm64 terminal. Install and
start Docker (Docker Desktop on Mac), and make it available to your user.
Bash, curl, tar, OpenSSL and Git are required; Linux also needs sha256sum/flock,
Mac uses shasum/Perl. A graphical desktop is needed only to open the browser.
The reference CI allocation is 4 CPUs / 16 GiB RAM, not a measured minimum.

You do not need a preinstalled Kubernetes cluster, kind, kubectl, Go or DSH.
Private tools and a separate kind cluster are managed by the installer. Ports
18443 and 15556 must be free. No system hosts or certificate trust changes.
Docker installation: https://docs.docker.com/get-started/get-docker/

## npm Entry

After npm publication, with Node.js 22+ and npm available:

```sh
npx dsh-isolated-runtime@0.2.0-alpha.1 start
```

Use `start --no-open` on headless hosts or `start --snapshots` to opt into
the reference CSI driver before creating the Cell. The thin launcher downloads
the exact checksum-bound release; it does not build images or install Docker.
The first start still requires network access to tools and image registries.
The npm name is proposed until publication permissions are confirmed.

For this entry, use the same npm prefix for later commands, in another terminal
while the browser is open: `npx dsh-isolated-runtime@0.2.0-alpha.1 credentials`,
`npx dsh-isolated-runtime@0.2.0-alpha.1 status --json`, or
`npx dsh-isolated-runtime@0.2.0-alpha.1 stop`. The ./dsh-runtime commands below
are the equivalent direct-download form.

## Direct Download

No host Node installation is required for this path. Download the matching
archive and SHA256SUMS from
[the release](https://github.com/GuoMonth/dsh-isolated-runtime/releases/tag/v0.2.0-alpha.1).
For Apple Silicon:

```sh
archive=dsh-isolated-runtime-v0.2.0-alpha.1-darwin-arm64.tar.gz
grep -F "  $archive" SHA256SUMS | shasum -a 256 -c -
tar -xzf "$archive"
cd "${archive%.tar.gz}"
./dsh-runtime doctor
./dsh-runtime up
./dsh-runtime open
```

Linux uses the `linux-amd64` archive and `sha256sum -c -`. Always verify
before extraction. `release.json` records source and exact image identities.

## First Login

Run `./dsh-runtime credentials` privately to see the generated local login.
Do not paste its output into an AI conversation or issue. The installation
generates an independent password and OIDC client secret, stored with private
permissions; Dex receives its configuration as a Kubernetes Secret.

In DSH's native onboarding, enter your own model API key. Choose workspace,
edit the path to `/var/lib/dsh/data/workspace`, then ask DSH to create and read
a file. Model charges belong to your account. Model credentials live in the
private volume, not in snapshots.

## Lifecycle and Data

```sh
./dsh-runtime status --json
./dsh-runtime doctor --json
./dsh-runtime stop
./dsh-runtime up
# Destructive: only after exporting needed data and explicitly deciding to delete.
./dsh-runtime uninstall --yes
```

`stop` stops browser/forwarding and the owned kind node without deleting data.
Closing the browser also retains data. `uninstall --yes` deletes the whole
owned cluster, including retained PVCs. It does not uninstall Docker or delete
other clusters. Downloaded tools and release cache remain reusable.

State defaults to `~/Library/Application Support/DSH Isolated Runtime` on Mac,
and `${XDG_STATE_HOME:-$HOME/.local/state}/dsh-isolated-runtime` on Linux.
An explicit XDG_STATE_HOME takes precedence on both. Set DSH_RUNTIME_HOME to
choose a directory; legacy DSH_DEMO_HOME is still accepted. Conflicting values
are rejected. Historical cluster/namespace identifiers remain for ownership
compatibility; they are not an instruction to discard user data.

Another release's state is never silently upgraded or removed. Export data
using that release before deciding whether to uninstall, or use a different
state directory after stopping the old installation. DSH V3 sessions cannot
be downgraded. Cross-version snapshot restore is unsupported.

## Advanced Installation

Existing clusters use `config/default`, `config/browser`, or
`config/snapshots`; administrators supply networking, storage, DNS, TLS, OIDC
and access grants. The local identity and local CA are not a production IdP/PKI.
See the [snapshot sample](../config/samples/dsh_v1alpha1_cellsnapshot.yaml).
The optional hostpath CSI driver is a local reference driver, not a backup
product. No k3d/k3s or Windows/Intel Mac installation guarantee in this alpha.

[AI runbook](ai/local-run.md) | [Distribution and network requirements](distribution.md)

Mac Docker Desktop end-to-end and real-model acceptance are maintainer follow-ups
after alpha publication, not implied by deterministic Linux CI success.
