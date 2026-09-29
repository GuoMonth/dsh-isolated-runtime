export const ENVIRONMENT_TEMPLATE = "dsh-rc2-single-pvc-v1";
export interface EnvironmentTemplateOptions {
  readonly image: string;
  readonly authority: string;
  readonly dataClaim: string;
  readonly serviceAccount: string;
  readonly resources: {
    readonly requests: { readonly cpu: string; readonly memory: string };
    readonly limits: { readonly cpu: string; readonly memory: string };
  };
}

/** Sole workload template. Installation and adapters consume this function, not a copy. */
export function environmentTemplate(options: EnvironmentTemplateOptions) {
  if (!/^\S+@sha256:[a-f0-9]{64}$/.test(options.image))
    throw new Error("image must use an immutable digest");
  if (!/^[a-z0-9.-]+(?::[0-9]+)?$/.test(options.authority))
    throw new Error("invalid authority");
  for (const name of [options.dataClaim, options.serviceAccount]) {
    if (!/^[a-z0-9]([-a-z0-9]*[a-z0-9])?$/.test(name) || name.length > 63)
      throw new Error("invalid resource name");
  }
  for (const resource of [
    options.resources.requests,
    options.resources.limits,
  ]) {
    if (!resource.cpu || !resource.memory)
      throw new Error("CPU and memory requests/limits are required");
  }
  return {
    metadata: {
      labels: {
        "app.kubernetes.io/name": "dsh-environment",
        "app.kubernetes.io/managed-by": "dsh-isolated-runtime",
      },
    },
    spec: {
      automountServiceAccountToken: false,
      enableServiceLinks: false,
      serviceAccountName: options.serviceAccount,
      terminationGracePeriodSeconds: 30,
      securityContext: {
        runAsNonRoot: true,
        runAsUser: 1000,
        runAsGroup: 1000,
        fsGroup: 1000,
        fsGroupChangePolicy: "OnRootMismatch",
        seccompProfile: { type: "RuntimeDefault" },
      },
      containers: [
        {
          name: "dsh",
          image: options.image,
          imagePullPolicy: "IfNotPresent",
          command: ["/usr/local/bin/environment-launcher"],
          workingDir: "/var/lib/dsh/data",
          env: [
            { name: "DSH_AUTHORITY", value: options.authority },
            { name: "HOME", value: "/var/lib/dsh/data/home" },
            { name: "DSH_HOME", value: "/var/lib/dsh/data/dsh" },
            { name: "DSH_PERMISSION_MODE", value: "danger-full-access" },
            { name: "DSH_TELEMETRY_DISABLED", value: "1" },
            { name: "XDG_CACHE_HOME", value: "/tmp/.cache" },
            {
              name: "NPM_CONFIG_PREFIX",
              value: "/var/lib/dsh/data/home/.local",
            },
            { name: "NPM_CONFIG_CACHE", value: "/tmp/.npm" },
          ],
          ports: [
            { name: "http", containerPort: 8080 },
            { name: "management", containerPort: 8081 },
          ],
          resources: structuredClone(options.resources),
          securityContext: {
            allowPrivilegeEscalation: false,
            readOnlyRootFilesystem: true,
            capabilities: { drop: ["ALL"] },
          },
          startupProbe: {
            httpGet: { path: "/readyz", port: "management" },
            periodSeconds: 3,
            failureThreshold: 30,
          },
          readinessProbe: {
            httpGet: { path: "/readyz", port: "management" },
            periodSeconds: 3,
          },
          livenessProbe: {
            httpGet: { path: "/livez", port: "management" },
            periodSeconds: 10,
          },
          volumeMounts: [
            { name: "data", mountPath: "/var/lib/dsh/data" },
            { name: "tmp", mountPath: "/tmp" },
          ],
        },
      ],
      volumes: [
        {
          name: "data",
          persistentVolumeClaim: { claimName: options.dataClaim },
        },
        { name: "tmp", emptyDir: { sizeLimit: "1Gi" } },
      ],
    },
  };
}
