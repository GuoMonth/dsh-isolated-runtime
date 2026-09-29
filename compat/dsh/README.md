# DSH RC2 compatibility

The exact source identity and npm integrity are in [baseline.json](baseline.json).
`images/environment/Dockerfile` installs the official `0.2.0-rc.2` distribution
with the committed npm lock; no upstream checkout, build:official or source deploy.

A real Chromium session at a remote hostname reproduced the native Models error
“settings are unavailable in this browser”. RC2 has no setting to select host
persistence for that hostname. `apply-patches.mjs` therefore changes one line of
the shipped ui-settings client module, checking the entire file SHA256 before
and after. It does not alter authentication, server routes or model protocols.
The old source `cell-settings.patch` and source-rebuild image recipe are removed.

One PVC contains workspace/, home/ and dsh/. The launcher starts DSH in workspace,
with HOME=home and DSH_HOME=dsh; native credentials remain in dsh/.credentials.yaml.
User npm tools install into HOME/.local and caches into /tmp. No platform secrets
or Kubernetes token enter this environment. Retention is not backup or migration.

Run `dev-run go=1.27 -- bash hack/verify-dsh-compat.sh` for official package install,
patch checksum, launcher auth/transport tests. See `test/rc2/README.md` for browser
and real upstream Sandbox smoke. Actual evidence and untested boundaries are in
`docs/evidence/mvp-a-rc2-2026-09-29.md`. Source standards are the only automatic CI.
