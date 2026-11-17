import { identities, type ResourceVersion, type SandboxUID } from './identity.ts'
import { setTimeout as delay } from 'node:timers/promises'
import { API, APIError, isTerminal, type Resource } from './api.ts'
import {
  annotation,
  coordinates,
  dataName,
  group,
  networkPolicy,
  ownership,
  sandboxName,
  serviceAccount,
} from './resources.ts'
import {
  Mismatch,
  own,
  quantity,
  readyTarget,
  templateFor,
  verifyAncillary,
  verifyBase,
  verifyPod,
} from './verify.ts'
import { connector } from './proxy.ts'
import { parseAllocationIdentity, parseEnvironmentRef, parseEnvironmentRuntimeOptions } from './parse.ts'
import {
  EnvironmentError,
  type AgentEnvironmentRuntime,
  type EnvironmentContext,
  type EnvironmentIntent,
  type EnvironmentRef,
  type EnvironmentRuntimeOptions,
  type EnvironmentView,
  type EnvironmentFailure,
  type EnvironmentErrorCode,
} from './port.ts'
const a = (r: Resource, key: string) =>
  r.metadata.annotations?.[annotation + key]
const nsPath = (ns: string) => `/api/v1/namespaces/${ns}`
const sbPath = (ns: string) => `/apis/${group}/namespaces/${ns}/sandboxes`
const policyPath = (ns: string) =>
  `/apis/networking.k8s.io/v1/namespaces/${ns}/networkpolicies`
const metadata = (
  name: string,
  namespace: string,
  intent: EnvironmentIntent,
  uid?: string,
) => ({
  name,
  namespace,
  annotations: {
    ...ownership(intent),
    ...(uid ? { [annotation + 'sandbox-uid']: uid } : {}),
  },
})
const validName = (s: string) => /^[a-z0-9](?:[-a-z0-9]*[a-z0-9])?$/.test(s)

/** The only product implementation. No background reconciliation or automatic retry. */
export function createAgentEnvironmentRuntime(
  input: EnvironmentRuntimeOptions,
): AgentEnvironmentRuntime {
  const connections = new Map<SandboxUID, Set<AbortController>>()
  const error = (
    code: EnvironmentErrorCode,
    ctx: EnvironmentContext,
    intent: Pick<EnvironmentFailure, 'allocationKey'>,
    stage: EnvironmentFailure['stage'],
    effect: EnvironmentFailure['effect'] = 'not-submitted',
    state: EnvironmentFailure['observedState'] = 'unverified',
    cause?: unknown,
  ) =>
    new EnvironmentError({
      code,
      stage,
      effect,
      observedState: state,
      allocationKey: intent.allocationKey,
      correlationId: ctx.correlationId,
      retry:
        [
          'CreateOutcomeUnknown',
          'StartOutcomeUnknown',
          'DeleteOutcomeUnknown',
          'AllocationUnresolved',
          'ReadUnavailable',
          'NotReady',
          'StopUnverified',
        ].includes(code) || effect !== 'not-submitted'
          ? 'read-first'
          : 'never',
      nextAction:
        effect === 'unknown'
          ? 'Inspect the original allocation and exact UIDs before any further operation; never allocate another key'
          : 'Inspect the exact Sandbox, PVC and Pod events; resolve the reported condition before an explicit new request',
    }, { cause })
  const invalid = (cause?: unknown) =>
    new EnvironmentError({
      code: 'InvalidConfiguration',
      stage: 'configuration',
      effect: 'not-submitted',
      observedState: 'unverified',
      retry: 'never',
      allocationKey: '',
      correlationId: 'configuration',
      nextAction:
        'Provide the fixed image digest, native resources, one storage size, namespace prefix and Kubernetes credentials',
    }, { cause })
  let options: EnvironmentRuntimeOptions
  try {
    options = parseEnvironmentRuntimeOptions(input)
  } catch (cause) {
    throw invalid(cause)
  }
  if (
    !validName(options.namespacePrefix) ||
    options.namespacePrefix.length > 24 ||
    !validName(options.platformNamespace) ||
    !options.domain.split('.').every(validName) ||
    options.domain.length > 220 ||
    !options.storage.storageClassName ||
    !Number.isFinite(quantity(options.storage.size)) ||
    quantity(options.storage.size) <= 0
  )
    throw invalid()
  for (const key of ['cpu', 'memory'] as const)
    if (
      !Number.isFinite(quantity(options.resources.requests[key])) ||
      !Number.isFinite(quantity(options.resources.limits[key])) ||
      quantity(options.resources.requests[key]) <= 0 ||
      quantity(options.resources.limits[key]) <
        quantity(options.resources.requests[key])
    )
      throw invalid()
  try {
    templateFor(
      {
        allocationKey: identities.allocationKey('configuration'),
        owner: { tenantId: identities.tenantId('configuration'), principalId: identities.principalId('configuration') },
        namespace: identities.namespace('configuration'),
        sandbox: { name: sandboxName, uid: identities.sandboxUID('configuration') },
        data: { name: dataName, uid: identities.pvcUID('configuration') },
      },
      options,
    )
  } catch (cause) {
    throw invalid(cause)
  }
  let api: API
  try {
    api = new API(options.kubernetes)
  } catch (cause) {
    throw invalid(cause)
  }
  function refOK(ref: EnvironmentRef, ctx: EnvironmentContext) {
    try {
      parseEnvironmentRef(ref)
    } catch (cause) {
      throw error('StaleInstance', ctx, ref, 'inspect', 'not-submitted', 'unverified', cause)
    }
    if (
      ref.namespace !== coordinates(ref, options).namespace ||
      ref.sandbox.name !== sandboxName ||
      !ref.sandbox.uid ||
      ref.data.name !== dataName ||
      !ref.data.uid
    )
      throw error('StaleInstance', ctx, ref, 'inspect')
  }
  async function execute<T>(
    stage: EnvironmentFailure['stage'],
    intent: EnvironmentIntent,
    ctx: EnvironmentContext,
    fn: (accepted: () => void) => Promise<T>,
  ): Promise<T> {
    const submission = { accepted: false }
    try {
      ctx.signal.throwIfAborted()
      try {
        parseAllocationIdentity(intent)
      } catch (cause) {
        throw error('InvalidConfiguration', ctx, { allocationKey: '' }, 'configuration', 'not-submitted', 'unverified', cause)
      }
      return await fn(() => {
        submission.accepted = true
      })
    } catch (e) {
      if (e instanceof EnvironmentError) {
        if (submission.accepted && e.failure.effect === 'not-submitted')
          throw new EnvironmentError({
            ...e.failure,
            stage,
            effect: 'accepted',
            retry: 'read-first',
          }, { cause: e })
        throw e
      }
      if (e instanceof Mismatch)
        throw error(
          e.code,
          ctx,
          intent,
          stage,
          submission.accepted ? 'accepted' : 'not-submitted',
          'unverified',
          e,
        )
      const unknown =
        e instanceof APIError &&
        e.submitted &&
        (e.status === 0 || e.status >= 500)
      const code: EnvironmentErrorCode =
        e instanceof APIError && (e.status === 401 || e.status === 403)
          ? 'Forbidden'
          : e instanceof APIError && (e.status === 409 || e.status === 422)
            ? 'IntentConflict'
            : e instanceof APIError && e.status === 404
              ? 'StaleInstance'
              : stage === 'create'
                ? unknown
                  ? 'CreateOutcomeUnknown'
                  : 'CreateRejected'
                : stage === 'start'
                  ? unknown
                    ? 'StartOutcomeUnknown'
                    : 'StartRejected'
                  : stage === 'delete'
                    ? unknown
                      ? 'DeleteOutcomeUnknown'
                      : 'DeleteRejected'
                    : stage === 'stop'
                      ? 'StopUnverified'
                      : 'ReadUnavailable'
      throw error(
        code,
        ctx,
        intent,
        stage,
        unknown ? 'unknown' : submission.accepted ? 'accepted' : 'not-submitted',
        'unverified',
        e,
      )
    }
  }
  const patch = async (
    sb: Resource,
    ctx: EnvironmentContext,
    changes: unknown[],
  ) => {
    if (!sb.metadata.namespace) throw new Mismatch('StaleInstance')
    return api.call(
      'PATCH',
      `${sbPath(sb.metadata.namespace)}/${sb.metadata.name}`,
      ctx.signal,
      [
        { op: 'test', path: '/metadata/uid', value: sb.metadata.uid },
        {
          op: 'test',
          path: '/metadata/resourceVersion',
          value: sb.metadata.resourceVersion,
        },
        ...changes,
      ],
    )
  }
  const add = (key: string, value: string) => ({
    op: 'add',
    path: '/metadata/annotations/' + (annotation + key).replace(/\//g, '~1'),
    value,
  })
  const revoke = (uid: SandboxUID) => {
    for (const c of connections.get(uid) ?? []) c.abort()
    connections.delete(uid)
  }
  async function read(ref: EnvironmentRef, ctx: EnvironmentContext) {
    refOK(ref, ctx)
    const root = nsPath(ref.namespace)
    const [namespace, sb, pvc, sa, policy, podList, service, sliceList] =
      await Promise.all([
        api.call('GET', root, ctx.signal),
        api.call('GET', `${sbPath(ref.namespace)}/${sandboxName}`, ctx.signal),
        api.maybe(`${root}/persistentvolumeclaims/${dataName}`, ctx.signal),
        api.call(
          'GET',
          `${root}/serviceaccounts/${serviceAccount}`,
          ctx.signal,
        ),
        api.call('GET', `${policyPath(ref.namespace)}/workload`, ctx.signal),
        api.list(`${root}/pods`, ctx.signal),
        api.maybe(`${root}/services/${sandboxName}`, ctx.signal),
        api.list(
          `/apis/discovery.k8s.io/v1/namespaces/${ref.namespace}/endpointslices?labelSelector=kubernetes.io%2Fservice-name%3D${sandboxName}`,
          ctx.signal,
        ),
      ])
    if (!pvc) throw error('StorageMismatch', ctx, ref, 'inspect')
    verifyBase(ref, options, namespace, sb, pvc)
    verifyAncillary(ref, options, sa, policy)
    const pods = podList.items,
      slices = sliceList.items
    for (const pod of pods) verifyPod(pod, ref, options)
    let state: EnvironmentView['state'] = 'Pending',
      address: string | null = null
    const phase = a(sb, 'phase')
    if (sb.metadata.deletionTimestamp || phase === 'deleting')
      state = 'Deleting'
    else if (phase === 'stopping' || phase === 'unverified')
      state = phase === 'stopping' ? 'Stopping' : 'StopUnverified'
    else if (phase === 'stopped') {
      const proof = a(sb, 'terminal-uid')
      state =
        proof &&
        pods.length === 0 &&
        sb.spec?.operatingMode === 'Suspended' &&
        sb.status?.conditions?.some(
          c =>
            c.type === 'Suspended' &&
            c.status === 'True' &&
            c.observedGeneration === sb.metadata.generation,
        ) &&
        !slices.some(s =>
          (s.endpoints ?? []).some(e => e.conditions?.ready === true),
        )
          ? 'Stopped'
          : 'StopUnverified'
    } else if (phase === 'running' && sb.spec?.operatingMode === 'Running') {
      address = readyTarget(ref, options, sb, pvc, pods, service, slices)
      if (address) state = 'Ready'
      else if (pods.some(p => p.status?.phase === 'Failed'))
        state = 'Unavailable'
    } else state = 'Unavailable'
    return {
      watchRevision: podList.metadata.resourceVersion,
      namespace,
      sb,
      pvc,
      sa,
      policy,
      pods,
      slices,
      address,
      view: {
        ref,
        revision: sb.metadata.resourceVersion,
        state,
        origin: coordinates(ref, options).origin,
      } satisfies EnvironmentView,
    }
  }
  const view = (ref: EnvironmentRef, ctx: EnvironmentContext) =>
    execute('inspect', ref, ctx, async () => (await read(ref, ctx)).view)
  async function discover(
    intent: EnvironmentIntent,
    expected: EnvironmentRef | undefined,
    ctx: EnvironmentContext,
  ): Promise<EnvironmentView | null> {
    if (expected) {
      if (
        expected.allocationKey !== intent.allocationKey ||
        expected.owner.tenantId !== intent.owner.tenantId ||
        expected.owner.principalId !== intent.owner.principalId
      )
        throw error('OwnerMismatch', ctx, intent, 'inspect')
      return (await read(expected, ctx)).view
    }
    const { namespace } = coordinates(intent, options),
      ns = await api.maybe(nsPath(namespace), ctx.signal)
    if (!ns) return null
    own(ns, intent)
    const sb = await api.maybe(
      `${sbPath(namespace)}/${sandboxName}`,
      ctx.signal,
    )
    if (!sb) {
      if (a(ns, 'sandbox-uid'))
        throw error('StaleInstance', ctx, intent, 'inspect')
      throw error('AllocationUnresolved', ctx, intent, 'inspect', 'accepted')
    }
    own(sb, intent)
    const uid = a(sb, 'data-uid')
    if (!uid || !a(ns, 'sandbox-uid'))
      throw error('AllocationUnresolved', ctx, intent, 'inspect', 'accepted')
    return (
      await read(
        {
          ...intent,
          namespace,
          sandbox: { name: sandboxName, uid: identities.sandboxUID(sb.metadata.uid) },
          data: { name: dataName, uid: identities.pvcUID(uid) },
        },
        ctx,
      )
    ).view
  }
  async function create(intent: EnvironmentIntent, ctx: EnvironmentContext) {
    return execute('create', intent, ctx, async (accepted) => {
      const { namespace } = coordinates(intent, options)
      const existing = await api.maybe(nsPath(namespace), ctx.signal)
      if (existing) {
        own(existing, intent)
        const result = await discover(intent, undefined, ctx)
        if (!result)
          throw error(
            'AllocationUnresolved',
            ctx,
            intent,
            'create',
            'accepted',
          )
        return result
      }
      let ns = await api.call('POST', '/api/v1/namespaces', ctx.signal, {
        apiVersion: 'v1',
        kind: 'Namespace',
        metadata: {
          name: namespace,
          annotations: ownership(intent),
          labels: { 'app.kubernetes.io/managed-by': 'dsh-isolated-runtime' },
        },
      })
      accepted()
      let sb = await api.call('POST', sbPath(namespace), ctx.signal, {
        apiVersion: group,
        kind: 'Sandbox',
        metadata: {
          ...metadata(sandboxName, namespace, intent),
          annotations: {
            ...ownership(intent),
            [annotation + 'phase']: 'creating',
          },
        },
        spec: {
          operatingMode: 'Suspended',
          shutdownPolicy: 'Retain',
          service: true,
          podTemplate: templateFor(
            {
              ...intent,
              namespace,
              sandbox: { name: sandboxName, uid: identities.sandboxUID('pending') },
              data: { name: dataName, uid: identities.pvcUID('pending') },
            },
            options,
          ),
        },
      })
      ns = await api.call('GET', nsPath(namespace), ctx.signal)
      own(ns, intent)
      await api.call('PATCH', nsPath(namespace), ctx.signal, [
        { op: 'test', path: '/metadata/uid', value: ns.metadata.uid },
        {
          op: 'test',
          path: '/metadata/resourceVersion',
          value: ns.metadata.resourceVersion,
        },
        add('sandbox-uid', sb.metadata.uid),
      ])
      const pvc = await api.call(
        'POST',
        `${nsPath(namespace)}/persistentvolumeclaims`,
        ctx.signal,
        {
          apiVersion: 'v1',
          kind: 'PersistentVolumeClaim',
          metadata: metadata(dataName, namespace, intent, sb.metadata.uid),
          spec: {
            accessModes: ['ReadWriteOnce'],
            storageClassName: options.storage.storageClassName,
            resources: { requests: { storage: options.storage.size } },
          },
        },
      )
      await api.call(
        'POST',
        `${nsPath(namespace)}/serviceaccounts`,
        ctx.signal,
        {
          apiVersion: 'v1',
          kind: 'ServiceAccount',
          metadata: metadata(
            serviceAccount,
            namespace,
            intent,
            sb.metadata.uid,
          ),
          automountServiceAccountToken: false,
        },
      )
      const policy = networkPolicy(namespace, options.platformNamespace)
      policy.metadata = {
        ...policy.metadata,
        ...metadata('workload', namespace, intent, sb.metadata.uid),
      }
      await api.call('POST', policyPath(namespace), ctx.signal, policy)
      // Status may have advanced while infrastructure was created; read once, then CAS.
      sb = await api.call(
        'GET',
        `${sbPath(namespace)}/${sandboxName}`,
        ctx.signal,
      )
      own(sb, intent)
      await patch(sb, ctx, [
        add('data-uid', pvc.metadata.uid),
        add('phase', 'running'),
        { op: 'replace', path: '/spec/operatingMode', value: 'Running' },
      ])
      return (
        await read(
          {
            ...intent,
            namespace,
            sandbox: { name: sandboxName, uid: identities.sandboxUID(sb.metadata.uid) },
            data: { name: dataName, uid: identities.pvcUID(pvc.metadata.uid) },
          },
          ctx,
        )
      ).view
    })
  }
  async function healthyWriter(pod: Resource, ctx: EnvironmentContext) {
    const name = pod.spec?.nodeName
    if (!name) throw new Error('Writer has no node')
    const node = await api.call(
      'GET',
      `/api/v1/nodes/${encodeURIComponent(name)}`,
      ctx.signal,
    )
    const lease = await api.call(
      'GET',
      `/apis/coordination.k8s.io/v1/namespaces/kube-node-lease/leases/${encodeURIComponent(name)}`,
      ctx.signal,
    )
    const age = Date.now() - Date.parse(lease.spec?.renewTime ?? '')
    if (
      node.metadata.deletionTimestamp ||
      !node.status?.conditions?.some(
        c => c.type === 'Ready' && c.status === 'True',
      ) ||
      node.spec?.taints?.some(t =>
        [
          'node.kubernetes.io/not-ready',
          'node.kubernetes.io/unreachable',
        ].includes(t.key),
      ) ||
      !lease.metadata.ownerReferences?.some(
        r => r.kind === 'Node' && r.uid === node.metadata.uid,
      ) ||
      !Number.isFinite(age) ||
      age < -5000 ||
      age > 40_000
    )
      throw new Error('Writer node health unverified')
    return node.metadata.uid
  }
  async function stop(
    ref: EnvironmentRef,
    revision: ResourceVersion,
    ctx: EnvironmentContext,
  ) {
    return execute('stop', ref, ctx, async (accepted) => {
      let current = await read(ref, ctx)
      if (current.sb.metadata.resourceVersion !== revision)
        throw error('IntentConflict', ctx, ref, 'stop')
      if (current.view.state === 'Stopped')
        return { view: current.view, effect: 'not-submitted' as const }
      if (a(current.sb, 'phase') !== 'running')
        throw error('StopRejected', ctx, ref, 'stop')
      revoke(ref.sandbox.uid)
      const pod = current.pods.length === 1 ? current.pods[0] : undefined
      // Item RV may be older than watch history for a long-running Pod.
      // The list RV observes that exact item while providing a current cursor.
      const watchRevision = current.watchRevision
      const updated = await patch(current.sb, ctx, [
        add('phase', 'stopping'),
        add('writer-uid', pod?.metadata.uid ?? 'unknown'),
        add('writer-rv', pod?.metadata.resourceVersion ?? 'unknown'),
        add('watch-rv', watchRevision),
        { op: 'replace', path: '/spec/operatingMode', value: 'Suspended' },
      ])
      accepted()
      try {
        if (!pod) throw new Error('No positive writer identity')
        const writerUID = identities.podUID(pod.metadata.uid)
        const nodeUID = await healthyWriter(pod, ctx)
        if (!isTerminal(pod, writerUID))
          await api.terminal(
            `${nsPath(ref.namespace)}/pods?watch=true&fieldSelector=metadata.name%3D${encodeURIComponent(pod.metadata.name)}&resourceVersion=${encodeURIComponent(watchRevision)}&timeoutSeconds=50`,
            writerUID,
            ctx.signal,
          )
        const deadline = Date.now() + 15_000
        while (true) {
          current = await read(ref, ctx)
          if (
            current.pods.length === 0 &&
            current.sb.spec?.operatingMode === 'Suspended' &&
            current.sb.status?.conditions?.some(
              c =>
                c.type === 'Suspended' &&
                c.status === 'True' &&
                c.observedGeneration === current.sb.metadata.generation,
            )
          )
            break
          if (Date.now() > deadline) throw new Error('Stop not observed')
          await delay(200, undefined, { signal: ctx.signal })
        }
        if ((await healthyWriter(pod, ctx)) !== nodeUID)
          throw new Error('Writer node replaced')
        await patch(current.sb, ctx, [
          add('phase', 'stopped'),
          add('terminal-uid', writerUID),
        ])
        const result = await read(ref, ctx)
        if (result.view.state !== 'Stopped')
          throw new Error('Stop still unverified')
        return { view: result.view, effect: 'accepted' as const }
      } catch (cause) {
        // Best effort diagnostic only; the durable stopping gate already denies start.
        let failure = cause
        try {
          const latest = await api.call(
            'GET',
            `${sbPath(ref.namespace)}/${sandboxName}`,
            AbortSignal.timeout(3000),
          )
          if (
            latest.metadata.uid === updated.metadata.uid &&
            a(latest, 'phase') === 'stopping'
          )
            await patch(latest, { ...ctx, signal: AbortSignal.timeout(3000) }, [
              add('phase', 'unverified'),
            ])
        } catch (diagnosticCause) {
          failure = new AggregateError([cause, diagnosticCause], 'Stop proof and diagnostic update failed')
        }
        throw error(
          'StopUnverified',
          ctx,
          ref,
          'stop',
          'accepted',
          'StopUnverified',
          failure,
        )
      }
    })
  }
  async function start(
    ref: EnvironmentRef,
    revision: ResourceVersion,
    ctx: EnvironmentContext,
  ) {
    return execute('start', ref, ctx, async (accepted) => {
      const current = await read(ref, ctx)
      if (current.view.revision !== revision)
        throw error('IntentConflict', ctx, ref, 'start')
      if (current.view.state !== 'Stopped')
        throw error(
          'StartRejected',
          ctx,
          ref,
          'start',
          'not-submitted',
          current.view.state,
        )
      if (current.pvc.status?.phase !== 'Bound' || !current.pvc.spec?.volumeName)
        throw error('StorageMismatch', ctx, ref, 'start')
      await patch(current.sb, ctx, [
        add('phase', 'running'),
        {
          op: 'remove',
          path:
            '/metadata/annotations/' +
            (annotation + 'terminal-uid').replace(/\//g, '~1'),
        },
        { op: 'replace', path: '/spec/operatingMode', value: 'Running' },
      ])
      accepted()
      return { view: (await read(ref, ctx)).view, effect: 'accepted' as const }
    })
  }
  async function remove(
    ref: EnvironmentRef,
    revision: ResourceVersion,
    ctx: EnvironmentContext,
  ) {
    return execute('delete', ref, ctx, async (accepted) => {
      refOK(ref, ctx)
      const ns = await api.call('GET', nsPath(ref.namespace), ctx.signal)
      own(ns, ref)
      if (a(ns, 'sandbox-uid') !== ref.sandbox.uid)
        throw error('StaleInstance', ctx, ref, 'delete')
      const sb = await api.maybe(
        `${sbPath(ref.namespace)}/${sandboxName}`,
        ctx.signal,
      )
      if (!sb) {
        const pvc = await api.maybe(
          `${nsPath(ref.namespace)}/persistentvolumeclaims/${dataName}`,
          ctx.signal,
        )
        if (!pvc || pvc.metadata.uid !== ref.data.uid)
          throw error('StorageMismatch', ctx, ref, 'delete')
        own(pvc, ref)
        const pods = await api.list(
          `${nsPath(ref.namespace)}/pods`,
          ctx.signal,
        )
        if (a(ns, 'deleted-uid') !== ref.sandbox.uid || pods.items.length)
          throw error('StaleInstance', ctx, ref, 'delete')
        return {
          ref,
          effect: 'not-submitted' as const,
          state: 'Deleted' as const,
          dataRetained: true as const,
        }
      }
      if (sb.metadata.uid !== ref.sandbox.uid)
        throw error('StaleInstance', ctx, ref, 'delete')
      if (
        sb.metadata.deletionTimestamp &&
        a(ns, 'deleted-uid') === ref.sandbox.uid
      ) {
        own(sb, ref)
        const pvc = await api.maybe(
          `${nsPath(ref.namespace)}/persistentvolumeclaims/${dataName}`,
          ctx.signal,
        )
        if (!pvc || pvc.metadata.uid !== ref.data.uid)
          throw error('StorageMismatch', ctx, ref, 'delete')
        own(pvc, ref)
        return {
          ref,
          effect: 'not-submitted' as const,
          state: 'Deleting' as const,
          dataRetained: true as const,
        }
      }
      const current = await read(ref, ctx)
      if (current.view.revision !== revision)
        throw error('IntentConflict', ctx, ref, 'delete')
      if (
        current.view.state !== 'Stopped' &&
        a(current.sb, 'phase') !== 'deleting'
      )
        throw error(
          'DeleteRejected',
          ctx,
          ref,
          'delete',
          'not-submitted',
          current.view.state,
        )
      revoke(ref.sandbox.uid)
      const gated =
        a(current.sb, 'phase') === 'deleting'
          ? current.sb
          : await patch(current.sb, ctx, [add('phase', 'deleting')])
      accepted()
      // Receipt lives with this retained volume's namespace, not in a tombstone database.
      const latestNS = await api.call('GET', nsPath(ref.namespace), ctx.signal)
      own(latestNS, ref)
      await api.call('PATCH', nsPath(ref.namespace), ctx.signal, [
        { op: 'test', path: '/metadata/uid', value: latestNS.metadata.uid },
        {
          op: 'test',
          path: '/metadata/resourceVersion',
          value: latestNS.metadata.resourceVersion,
        },
        add('deleted-uid', ref.sandbox.uid),
      ])
      await api.remove(
        `${sbPath(ref.namespace)}/${sandboxName}`,
        ctx.signal,
        {
          apiVersion: 'v1',
          kind: 'DeleteOptions',
          preconditions: {
            uid: ref.sandbox.uid,
            resourceVersion: gated.metadata.resourceVersion,
          },
          propagationPolicy: 'Foreground',
        },
      )
      return {
        ref,
        effect: 'accepted' as const,
        state: 'Deleting' as const,
        dataRetained: true as const,
      }
    })
  }
  return {
    create,
    inspect: view,
    inspectAllocation: (intent, expected, ctx) =>
      execute('inspect', intent, ctx, () => discover(intent, expected, ctx)),
    stop,
    start,
    delete: remove,
    connect: (ref, ctx) =>
      execute('access', ref, ctx, async () => {
        const checked = await read(ref, ctx)
        if (checked.view.state !== 'Ready')
          throw error(
            'NotReady',
            ctx,
            ref,
            'access',
            'not-submitted',
            checked.view.state,
          )
        const lifetime = new AbortController()
        const active =
          connections.get(ref.sandbox.uid) ?? new Set<AbortController>()
        active.add(lifetime)
        connections.set(ref.sandbox.uid, active)
        const signal = AbortSignal.any([ctx.signal, lifetime.signal])
        signal.addEventListener(
          'abort',
          () => {
            active.delete(lifetime)
            if (!active.size) connections.delete(ref.sandbox.uid)
          },
          { once: true },
        )
        const transport = connector(
          checked.view.origin,
          { ...ctx, signal },
          async () => {
            const current = await read(ref, { ...ctx, signal })
            if (current.view.state !== 'Ready' || !current.address)
              throw error(
                'NotReady',
                ctx,
                ref,
                'access',
                'not-submitted',
                current.view.state,
              )
            return current.address
          },
        )
        const release = () => {
          active.delete(lifetime)
          if (!active.size) connections.delete(ref.sandbox.uid)
        }
        return {
          origin: transport.origin,
          async forward(req, res) {
            res.once('close', release)
            try {
              await transport.forward(req, res)
            } catch (e) {
              release()
              throw e
            }
          },
          async upgrade(req, socket, head) {
            socket.once('close', release)
            try {
              await transport.upgrade(req, socket, head)
            } catch (e) {
              release()
              throw e
            }
          },
        }
      }),
  }
}
