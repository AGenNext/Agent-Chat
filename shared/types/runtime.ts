export interface RuntimeStatus {
  graphSchema: string
  runtimeRepository: string
  authConfigured: boolean
  identityConfigured: boolean
  runtimeConfigured: boolean
  surrealdbConfigured: boolean
  publicAuthUrl: string
  publicIdentityUrl: string
}
