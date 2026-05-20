export default defineEventHandler((event) => {
  const config = useRuntimeConfig(event)
  const runtime = getRuntimeStatus(config)

  return {
    ok: true,
    service: 'agent-chat',
    runtime: {
      graphSchema: runtime.graphSchema,
      runtimeRepository: runtime.runtimeRepository,
      authConfigured: runtime.authConfigured,
      identityConfigured: runtime.identityConfigured,
      runtimeConfigured: runtime.runtimeConfigured,
      surrealdbConfigured: runtime.surrealdbConfigured
    }
  }
})
