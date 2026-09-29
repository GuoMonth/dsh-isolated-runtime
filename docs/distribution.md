# Local RC artifacts

The runtime publishes no installation CLI. The platform installation artifact consumes:

1. The exact internal Connector tarball produced from a clean committed tree by `node hack/pack-environment-connector.mjs /absolute/output-directory`. Record `source.json`, SHA-256 and npm SHA-512 integrity.
2. The environment image built with `docker build --platform linux/amd64 --build-arg SOURCE_REVISION=$(git rev-parse HEAD) -f images/environment/Dockerfile -t dsh-environment:candidate .`. Record the manifest digest and local image identity; import into the dedicated kind cluster without a registry push.
3. Upstream Agent Sandbox core v1.0.3 and the runtime [ClusterRole](../config/runtime/cluster-role.yaml). Platform packaging may include checksummed copies, not a second implementation.

The fixed RC npm closure and minimal remote-settings patch are described in [compat/dsh](../compat/dsh/README.md). One PVC contains all durable state. Deletion of a stopped Sandbox leaves its data and isolated namespace; operator data removal is outside the runtime API.

No package publication, public image push or Release is authorized in this implementation wave. The user performs final E2E and decides publication afterward.
