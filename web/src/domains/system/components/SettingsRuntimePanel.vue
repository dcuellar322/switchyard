<script setup lang="ts">
import { computed } from 'vue'

import { formatBytes } from '../../../lib/format'
import { useHostObservation } from '../composables/useHostObservation'
import { useSystemInfo } from '../composables/useSystemInfo'

const system = useSystemInfo()
const host = useHostObservation()
const uptime = computed(() =>
  system.data.value ? Date.now() - Date.parse(system.data.value.startedAt) : 0,
)
</script>

<template>
  <article class="settings-panel">
    <div class="settings-panel__head">
      <div>
        <p>Runtime identity</p>
        <h2>Switchyard daemon</h2>
      </div>
      <span :class="{ ready: system.data.value }">{{
        system.data.value ? 'ready' : 'unavailable'
      }}</span>
    </div>
    <p v-if="system.isError.value" class="settings-message error" role="alert">
      Daemon information is unavailable.
      <button type="button" @click="system.refetch()">Retry</button>
    </p>
    <dl class="settings-facts">
      <div>
        <dt>Version</dt>
        <dd>{{ system.data.value?.version ?? '—' }}</dd>
      </div>
      <div>
        <dt>Commit</dt>
        <dd>
          <code>{{ system.data.value?.commit ?? '—' }}</code>
        </dd>
      </div>
      <div>
        <dt>API / schema</dt>
        <dd>
          {{ system.data.value?.apiVersion ?? '—' }} /
          {{ system.data.value?.databaseSchemaVersion ?? '—' }}
        </dd>
      </div>
      <div>
        <dt>Uptime snapshot</dt>
        <dd>
          {{
            uptime
              ? `${Math.floor(uptime / 3_600_000)}h ${Math.floor((uptime % 3_600_000) / 60_000)}m`
              : '—'
          }}
        </dd>
      </div>
    </dl>
  </article>
  <article class="settings-panel">
    <div class="settings-panel__head">
      <div>
        <p>Capabilities</p>
        <h2>Host observation</h2>
      </div>
      <button type="button" @click="host.refetch()">Refresh</button>
    </div>
    <dl class="settings-facts">
      <div>
        <dt>CPU</dt>
        <dd>{{ host.data.value ? `${host.data.value.cpuPercent.toFixed(1)}%` : '—' }}</dd>
      </div>
      <div>
        <dt>Memory</dt>
        <dd>
          {{
            host.data.value
              ? `${formatBytes(host.data.value.memoryUsedBytes)} / ${formatBytes(host.data.value.memoryTotalBytes)}`
              : '—'
          }}
        </dd>
      </div>
      <div>
        <dt>Docker</dt>
        <dd>{{ host.data.value?.docker.connected ? 'Connected' : 'Unavailable' }}</dd>
      </div>
      <div>
        <dt>Storage attribution</dt>
        <dd>{{ host.data.value?.docker.attribution ?? 'unknown' }}</dd>
      </div>
    </dl>
    <p v-if="host.data.value?.warnings.length" class="settings-message warning">
      {{ host.data.value.warnings.join(' ') }}
    </p>
  </article>
</template>
