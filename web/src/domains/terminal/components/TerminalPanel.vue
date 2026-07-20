<script setup lang="ts">
import '@xterm/xterm/css/xterm.css'
import type { TerminalPanelProps } from '../composables/useTerminalPanel'
import { useTerminalPanel } from '../composables/useTerminalPanel'

const props = defineProps<TerminalPanelProps>()
const emit = defineEmits<{ external: [] }>()
const {
  terminalHost,
  selectedKind,
  selectedShell,
  selectedService,
  selectedEnvironment,
  selectedProvider,
  selectedDatabase,
  selectedAction,
  attached,
  connectionState,
  notice,
  sessions,
  interactiveActions,
  activeSessions,
  create,
  terminate,
  attach,
} = useTerminalPanel(props)
</script>

<template>
  <article class="terminal-panel">
    <header class="terminal-toolbar">
      <div>
        <p>Daemon-owned PTY</p>
        <h2>Interactive terminal</h2>
      </div>
      <span class="connection" :class="`connection--${connectionState}`"
        ><i></i>{{ connectionState }}</span
      >
    </header>
    <form class="launcher" @submit.prevent="create.mutate()">
      <label
        >Launch<select v-model="selectedKind">
          <option value="shell">Project shell</option>
          <option value="service">Service shell</option>
          <option value="database">Database client</option>
          <option value="agent">Coding agent</option>
          <option v-if="interactiveActions.length" value="action">Interactive action</option>
        </select></label
      >
      <label v-if="['service', 'database'].includes(selectedKind)"
        >Service<select v-model="selectedService" required>
          <option disabled value="">Select service</option>
          <option v-for="service in services" :key="service" :value="service">{{ service }}</option>
        </select></label
      >
      <label v-if="['shell', 'service'].includes(selectedKind)"
        >Shell<select v-model="selectedShell">
          <option value="">User default</option>
          <option value="sh">sh</option>
          <option value="bash">bash</option>
          <option value="zsh">zsh</option>
        </select></label
      >
      <label v-if="selectedKind === 'database'"
        >Client<select v-model="selectedDatabase">
          <option>psql</option>
          <option>mysql</option>
          <option>redis-cli</option>
          <option>mongosh</option>
          <option>sqlite3</option>
        </select></label
      >
      <label v-if="selectedKind === 'agent'"
        >Provider<select v-model="selectedProvider">
          <option value="codex">Codex</option>
          <option value="claude">Claude Code</option>
        </select></label
      >
      <label v-if="selectedKind === 'action'"
        >Action<select v-model="selectedAction" required>
          <option disabled value="">Select action</option>
          <option v-for="action in interactiveActions" :key="action.id" :value="action.id">
            {{ action.name }}
          </option>
        </select></label
      >
      <label v-if="environments.length"
        >Checkout<select v-model="selectedEnvironment">
          <option value="">Primary checkout</option>
          <option v-for="environment in environments" :key="environment.id" :value="environment.id">
            {{ environment.name }}
          </option>
        </select></label
      >
      <button type="submit" :disabled="create.isPending.value">
        {{ create.isPending.value ? 'Starting…' : 'New session' }}
      </button>
      <button
        type="button"
        :disabled="!externalAvailable"
        :title="
          externalAvailable
            ? 'Open the project root in the operating system terminal'
            : 'No external terminal action is available'
        "
        @click="emit('external')"
      >
        Open external terminal
      </button>
    </form>
    <p v-if="create.isError.value" class="terminal-message terminal-message--error" role="alert">
      {{ create.error.value?.message }}
    </p>
    <p v-if="notice" class="terminal-message" role="status">{{ notice }}</p>
    <div class="session-layout">
      <aside aria-label="Terminal sessions">
        <button
          v-for="session in activeSessions"
          :key="session.id"
          type="button"
          :aria-current="attached?.id === session.id"
          @click="attach(session)"
        >
          <strong>{{ session.displayName }}</strong
          ><small
            >{{ session.status }} · {{ session.environmentId ? 'worktree' : 'primary' }}</small
          >
        </button>
        <p v-if="sessions.isPending.value">Loading sessions…</p>
        <p v-else-if="sessions.isError.value">Sessions unavailable.</p>
        <p v-else-if="!activeSessions.length">No active sessions.</p>
      </aside>
      <section class="terminal-stage" aria-label="Interactive terminal output">
        <div ref="terminalHost" class="terminal-host"></div>
        <div v-if="!attached" class="terminal-empty">
          <strong>Start or reconnect to a session</strong
          ><span
            >Disconnecting detaches the browser. Output is held in a bounded 1 MiB memory buffer and
            is never written to SQLite.</span
          >
        </div>
      </section>
    </div>
    <footer v-if="attached" class="terminal-footer">
      <span
        ><strong>{{ attached.displayName }}</strong> · {{ attached.workingDirectory }}</span
      ><button
        type="button"
        :disabled="terminate.isPending.value"
        @click="terminate.mutate(attached.id)"
      >
        Terminate process
      </button>
    </footer>
  </article>
</template>

<style scoped>
.terminal-panel {
  min-width: 0;
  overflow: hidden;
  border: 1px solid var(--border);
  border-radius: 13px;
  background: #0a0e14;
}
.terminal-toolbar,
.launcher,
.terminal-footer {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 12px 14px;
  border-bottom: 1px solid var(--border);
  background: var(--panel);
}
.terminal-toolbar {
  justify-content: space-between;
}
.terminal-toolbar p {
  margin: 0;
  color: var(--accent);
  font-size: 9px;
  font-weight: 800;
  letter-spacing: 0.12em;
  text-transform: uppercase;
}
.terminal-toolbar h2 {
  margin: 3px 0 0;
  font-size: 15px;
}
.connection {
  display: flex;
  align-items: center;
  gap: 6px;
  color: var(--muted);
  font-size: 10px;
  text-transform: uppercase;
}
.connection i {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: currentColor;
}
.connection--connected {
  color: var(--green);
}
.connection--error {
  color: var(--red);
}
.connection--connecting {
  color: var(--yellow);
}
.launcher {
  flex-wrap: wrap;
}
.launcher label {
  display: grid;
  gap: 3px;
  color: var(--soft);
  font-size: 9px;
  text-transform: uppercase;
}
.launcher select,
.launcher button,
.terminal-footer button {
  min-height: 32px;
  padding: 6px 9px;
  border: 1px solid var(--border);
  border-radius: 7px;
  background: var(--panel-2);
  color: var(--text);
}
.launcher button:first-of-type {
  align-self: end;
  border-color: rgba(120, 166, 255, 0.5);
  background: var(--accent);
  color: #07111d;
  font-weight: 800;
}
.launcher button:last-child {
  align-self: end;
}
.terminal-message {
  margin: 0;
  padding: 8px 14px;
  border-bottom: 1px solid var(--border);
  color: var(--yellow);
  font-size: 11px;
}
.terminal-message--error {
  color: var(--red);
}
.session-layout {
  display: grid;
  grid-template-columns: 190px minmax(0, 1fr);
  min-height: 500px;
}
.session-layout aside {
  padding: 8px;
  border-right: 1px solid var(--border);
  overflow: auto;
  background: #0c1118;
}
.session-layout aside button {
  display: grid;
  width: 100%;
  gap: 3px;
  margin-bottom: 5px;
  padding: 9px;
  border: 1px solid transparent;
  border-radius: 7px;
  background: transparent;
  color: var(--text);
  text-align: left;
}
.session-layout aside button:hover,
.session-layout aside button[aria-current='true'] {
  border-color: var(--border);
  background: var(--panel-2);
}
.session-layout aside small,
.session-layout aside p {
  color: var(--soft);
  font-size: 9px;
}
.terminal-stage {
  position: relative;
  min-width: 0;
  padding: 10px;
  background: #080c11;
}
.terminal-host {
  width: 100%;
  height: 480px;
}
.terminal-empty {
  position: absolute;
  inset: 0;
  display: grid;
  place-content: center;
  justify-items: center;
  gap: 8px;
  padding: 24px;
  background: #080c11;
  color: var(--muted);
  text-align: center;
}
.terminal-empty strong {
  color: var(--text);
}
.terminal-empty span {
  max-width: 520px;
  font-size: 11px;
  line-height: 1.6;
}
.terminal-footer {
  justify-content: space-between;
  border-top: 1px solid var(--border);
  border-bottom: 0;
  color: var(--muted);
  font-size: 10px;
}
.terminal-footer span {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.terminal-footer button {
  color: var(--red);
}
@media (max-width: 760px) {
  .session-layout {
    grid-template-columns: 1fr;
  }
  .session-layout aside {
    display: flex;
    gap: 5px;
    border-right: 0;
    border-bottom: 1px solid var(--border);
  }
  .session-layout aside button {
    width: auto;
    min-width: 160px;
  }
  .terminal-host {
    height: 420px;
  }
}
</style>
