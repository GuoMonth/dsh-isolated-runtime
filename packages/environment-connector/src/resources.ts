import { createHash } from "node:crypto";
import type { EnvironmentIntent, EnvironmentRuntimeOptions } from "./index.js";
export const annotation = "environment.dsh.io/";
export const group = "agents.x-k8s.io/v1beta1";
export const sandboxName = "environment",
  dataName = "data",
  serviceAccount = "workload";
export function coordinates(
  intent: EnvironmentIntent,
  options: EnvironmentRuntimeOptions,
) {
  const key = createHash("sha256")
    .update(intent.allocationKey)
    .digest("hex")
    .slice(0, 32);
  const namespace = `${options.namespacePrefix}-${key}`;
  return { namespace, origin: `https://${key}.${options.domain}` };
}
export function ownership(intent: EnvironmentIntent) {
  return {
    [annotation + "allocation"]: intent.allocationKey,
    [annotation + "owner"]: JSON.stringify({
      tenantId: intent.owner.tenantId,
      principalId: intent.owner.principalId,
    }),
  };
}
export function networkPolicy(namespace: string, platformNamespace: string) {
  return {
    apiVersion: "networking.k8s.io/v1",
    kind: "NetworkPolicy",
    metadata: { name: "workload", namespace },
    spec: {
      podSelector: {},
      policyTypes: ["Ingress", "Egress"],
      ingress: [
        {
          from: [
            {
              namespaceSelector: {
                matchLabels: {
                  "kubernetes.io/metadata.name": platformNamespace,
                },
              },
              podSelector: {
                matchLabels: { "app.kubernetes.io/name": "dsh-platform" },
              },
            },
          ],
          ports: [{ protocol: "TCP", port: 8080 }],
        },
      ],
      egress: [
        {
          to: [
            {
              namespaceSelector: {
                matchLabels: { "kubernetes.io/metadata.name": "kube-system" },
              },
              podSelector: { matchLabels: { "k8s-app": "kube-dns" } },
            },
          ],
          ports: [
            { protocol: "UDP", port: 53 },
            { protocol: "TCP", port: 53 },
          ],
        },
        {
          to: [
            {
              ipBlock: {
                cidr: "0.0.0.0/0",
                except: [
                  "0.0.0.0/8",
                  "10.0.0.0/8",
                  "100.64.0.0/10",
                  "127.0.0.0/8",
                  "169.254.0.0/16",
                  "172.16.0.0/12",
                  "192.168.0.0/16",
                  "224.0.0.0/4",
                  "240.0.0.0/4",
                ],
              },
            },
          ],
        },
      ],
    },
  };
}
