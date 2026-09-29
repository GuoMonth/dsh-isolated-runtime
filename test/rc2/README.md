# RC2 G1 real distribution smoke

This is an A-stage sample, not the production create/stop/start implementation.
The latter is B's AgentEnvironment adapter. No model network request or real
external-tool authorization is claimed here.

1. Install the exact npm closure with `npm ci --prefix images/environment`.
   Before patching, run the browser case with `RC2_EXPECT_UNPATCHED=1` to reproduce
   remote Models being unavailable. Then run
   `node compat/dsh/apply-patches.mjs "$PWD/images/environment"` (once per clean install).
2. Start the test-only TLS carrier with `dev-run go=1.27 -- go run ./test/rc2`.
   Set `DSH_REAL_CLI` to the installed `@deepseek-ai/dsh/lib/bin.js`, `RC2_DATA`
   to a fresh private temp directory, `RC2_URL_FILE` to a private output filename,
   and `RC2_PATCH` to the absolute `images/environment/environment.patch.yml`.
   The carrier owns its launcher child and stops it on SIGTERM.
3. Install Playwright 1.58.2 separately if needed, always with
   `PLAYWRIGHT_SKIP_BROWSER_GC=1`. Run `node test/rc2/browser.cjs` with the same
   `RC2_URL_FILE` and `PLAYWRIGHT_MODULE` pointing to Playwright's installed module.
   It visits real `environment.test` through Chromium host resolution and a
   test-only self-signed TLS carrier, writes a deliberately non-functional key
   through DSH's own onboarding UI, edits native model configuration, creates a
   Session and verifies a native WebSocket snapshot. No credential value is logged.
4. Restart the carrier on the same data root and rerun. Inspect file existence
   under dsh/home/workspace without printing `.credentials.yaml` contents.

For a cluster sample, build `images/environment/Dockerfile` for linux/amd64,
import its exact manifest into the dedicated kind cluster (local only), and use
`KUBECONFIG=... RC2_IMAGE=...@sha256:... RC2_EVIDENCE=/private/path.json node test/rc2/sandbox.mjs`.
The script creates only the new `dsh-mvp-a-smoke` namespace (override `RC2_NAMESPACE`
for another fresh sample), an unprivileged SA, one PVC and one upstream Sandbox.
It consumes the shipped template function directly. It deliberately fails on
existing names, has no cleanup, and never deletes data. Core controller/CRD must
already be installed from the pinned v1.0.3 release; only the current cluster
write owner may install it. Wait for Sandbox Ready, then verify the actual Pod.

This minimal sample has no NetworkPolicy and does not prove cross-user isolation;
B owns the production policy and E proves platform OIDC/isolation. Do not expose
its application outside the dedicated test cluster. A's detailed results are in
`docs/evidence/mvp-a-rc2-2026-09-29.md`.
