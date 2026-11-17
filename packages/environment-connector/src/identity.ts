/** Domain identities retain their exact wire strings and cannot be interchanged. */
export type Identity<Tag extends string> = string & { readonly __dshIdentity: Tag }
export type TenantId = Identity<'TenantId'>
export type PrincipalId = Identity<'PrincipalId'>
export type AllocationKey = Identity<'AllocationKey'>
export type EnvironmentNamespace = Identity<'EnvironmentNamespace'>
export type SandboxUID = Identity<'SandboxUID'>
export type PVCUID = Identity<'PVCUID'>
export type PodUID = Identity<'PodUID'>
export type ResourceVersion = Identity<'ResourceVersion'>

function identity<Tag extends string>(_tag: Tag, value: string): Identity<Tag> {
  return value as Identity<Tag>
}

/** Apply a role after its string was admitted by a parser or produced by a trusted generator. */
export const identities = {
  tenantId: (value: string): TenantId => identity('TenantId', value),
  principalId: (value: string): PrincipalId => identity('PrincipalId', value),
  allocationKey: (value: string): AllocationKey => identity('AllocationKey', value),
  namespace: (value: string): EnvironmentNamespace => identity('EnvironmentNamespace', value),
  sandboxUID: (value: string): SandboxUID => identity('SandboxUID', value),
  pvcUID: (value: string): PVCUID => identity('PVCUID', value),
  podUID: (value: string): PodUID => identity('PodUID', value),
  revision: (value: string): ResourceVersion => identity('ResourceVersion', value),
}
