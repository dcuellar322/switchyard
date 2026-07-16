<script setup lang="ts">
import SystemStatusCard from '../components/SystemStatusCard.vue'
import { useEventConnection } from '../composables/useEventConnection'
import { useSystemInfo } from '../composables/useSystemInfo'

const system = useSystemInfo()
const events = useEventConnection()
</script>

<template>
  <section class="system-view" aria-labelledby="system-title">
    <header>
      <div>
        <p class="eyebrow">Control plane</p>
        <h1 id="system-title">Switchyard is taking shape.</h1>
        <p class="subtitle">The real daemon, database, API client, and event stream are connected end to end.</p>
      </div>
      <span class="connection" :class="`connection--${events}`">
        <span aria-hidden="true"></span>
        Event stream: {{ events }}
      </span>
    </header>

    <div v-if="system.isPending.value" class="state-panel" aria-live="polite">Connecting to the local daemon…</div>
    <div v-else-if="system.isError.value" class="state-panel state-panel--error" role="alert">
      <strong>Daemon unavailable</strong>
      <span>Start the daemon and launch this page with <code>switchyard ui</code>, then retry.</span>
      <button type="button" @click="system.refetch()">Retry connection</button>
    </div>
    <div v-else-if="system.data.value" class="status-grid">
      <SystemStatusCard
        label="Daemon"
        :value="system.data.value.status"
        detail="Loopback-only local control plane"
        tone="ready"
      />
      <SystemStatusCard label="Version" :value="system.data.value.version" :detail="`Commit ${system.data.value.commit}`" />
      <SystemStatusCard
        label="Database schema"
        :value="String(system.data.value.databaseSchemaVersion)"
        detail="SQLite migration state"
      />
    </div>

    <aside class="scope-note">
      <strong>Phase 1 walking skeleton</strong>
      <p>Project lifecycle controls arrive only after the operations kernel and trusted catalog exist.</p>
    </aside>
  </section>
</template>

<style scoped>
.system-view {
  max-width: 1240px;
  padding: 54px 28px;
  margin: 0 auto;
}

header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 24px;
  margin-bottom: 28px;
}

.eyebrow {
  margin: 0 0 10px;
  color: var(--accent);
  font-size: 11px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.14em;
}

h1 {
  max-width: 700px;
  margin: 0;
  font-size: clamp(30px, 5vw, 54px);
  line-height: 1.04;
  letter-spacing: -0.045em;
}

.subtitle {
  max-width: 680px;
  margin: 16px 0 0;
  color: var(--muted);
  font-size: 16px;
  line-height: 1.6;
}

.connection {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  padding: 7px 10px;
  border: 1px solid var(--border);
  border-radius: 999px;
  color: var(--muted);
  white-space: nowrap;
}

.connection span {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--yellow);
}

.connection--connected span {
  background: var(--green);
  box-shadow: 0 0 12px rgba(84, 212, 154, 0.6);
}

.connection--disconnected span {
  background: var(--red);
}

.status-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 14px;
}

.state-panel,
.scope-note {
  padding: 20px;
  border: 1px solid var(--border);
  border-radius: 14px;
  background: var(--panel);
  color: var(--muted);
}

.state-panel--error {
  display: grid;
  gap: 10px;
  border-color: rgba(255, 115, 115, 0.45);
}

.state-panel button {
  width: fit-content;
  padding: 8px 12px;
  border: 0;
  border-radius: 8px;
  background: var(--accent);
  color: #07111f;
  font-weight: 700;
}

.scope-note {
  margin-top: 18px;
  background: rgba(120, 166, 255, 0.055);
}

.scope-note p {
  margin: 6px 0 0;
  color: var(--muted);
}

@media (max-width: 900px) {
  header {
    display: grid;
  }

  .status-grid {
    grid-template-columns: 1fr;
  }
}
</style>
