<script setup lang="ts">
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { computed, ref, watch } from 'vue'

import type { TelemetryStatus } from '../../../api/generated/types.gen'
import { disableTelemetry, enableTelemetry, loadTelemetryStatus, sendTelemetry } from '../api'

const queryClient = useQueryClient()
const endpoint = ref('')
const reviewed = ref(false)
const status = useQuery({ queryKey: ['telemetry'], queryFn: loadTelemetryStatus })

watch(
  () => status.data.value?.settings.endpoint,
  (value) => {
    if (value) endpoint.value = value
  },
  { immediate: true },
)

function update(value: TelemetryStatus) {
  queryClient.setQueryData(['telemetry'], value)
  reviewed.value = false
}

const enable = useMutation({ mutationFn: () => enableTelemetry(endpoint.value), onSuccess: update })
const disable = useMutation({ mutationFn: disableTelemetry, onSuccess: update })
const send = useMutation({ mutationFn: sendTelemetry, onSuccess: update })
const pending = computed(
  () => enable.isPending.value || disable.isPending.value || send.isPending.value,
)
const error = computed(
  () => status.error.value || enable.error.value || disable.error.value || send.error.value,
)
</script>

<template>
  <article class="telemetry-panel">
    <div class="panel-head">
      <div>
        <p>Privacy</p>
        <h2>Anonymous usage counters</h2>
      </div>
      <span :class="{ ready: status.data.value?.settings.enabled }">{{
        status.isPending.value
          ? 'loading'
          : status.data.value?.settings.enabled
            ? 'opted in'
            : 'off by default'
      }}</span>
    </div>
    <p class="summary">
      Switchyard sends nothing unless you opt in. The complete payload is limited to the
      installation ID, build and OS, architecture, fixed counters below, and a timestamp—never
      projects, paths, logs, commands, or machine identities.
    </p>
    <p v-if="error" class="inline-error" role="alert">{{ error.message }}</p>
    <div v-if="status.isPending.value" class="state">Loading consent status…</div>
    <template v-else-if="status.data.value">
      <div class="payload" aria-label="Pending anonymous counters">
        <dl v-if="status.data.value.preview" class="identity">
          <div>
            <dt>Schema</dt>
            <dd>{{ status.data.value.preview.schemaVersion }}</dd>
          </div>
          <div>
            <dt>Installation</dt>
            <dd>
              <code>{{ status.data.value.preview.installationId }}</code>
            </dd>
          </div>
          <div>
            <dt>Build</dt>
            <dd>{{ status.data.value.preview.version }}</dd>
          </div>
          <div>
            <dt>Platform</dt>
            <dd>{{ status.data.value.preview.os }}/{{ status.data.value.preview.architecture }}</dd>
          </div>
          <div>
            <dt>Generated</dt>
            <dd>{{ new Date(status.data.value.preview.generatedAt).toLocaleString() }}</dd>
          </div>
        </dl>
        <span v-if="status.data.value.counters.length === 0">No counters are pending.</span>
        <dl v-else>
          <div v-for="counter in status.data.value.counters" :key="counter.name">
            <dt>{{ counter.name }}</dt>
            <dd>{{ counter.value }}</dd>
          </div>
        </dl>
        <small v-if="status.data.value.settings.enabled"
          >Destination: {{ status.data.value.settings.endpoint }}</small
        >
      </div>
      <template v-if="!status.data.value.settings.enabled">
        <label class="endpoint"
          ><span>HTTPS collection endpoint</span
          ><input
            v-model.trim="endpoint"
            type="url"
            inputmode="url"
            placeholder="https://metrics.example.com/v1"
            autocomplete="off"
        /></label>
        <label class="consent"
          ><input v-model="reviewed" type="checkbox" /><span
            >I reviewed the exact payload and destination and choose to opt in.</span
          ></label
        >
        <button
          type="button"
          :disabled="!reviewed || !endpoint || pending"
          @click="enable.mutate()"
        >
          Enable anonymous counters
        </button>
      </template>
      <div v-else class="actions">
        <button type="button" :disabled="pending" @click="send.mutate()">Send now</button>
        <button type="button" class="danger" :disabled="pending" @click="disable.mutate()">
          Disable and clear counters
        </button>
      </div>
    </template>
  </article>
</template>

<style scoped>
.telemetry-panel {
  grid-column: 1/-1;
  padding: 18px;
  border: 1px solid var(--border);
  border-radius: 13px;
  background: linear-gradient(145deg, var(--panel), #0d1219);
}
.panel-head {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 12px;
  margin-bottom: 12px;
}
.panel-head p {
  margin: 0;
  color: var(--accent);
  font-size: 9px;
  font-weight: 800;
  letter-spacing: 0.12em;
  text-transform: uppercase;
}
.panel-head h2 {
  margin: 4px 0 0;
  font-size: 17px;
}
.panel-head span {
  padding: 4px 7px;
  border: 1px solid var(--border);
  border-radius: 99px;
  color: var(--muted);
  font-size: 9px;
}
.panel-head .ready {
  color: var(--green);
  border-color: rgba(84, 212, 154, 0.28);
}
.summary,
.payload small,
.state {
  color: var(--muted);
  line-height: 1.55;
}
.payload {
  margin: 14px 0;
  padding: 12px;
  border: 1px solid var(--border);
  border-radius: 9px;
  background: #0b1017;
}
.payload dl {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(170px, 1fr));
  gap: 7px;
  margin: 0 0 9px;
}
.payload dl div {
  display: flex;
  justify-content: space-between;
  gap: 8px;
}
.payload .identity {
  padding-bottom: 10px;
  border-bottom: 1px solid var(--border);
}
.payload .identity div {
  display: grid;
  justify-content: initial;
  gap: 3px;
}
.payload dt {
  color: var(--muted);
}
.payload dd {
  margin: 0;
  font-variant-numeric: tabular-nums;
  overflow-wrap: anywhere;
}
.payload code {
  font-size: 10px;
}
.endpoint {
  display: grid;
  gap: 6px;
  color: var(--muted);
}
.endpoint input {
  width: min(100%, 540px);
  padding: 9px;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--panel-2);
  color: var(--text);
}
.consent {
  display: flex;
  align-items: flex-start;
  gap: 9px;
  margin: 13px 0;
  color: var(--muted);
}
.actions {
  display: flex;
  gap: 9px;
}
.danger {
  color: var(--red);
}
.inline-error {
  padding: 10px;
  border: 1px solid rgba(255, 115, 115, 0.3);
  border-radius: 8px;
  color: var(--red);
}
button {
  padding: 8px 11px;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--panel-2);
  color: var(--text);
}
button:disabled {
  opacity: 0.5;
}
</style>
