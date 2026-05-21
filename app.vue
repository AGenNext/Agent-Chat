<script setup lang="ts">
type ChatRole = 'assistant' | 'user'

type ChatMessage = {
  id: number
  role: ChatRole
  content: string
  meta?: string
  artifact?: string
}

type AgentModel = {
  label: string
  value: string
  description: string
}

const prompt = ref('')
const isSending = ref(false)
const isSidebarOpen = ref(false)
const selectedModel = ref('agent-orchestrator')
const activeArtifact = ref('Execution brief')

const models: AgentModel[] = [
  {
    label: 'Agent Orchestrator',
    value: 'agent-orchestrator',
    description: 'Best for planning and coordinating agent work'
  },
  {
    label: 'Runtime Debugger',
    value: 'runtime-debugger',
    description: 'Inspect failures, payloads, and service handoffs'
  },
  {
    label: 'Product Copilot',
    value: 'product-copilot',
    description: 'Sharper writing, specs, and execution notes'
  }
]

const messages = ref<ChatMessage[]>([
  {
    id: 1,
    role: 'assistant',
    content:
      'Hi, I’m your AGenNext copilot. I can plan agent workflows, inspect runtime handoffs, and turn messy context into clean execution artifacts.',
    meta: 'Agent Runtime ready',
    artifact: 'Execution brief'
  }
])

const suggestions = [
  'Create an execution brief for Agent-Runtime',
  'Debug an auth → identity → runtime handoff',
  'Draft a multi-agent workflow spec',
  'Turn this conversation into a launch checklist'
]

const runtimeChips = ['Agent-Auth', 'Agent-Identity', 'Agent-Runtime', 'SurrealDB']

const artifacts = computed(() => {
  const generated = messages.value
    .filter((message) => message.artifact)
    .map((message) => message.artifact as string)

  return Array.from(new Set(['Execution brief', 'Runtime checklist', 'Open decisions', ...generated]))
})

const currentModel = computed(() => models.find((model) => model.value === selectedModel.value) ?? models[0])
const canSend = computed(() => prompt.value.trim().length > 0 && !isSending.value)

function formatMessage(content: string) {
  return content
    .replace(/```([\s\S]*?)```/g, '<pre><code>$1</code></pre>')
    .replace(/`([^`]+)`/g, '<code>$1</code>')
    .replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>')
    .replace(/^### (.*)$/gm, '<h3>$1</h3>')
    .replace(/^## (.*)$/gm, '<h2>$1</h2>')
    .replace(/^# (.*)$/gm, '<h1>$1</h1>')
    .replace(/\n/g, '<br>')
}

function autoResize(event: Event) {
  const textarea = event.target as HTMLTextAreaElement
  textarea.style.height = 'auto'
  textarea.style.height = `${Math.min(textarea.scrollHeight, 180)}px`
}

function newChat() {
  messages.value = [
    {
      id: Date.now(),
      role: 'assistant',
      content: 'Fresh workspace ready. What should this agent conversation accomplish?',
      meta: currentModel.value.label,
      artifact: 'Execution brief'
    }
  ]
  prompt.value = ''
  isSidebarOpen.value = false
}

async function sendMessage(text = prompt.value) {
  const content = text.trim()

  if (!content || isSending.value) return

  messages.value.push({
    id: Date.now(),
    role: 'user',
    content,
    meta: 'Just now'
  })
  prompt.value = ''
  isSending.value = true

  try {
    const response = await $fetch<{ reply?: string; meta?: string; artifact?: string }>('/api/chat', {
      method: 'POST',
      body: {
        message: content,
        model: selectedModel.value,
        messages: messages.value
      }
    })

    messages.value.push({
      id: Date.now() + 1,
      role: 'assistant',
      content:
        response.reply ??
        'I received the runtime response, but it did not include a reply field. Check the Agent-Runtime response contract.',
      meta: response.meta ?? currentModel.value.label,
      artifact: response.artifact ?? 'Runtime checklist'
    })
  } catch (error) {
    messages.value.push({
      id: Date.now() + 1,
      role: 'assistant',
      content:
        '**Runtime fallback**\n\nI could not reach Agent-Runtime yet, so I kept this turn local. Configure `NUXT_AGENT_RUNTIME_URL` when the backend is ready, and this UI will continue using the same conversation surface.',
      meta: 'Local fallback',
      artifact: 'Runtime checklist'
    })
  } finally {
    isSending.value = false
  }
}

function useSuggestion(suggestion: string) {
  prompt.value = suggestion
}
</script>

<template>
  <main class="app-shell" :class="{ 'app-shell--nav-open': isSidebarOpen }">
    <button class="mobile-nav-toggle" type="button" @click="isSidebarOpen = !isSidebarOpen">
      {{ isSidebarOpen ? 'Close' : 'Menu' }}
    </button>

    <aside class="sidebar" aria-label="Agent navigation">
      <div class="brand-lockup">
        <div class="brand-mark">A</div>
        <div>
          <p class="eyebrow">AGenNext</p>
          <h1>Agent Chat</h1>
        </div>
      </div>

      <button class="new-chat-button" type="button" @click="newChat">+ New chat</button>

      <section class="sidebar-section model-card">
        <label class="section-label" for="model">Model mode</label>
        <select id="model" v-model="selectedModel">
          <option v-for="model in models" :key="model.value" :value="model.value">
            {{ model.label }}
          </option>
        </select>
        <p>{{ currentModel.description }}</p>
      </section>

      <section class="sidebar-section">
        <p class="section-label">Runtime plane</p>
        <div class="runtime-stack">
          <span v-for="chip in runtimeChips" :key="chip">{{ chip }}</span>
        </div>
      </section>

      <section class="sidebar-section history-section">
        <p class="section-label">Recent</p>
        <button type="button">Agent orchestration brief</button>
        <button type="button">Identity handoff notes</button>
        <button type="button">Runtime debugging session</button>
      </section>
    </aside>

    <section class="chat-panel" aria-label="Chat workspace">
      <header class="chat-header">
        <div>
          <p class="eyebrow">Conversational control room</p>
          <h2>{{ currentModel.label }}</h2>
        </div>
        <div class="status-pill">
          <span aria-hidden="true"></span>
          Live-ready
        </div>
      </header>

      <div class="hero-card">
        <div>
          <p class="eyebrow">Claude / ChatGPT inspired</p>
          <h3>Calm, focused, and built for long-running agent conversations.</h3>
        </div>
        <p>
          A spacious message stream, sticky composer, quick prompts, runtime-aware fallback, and artifact rail make the interface feel premium before every service is wired up.
        </p>
      </div>

      <section class="messages" aria-live="polite">
        <article
          v-for="message in messages"
          :key="message.id"
          class="message-row"
          :class="`message-row--${message.role}`"
        >
          <div class="avatar" :aria-label="message.role">{{ message.role === 'assistant' ? 'A' : 'You' }}</div>
          <div class="message-bubble">
            <div class="message-meta">
              <strong>{{ message.role === 'assistant' ? 'AGenNext' : 'You' }}</strong>
              <span>{{ message.meta ?? 'Just now' }}</span>
            </div>
            <div class="message-content" v-html="formatMessage(message.content)" />
            <div v-if="message.role === 'assistant'" class="message-actions">
              <button type="button">Copy</button>
              <button type="button">Save artifact</button>
              <button type="button">Regenerate</button>
            </div>
          </div>
        </article>

        <article v-if="isSending" class="message-row message-row--assistant">
          <div class="avatar">A</div>
          <div class="message-bubble typing-bubble">
            <span></span>
            <span></span>
            <span></span>
          </div>
        </article>
      </section>

      <section class="suggestions" aria-label="Prompt suggestions">
        <button v-for="suggestion in suggestions" :key="suggestion" type="button" @click="useSuggestion(suggestion)">
          {{ suggestion }}
        </button>
      </section>

      <form class="composer" @submit.prevent="sendMessage()">
        <div class="composer-toolbar">
          <span>{{ currentModel.label }}</span>
          <span>Shift + Enter for newline</span>
        </div>
        <div class="composer-row">
          <textarea
            v-model="prompt"
            rows="1"
            placeholder="Message Agent Chat..."
            aria-label="Message Agent Chat"
            @input="autoResize"
            @keydown.enter.exact.prevent="sendMessage()"
          />
          <button type="submit" :disabled="!canSend" aria-label="Send message">↗</button>
        </div>
      </form>
    </section>

    <aside class="artifact-panel" aria-label="Artifacts">
      <div class="artifact-header">
        <p class="eyebrow">Artifacts</p>
        <h2>{{ activeArtifact }}</h2>
      </div>

      <div class="artifact-tabs">
        <button
          v-for="artifact in artifacts"
          :key="artifact"
          type="button"
          :class="{ active: artifact === activeArtifact }"
          @click="activeArtifact = artifact"
        >
          {{ artifact }}
        </button>
      </div>

      <div class="artifact-document">
        <p class="section-label">Preview</p>
        <h3>{{ activeArtifact }}</h3>
        <p>
          This rail is ready for Claude-style generated docs, specs, plans, code snippets, and runtime traces. Agent-Runtime can return an artifact key with any assistant response to keep this panel in sync.
        </p>
        <ul>
          <li>Conversation summary</li>
          <li>Runtime handoff status</li>
          <li>Next action checklist</li>
        </ul>
      </div>
    </aside>
  </main>
</template>
