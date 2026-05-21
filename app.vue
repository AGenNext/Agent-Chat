<script setup lang="ts">
type ChatRole = 'assistant' | 'user'

type ChatMessage = {
  id: number
  role: ChatRole
  content: string
  meta?: string
}

const prompt = ref('')
const isSending = ref(false)
const messages = ref<ChatMessage[]>([
  {
    id: 1,
    role: 'assistant',
    content:
      'Hi, I’m your AGenNext copilot. Ask me to reason through a workflow, coordinate an agent, or turn messy context into a crisp next step.',
    meta: 'Agent Runtime ready'
  }
])

const suggestions = [
  'Summarize the latest runtime state',
  'Draft a plan for a new agent workflow',
  'What should I do next?',
  'Turn this into an execution checklist'
]

const runtimeChips = ['Agent-Auth', 'Agent-Identity', 'Agent-Runtime', 'SurrealDB']

const canSend = computed(() => prompt.value.trim().length > 0 && !isSending.value)

async function sendMessage(text = prompt.value) {
  const content = text.trim()

  if (!content || isSending.value) return

  messages.value.push({
    id: Date.now(),
    role: 'user',
    content
  })
  prompt.value = ''
  isSending.value = true

  try {
    const response = await $fetch<{ reply: string; meta?: string }>('/api/chat', {
      method: 'POST',
      body: { message: content, messages: messages.value }
    })

    messages.value.push({
      id: Date.now() + 1,
      role: 'assistant',
      content: response.reply,
      meta: response.meta ?? 'Runtime response'
    })
  } catch (error) {
    messages.value.push({
      id: Date.now() + 1,
      role: 'assistant',
      content:
        'I could not reach Agent-Runtime yet. I kept this turn local, so you can continue shaping the workflow while the backend is configured.',
      meta: 'Local fallback'
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
  <main class="app-shell">
    <aside class="sidebar" aria-label="Agent navigation">
      <div class="brand-lockup">
        <div class="brand-mark">A</div>
        <div>
          <p class="eyebrow">AGenNext</p>
          <h1>Agent Chat</h1>
        </div>
      </div>

      <button class="new-chat-button" type="button">+ New chat</button>

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
          <h2>Fluid agent workspace</h2>
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
          A spacious message stream, sticky composer, quick prompts, and runtime-aware fallback make the interface feel polished before every service is wired up.
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
            <p>{{ message.content }}</p>
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
        <textarea
          v-model="prompt"
          rows="1"
          placeholder="Message Agent Chat..."
          aria-label="Message Agent Chat"
          @keydown.enter.exact.prevent="sendMessage()"
        />
        <button type="submit" :disabled="!canSend" aria-label="Send message">↗</button>
      </form>
    </section>
  </main>
</template>
