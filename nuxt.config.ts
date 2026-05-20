export default defineNuxtConfig({
  compatibilityDate: '2025-07-15',
  modules: ['@nuxt/ui'],
  css: ['~/assets/css/main.css'],
  devtools: { enabled: true },
  runtimeConfig: {
    agentRuntimeUrl: '',
    agentRuntimeApiKey: '',
    agentAuthUrl: '',
    agentIdentityUrl: '',
    surrealdbUrl: '',
    public: {
      agentGraphSchema: 'Agent-Graph',
      agentRuntimeRepository: 'https://github.com/AGenNext/Agent-Runtime',
      agentAuthUrl: '',
      agentIdentityUrl: ''
    }
  }
})
