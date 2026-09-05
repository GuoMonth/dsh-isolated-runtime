# v0.1.0 — first local MVP

This pre-release provides a Linux x86_64 local kind experience using DSH's own
browser UI: create a Cell, authenticate, configure a model, use tools and files,
retain state across Pod replacement and optionally restore a writer-stopped CSI
snapshot into a fresh Cell.

Download the archive and SHA256SUMS, verify the checksum and follow the included
English or Chinese Quickstart. Docker is required. Model usage requires your own
credentials. The archive's release.json records the exact DSH source, image
digests and candidate workflow; live-model.json records the final live smoke.
Images retain their original SBOM/provenance and are not rebuilt for publication.

DSH 0.1.3-alpha.1 is built from its pinned official source because the matching
npm artifact was not published at baseline selection. A recorded, hashed Cell
integration patch enables DSH's native settings at the authenticated Cell
hostname. The release manifest records this source and patch, without claiming
an unavailable upstream npm integrity value.

Only this DSH baseline and linux/amd64 are supported. Snapshots are crash-consistent,
exclude private credentials and require fresh authorization and provider setup
after restore. There is no old-version migration support, production capacity
promise, HA or multi-cluster scope. Local demo teardown deletes its cluster/data.

The upstream 0.1.3-alpha.1 release notes report a session-loading performance
regression. This MVP keeps that upstream limitation explicit; it does not add a
session engine or compatibility layer to work around it.
