import type { RuntimeConfig } from 'nuxt/schema'
import type { RuntimeStatus } from '~~/shared/types/runtime'

export function trimTrailingSlash(value: string) {
  return value.replace(/\/+$/, '')
}

export function getRuntimeStatus(config: RuntimeConfig): RuntimeStatus {
  return {
    graphSchema: config.public.agentGraphSchema,
    runtimeRepository: config.public.agentRuntimeRepository,
    authConfigured: Boolean(config.agentAuthUrl || config.public.agentAuthUrl),
    identityConfigured: Boolean(config.agentIdentityUrl || config.public.agentIdentityUrl),
    runtimeConfigured: Boolean(config.agentRuntimeUrl),
    surrealdbConfigured: Boolean(config.surrealdbUrl),
    publicAuthUrl: config.public.agentAuthUrl,
    publicIdentityUrl: config.public.agentIdentityUrl
  }
}
