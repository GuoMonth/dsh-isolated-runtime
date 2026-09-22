import test from "node:test";
import assert from "node:assert/strict";
import { createHash, randomUUID } from "node:crypto";
import { createServer } from "node:https";
import { execFileSync } from "node:child_process";
import { mkdtempSync, readFileSync, writeFileSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { createCellAllocationRuntime } from "../dist/index.js";
import { bindCellTemplate } from "../dist/cell-template.js";

const image = "ghcr.io/example/dsh@sha256:" + "a".repeat(64);
const config = {
  template: "cell-mvp-v1",
  image,
  storage: { size: "20Gi", storageClassName: "fast-rwo" },
  resources: {
    requests: { cpu: "250m", memory: "512Mi" },
    limits: { cpu: "1", memory: "1Gi" },
  },
  credentialsSecret: "alice-dsh-credentials",
  namespaces: { "tenant-a": "tenant-alice" },
  domain: "cells.example.com",
};
const replace = (value, identity, name, origin) =>
  typeof value === "string"
    ? value
        .replaceAll("${INSTANCE_ID}", identity)
        .replaceAll("${CELL_NAME}", name)
        .replaceAll("${ORIGIN_HOST}", new URL(origin).host)
    : Array.isArray(value)
      ? value.map((entry) => replace(entry, identity, name, origin))
      : value && typeof value === "object"
        ? Object.fromEntries(
            Object.entries(value).map(([key, entry]) => [
              key,
              replace(entry, identity, name, origin),
            ]),
          )
        : value;

async function readyFixture(
  t,
  mutatePod = () => {},
  mutateWorkload = () => {},
) {
  const directory = mkdtempSync(join(tmpdir(), "dsh-cell-template-"));
  const key = join(directory, "key.pem");
  const cert = join(directory, "cert.pem");
  const token = join(directory, "token");
  execFileSync(
    "openssl",
    [
      "req",
      "-x509",
      "-newkey",
      "rsa:2048",
      "-nodes",
      "-keyout",
      key,
      "-out",
      cert,
      "-days",
      "1",
      "-subj",
      "/CN=localhost",
      "-addext",
      "subjectAltName=IP:127.0.0.1",
    ],
    { stdio: "ignore" },
  );
  writeFileSync(token, "local-test-token");

  let cell;
  let objects;
  const server = createServer(
    { key: readFileSync(key), cert: readFileSync(cert) },
    async (req, res) => {
      const chunks = [];
      for await (const chunk of req) chunks.push(chunk);
      const path = new URL(req.url, "https://localhost").pathname;
      assert.equal(req.headers.authorization, "Bearer local-test-token");
      res.setHeader("content-type", "application/json");
      if (req.method === "POST") {
        cell = JSON.parse(Buffer.concat(chunks).toString());
        cell.metadata.uid = randomUUID();
        cell.metadata.resourceVersion = "7";
        cell.metadata.generation = 1;
        cell.status = {
          observedGeneration: 1,
          dshVersion: "0.1.5-rc.2",
          imageDigest: image.split("@")[1],
          conditions: [
            {
              type: "Ready",
              status: "False",
              observedGeneration: 1,
              reason: "Ready",
            },
          ],
        };
        objects = createObjects(cell);
        res.writeHead(201);
        res.end(JSON.stringify(cell));
        return;
      }
      if (path.endsWith("/endpointslices")) {
        res.writeHead(200);
        res.end(JSON.stringify({ items: [objects.slice] }));
        return;
      }
      const object =
        path.includes("/cells/")
          ? cell
          : path.includes("/statefulsets/")
            ? objects.workload
            : path.includes("/services/")
              ? objects.service
              : path.includes("/pods/")
                ? objects.pod
                : undefined;
      if (!object) {
        res.writeHead(404);
        res.end("{}");
        return;
      }
      if (path.includes("/pods/")) mutatePod(object);
      if (path.includes("/statefulsets/")) mutateWorkload(object);
      res.writeHead(200);
      res.end(JSON.stringify(object));
    },
  );
  await new Promise((resolve) => server.listen(0, "127.0.0.1", resolve));
  t.after(async () => {
    server.closeAllConnections();
    await new Promise((resolve) => server.close(resolve));
    rmSync(directory, { recursive: true });
  });

  const runtime = createCellAllocationRuntime(
    {
      server: `https://127.0.0.1:${server.address().port}`,
      caFile: cert,
      tokenFile: token,
    },
    config,
  );
  const intent = {
    allocationKey: randomUUID(),
    owner: { tenantId: "tenant-a", principalId: "alice" },
    template: "cell-mvp-v1",
  };
  const context = () => ({
    signal: new AbortController().signal,
    correlationId: randomUUID(),
  });
  const created = await runtime.create(intent, context());
  cell.status.conditions[0].status = "True";
  return { created, runtime, intent, context, cell, objects };

  function createObjects(createdCell) {
    const identity = createdCell.metadata.uid;
    const name = createdCell.metadata.name;
    const base = `cell-${identity}`;
    const origin = `https://cell-${identity}.${config.domain}`;
    const rendered = bindCellTemplate({
      image: config.image,
      storage: {
        size: config.storage.size,
        storageClassName: config.storage.storageClassName,
        retentionPolicy: "Retain",
      },
      resources: config.resources,
      credentialsSecret: config.credentialsSecret,
    });
    const podTemplate = replace(rendered.podTemplate, identity, name, origin);
    podTemplate.spec.serviceAccount = podTemplate.spec.serviceAccountName;
    const ownerCell = [
      {
        apiVersion: "dsh.isolated.io/v1alpha1",
        kind: "Cell",
        name,
        uid: identity,
        controller: true,
      },
    ];
    const workloadUID = randomUUID();
    const serviceUID = randomUUID();
    const podUID = randomUUID();
    const annotations = {
      "dsh.isolated.io/cell-uid": identity,
      "dsh.isolated.io/cell-name": name,
    };
    const podSpec = structuredClone(podTemplate.spec);
    Object.assign(podSpec, {
      serviceAccount: podSpec.serviceAccountName,
      hostname: `${base}-0`,
      subdomain: `${base}-headless`,
      nodeName: "worker-1",
      priority: 0,
      preemptionPolicy: "PreemptLowerPriority",
      tolerations: [
        {
          key: "node.kubernetes.io/not-ready",
          operator: "Exists",
          effect: "NoExecute",
          tolerationSeconds: 300,
        },
        {
          key: "node.kubernetes.io/unreachable",
          operator: "Exists",
          effect: "NoExecute",
          tolerationSeconds: 300,
        },
      ],
    });
    return {
      workload: {
        metadata: {
          name: base,
          namespace: "tenant-alice",
          uid: workloadUID,
          annotations: { ...annotations, "dsh.isolated.io/access-mode": "platform" },
          ownerReferences: ownerCell,
        },
        spec: {
          replicas: 1,
          serviceName: `${base}-headless`,
          selector: { matchLabels: podTemplate.metadata.labels },
          template: podTemplate,
        },
      },
      service: {
        metadata: {
          name: base,
          namespace: "tenant-alice",
          uid: serviceUID,
          annotations,
          ownerReferences: ownerCell,
        },
        spec: {
          type: "ClusterIP",
          selector: podTemplate.metadata.labels,
          ports: [
            { name: "http", port: 80, targetPort: "http", protocol: "TCP" },
          ],
        },
      },
      pod: {
        metadata: {
          name: `${base}-0`,
          namespace: "tenant-alice",
          uid: podUID,
          annotations,
          labels: {
            ...podTemplate.metadata.labels,
            "statefulset.kubernetes.io/pod-name": `${base}-0`,
          },
          ownerReferences: [
            {
              apiVersion: "apps/v1",
              kind: "StatefulSet",
              name: base,
              uid: workloadUID,
              controller: true,
            },
          ],
        },
        spec: podSpec,
        status: {
          podIP: "10.0.0.5",
          conditions: [{ type: "Ready", status: "True" }],
        },
      },
      slice: {
        metadata: {
          name: `${base}-slice`,
          namespace: "tenant-alice",
          uid: randomUUID(),
          labels: { "kubernetes.io/service-name": base },
          ownerReferences: [
            {
              apiVersion: "v1",
              kind: "Service",
              name: base,
              uid: serviceUID,
              controller: true,
            },
          ],
        },
        ports: [{ port: 8080, protocol: "TCP" }],
        endpoints: [
          {
            conditions: { ready: true, terminating: false },
            targetRef: {
              kind: "Pod",
              uid: podUID,
              name: `${base}-0`,
              namespace: "tenant-alice",
            },
            addresses: ["10.0.0.5"],
          },
        ],
      },
    };
  }
}

test("generated complete StatefulSet and live Pod templates accept known defaults", async (t) => {
  const fixture = await readyFixture(t);
  const ready = await fixture.runtime.inspectAllocation(
    fixture.intent,
    fixture.created.ref.identity,
    fixture.context(),
  );
  assert.equal(ready.state, "Ready");
});

test("StatefulSet template sidecar injection fails fixed template verification", async (t) => {
  const fixture = await readyFixture(t, () => {}, (workload) =>
    workload.spec.template.spec.containers.push({ name: "injected", image: "sidecar" }),
  );
  await assert.rejects(
    fixture.runtime.inspectAllocation(
      fixture.intent,
      fixture.created.ref.identity,
      fixture.context(),
    ),
    { code: "TemplateMismatch" },
  );
});

for (const [name, mutate] of [
  ["sidecar", (pod) => pod.spec.containers.push({ name: "injected", image: "sidecar" })],
  ["extra env", (pod) => pod.spec.containers[0].env.push({ name: "INJECTED", value: "1" })],
  ["extra volume", (pod) => pod.spec.volumes.push({ name: "injected", emptyDir: {} })],
  ["extra toleration", (pod) => pod.spec.tolerations.push({ key: "injected", operator: "Exists" })],
  ["LimitRange resources", (pod) => (pod.spec.containers[0].resources.requests.cpu = "500m")],
  ["RuntimeClass mutation", (pod) => (pod.spec.runtimeClassName = "gvisor")],
  ["missing scheduler node", (pod) => delete pod.spec.nodeName],
]) {
  test(`live Pod ${name} injection fails fixed template verification`, async (t) => {
    const fixture = await readyFixture(t, mutate);
    await assert.rejects(
      fixture.runtime.inspectAllocation(
        fixture.intent,
        fixture.created.ref.identity,
        fixture.context(),
      ),
      { code: "TemplateMismatch" },
    );
  });
}
