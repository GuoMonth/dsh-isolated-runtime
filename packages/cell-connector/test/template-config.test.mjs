import test from "node:test";
import assert from "node:assert/strict";
import { randomUUID } from "node:crypto";
import { createCellAllocationRuntime } from "../dist/index.js";
import { CELL_DSH_VERSION } from "../dist/cell-template.js";
import fixedTemplate from "../dist/templates/cell-mvp-v1.json" with { type: "json" };

const base = {
  template: "cell-mvp-v1",
  image: "ghcr.io/example/dsh@sha256:" + "a".repeat(64),
  storage: { size: "20Gi" },
  resources: {
    requests: { cpu: "250m", memory: "512Mi" },
    limits: { cpu: "1", memory: "1Gi" },
  },
  namespaces: { "tenant-a": "tenant-alice" },
  domain: "cells.example.com",
};
const kubernetes = { server: "https://127.0.0.1" };

test("fixed Cell template rejects legacy profiles, unknown keys and version drift", async () => {
  assert.equal(fixedTemplate.dshVersion, CELL_DSH_VERSION);
  assert.throws(
    () => createCellAllocationRuntime(kubernetes, { ...base, profiles: [] }),
    /allocation fields/,
  );
  assert.throws(
    () => createCellAllocationRuntime(kubernetes, { ...base, expectedSpec: {} }),
    /allocation fields/,
  );
  assert.throws(
    () => createCellAllocationRuntime(kubernetes, { ...base, template: "next" }),
    /template version/,
  );
  const runtime = createCellAllocationRuntime(kubernetes, base);
  await assert.rejects(
    runtime.create(
      {
        allocationKey: randomUUID(),
        owner: { tenantId: "tenant-a", principalId: "alice" },
        template: "next",
      },
      { signal: new AbortController().signal, correlationId: randomUUID() },
    ),
    { code: "InvalidConfiguration" },
  );
});

test("Cell inputs accept only positive canonical bounded quantities and names", () => {
  assert.doesNotThrow(() =>
    createCellAllocationRuntime(kubernetes, {
      ...base,
      storage: {
        size: "20Gi",
        storageClassName: "fast-rwo",
        retentionPolicy: "Delete",
      },
      credentialsSecret: "alice-dsh-credentials",
    }),
  );
  for (const cpu of ["0", "0.5", "1000m", "01", "1e3", "99999999999999999999"]) {
    assert.throws(
      () =>
        createCellAllocationRuntime(kubernetes, {
          ...base,
          resources: {
            requests: { cpu, memory: "512Mi" },
            limits: { cpu: "2", memory: "1Gi" },
          },
        }),
      /resources/,
    );
  }
  for (const memory of ["0Gi", "1024Mi", "1.5Gi", "1e3Mi", "01Gi", "99999999999999999999Ti"]) {
    assert.throws(
      () =>
        createCellAllocationRuntime(kubernetes, {
          ...base,
          resources: {
            requests: { cpu: "250m", memory },
            limits: { cpu: "1", memory: "2Gi" },
          },
        }),
      /resources/,
    );
  }
  for (const size of ["0Gi", "1024Mi", "20g", "1.5Gi"]) {
    assert.throws(
      () => createCellAllocationRuntime(kubernetes, { ...base, storage: { size } }),
      /storage/,
    );
  }
  assert.throws(
    () =>
      createCellAllocationRuntime(kubernetes, {
        ...base,
        namespaces: { "tenant-a": "Tenant-A" },
      }),
    /namespaces/,
  );
  assert.throws(
    () => createCellAllocationRuntime(kubernetes, { ...base, domain: "cells.example.com:443" }),
    /domain/,
  );
});
