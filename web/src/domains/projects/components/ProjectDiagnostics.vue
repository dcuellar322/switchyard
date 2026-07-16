<script setup lang="ts">
import { useQuery } from '@tanstack/vue-query'
import { computed, ref, watch } from 'vue'

import type { Project, RuntimeLogEntry } from '../../../api/generated/types.gen'
import { loadProjectHealth, loadProjectLogs, loadProjectRuntime } from '../api'
import { useProjectLogStream } from '../composables/useProjectLogStream'

const props = defineProps<{ project: Project }>()
const projectId = computed(() => props.project.id)
const runtime = useQuery({ queryKey: ['project-runtime', projectId], queryFn: () => loadProjectRuntime(projectId.value), refetchInterval: 5_000 })
const health = useQuery({ queryKey: ['project-health', projectId], queryFn: () => loadProjectHealth(projectId.value), refetchInterval: 5_000 })
const history = useQuery({ queryKey: ['project-logs', projectId], queryFn: () => loadProjectLogs(projectId.value) })
const logEntries = ref<Array<RuntimeLogEntry>>([])
const services = computed(() => runtime.data.value?.services ?? [])
const healthResults = computed(() => health.data.value?.results ?? [])

function mergeLogs(entries: Array<RuntimeLogEntry>) {
  const bySequence = new Map(logEntries.value.map((entry) => [entry.sequence, entry]))
  for (const entry of entries) bySequence.set(entry.sequence, entry)
  logEntries.value = [...bySequence.values()].sort((left, right) => left.sequence - right.sequence).slice(-200)
}

watch(() => history.data.value, (entries) => entries && mergeLogs(entries), { immediate: true })
const logStream = useProjectLogStream(projectId, (entry) => mergeLogs([entry]))

const stateLabel = computed(() => runtime.data.value?.state.replaceAll('_', ' ') ?? 'unknown')
const stateTone = computed(() => {
  if (runtime.data.value?.state === 'degraded' || runtime.data.value?.state === 'failed') return 'danger'
  if (runtime.data.value?.state === 'running') return 'ready'
  return 'muted'
})

async function refresh() {
  await Promise.all([runtime.refetch(), health.refetch(), history.refetch()])
}
</script>

<template>
  <section class="diagnostics" :aria-labelledby="`diagnostics-${project.id}`">
    <header class="diagnostics__heading">
      <div>
        <p class="eyebrow">Project diagnostics</p>
        <h2 :id="`diagnostics-${project.id}`">{{ project.displayName }}</h2>
        <code>{{ project.primaryLocation }}</code>
      </div>
      <div class="diagnostics__actions">
        <span class="state-badge" :class="`state-badge--${stateTone}`">{{ stateLabel }}</span>
        <button class="button--secondary" type="button" @click="refresh">Refresh</button>
      </div>
    </header>

    <p v-if="runtime.isPending.value || health.isPending.value" class="state-panel" aria-live="polite">Loading current observations…</p>
    <p v-else-if="runtime.isError.value || health.isError.value" class="message message--error" role="alert">Project diagnostics are temporarily unavailable.</p>
    <p v-else-if="health.data.value?.observerState === 'disconnected'" class="observer-banner observer-banner--danger" role="status">Runtime observer disconnected. Last-known health is not treated as current.</p>
    <p v-else-if="health.data.value?.observerState === 'stale'" class="observer-banner" role="status">Health observations are stale. Refresh before acting on this state.</p>

    <div class="diagnostics__grid">
      <article class="diagnostic-card diagnostic-card--services">
        <div class="card-heading"><div><p class="eyebrow">Runtime</p><h3>Services</h3></div><span>{{ runtime.data.value?.driver ?? '—' }}</span></div>
        <div v-if="services.length" class="service-table" role="table" aria-label="Runtime services">
          <div class="service-row service-row--header" role="row"><span>Service</span><span>State</span><span>Health</span><span>Binding</span></div>
          <div v-for="service in services" :key="service.id" class="service-row" role="row">
            <strong>{{ service.id }}</strong><span>{{ service.state }}</span><span>{{ service.health || 'not reported' }}</span>
            <span>{{ (service.ports ?? []).map((port) => port.hostPort).filter(Boolean).join(', ') || '—' }}</span>
          </div>
        </div>
        <p v-else class="empty">No service observations are available.</p>
      </article>

      <article class="diagnostic-card">
        <div class="card-heading"><div><p class="eyebrow">Readiness</p><h3>Health checks</h3></div><span>{{ health.data.value?.status ?? 'unknown' }}</span></div>
        <ul v-if="healthResults.length" class="health-list">
          <li v-for="check in healthResults" :key="`${check.serviceId}-${check.checkId}`">
            <span class="health-dot" :class="`health-dot--${check.status}`" aria-hidden="true"></span>
            <span><strong>{{ check.serviceId }} · {{ check.checkId }}</strong><small>{{ check.message }} · {{ check.latencyMs }} ms</small></span>
            <em>{{ check.required ? 'required' : check.severity }}</em>
          </li>
        </ul>
        <p v-else class="empty">No health checks are declared for this project.</p>
      </article>

      <article class="diagnostic-card diagnostic-card--logs">
        <div class="card-heading">
          <div><p class="eyebrow">Persisted and live</p><h3>Log preview</h3></div>
          <span class="connection" :class="`connection--${logStream.state.value}`"><i aria-hidden="true"></i>{{ logStream.state.value }}</span>
        </div>
        <ol v-if="logEntries.length" class="log-preview" aria-label="Project logs" aria-live="polite">
          <li v-for="entry in logEntries" :key="entry.sequence">
            <time :datetime="entry.timestamp">{{ new Date(entry.timestamp).toLocaleTimeString() }}</time>
            <strong>{{ entry.serviceId }}</strong><span :class="`log-level log-level--${entry.level}`">{{ entry.stream }}</span><code>{{ entry.message }}</code>
            <span v-if="entry.redacted" class="redacted-note">redacted</span>
          </li>
        </ol>
        <p v-else class="empty log-empty">No persisted logs yet. New runtime output will appear here automatically.</p>
      </article>
    </div>
  </section>
</template>

<style scoped>
.diagnostics{margin-top:24px;padding-top:24px;border-top:1px solid var(--border)}
.diagnostics__heading,.diagnostics__actions,.card-heading,.connection{display:flex;align-items:center}.diagnostics__heading{justify-content:space-between;gap:20px;margin-bottom:16px}.diagnostics__heading h2{margin:4px 0 5px;font-size:24px}.diagnostics__heading code{color:var(--muted);font-size:11px}.diagnostics__actions{gap:10px}.state-badge{padding:6px 10px;border-radius:999px;background:#202938;color:var(--muted);font-size:11px;text-transform:capitalize}.state-badge--ready{background:rgba(84,212,154,.1);color:var(--green)}.state-badge--danger{background:rgba(255,115,115,.1);color:var(--red)}
.observer-banner{padding:10px 12px;border:1px solid rgba(241,199,91,.3);border-radius:9px;background:rgba(241,199,91,.08);color:var(--yellow)}.observer-banner--danger{border-color:rgba(255,115,115,.3);background:rgba(255,115,115,.08);color:var(--red)}
.diagnostics__grid{display:grid;grid-template-columns:1.2fr .8fr;gap:14px}.diagnostic-card{min-width:0;padding:18px;border:1px solid var(--border);border-radius:12px;background:#0f141c}.diagnostic-card--logs{grid-column:1/-1}.card-heading{justify-content:space-between;margin-bottom:13px}.card-heading h3{margin:3px 0 0;font-size:16px}.card-heading>span{color:var(--soft);font-size:11px;text-transform:uppercase}
.service-table{border:1px solid var(--border);border-radius:8px;overflow:hidden}.service-row{display:grid;grid-template-columns:1.2fr 1fr 1fr .7fr;gap:10px;padding:9px 10px;border-top:1px solid var(--border);font-size:12px}.service-row:first-child{border-top:0}.service-row span{color:var(--muted)}.service-row--header{background:#0b1017;color:var(--soft);font-size:10px;text-transform:uppercase;letter-spacing:.07em}
.health-list{list-style:none;margin:0;padding:0;display:grid;gap:8px}.health-list li{display:grid;grid-template-columns:8px 1fr auto;gap:9px;align-items:center;padding:9px;border-radius:8px;background:#0b1017}.health-list span:nth-child(2){display:grid;gap:3px}.health-list small{color:var(--muted)}.health-list em{color:var(--soft);font-size:10px;font-style:normal}.health-dot{width:7px;height:7px;border-radius:50%;background:var(--soft)}.health-dot--healthy{background:var(--green)}.health-dot--unhealthy{background:var(--red)}
.connection{gap:6px!important;text-transform:none!important}.connection i{width:7px;height:7px;border-radius:50%;background:var(--yellow)}.connection--connected i{background:var(--green)}.connection--disconnected i{background:var(--red)}.log-preview{list-style:none;margin:0;padding:0;max-height:280px;overflow:auto;border:1px solid var(--border);border-radius:8px;background:#080c11}.log-preview li{display:grid;grid-template-columns:78px 90px 48px minmax(0,1fr) auto;gap:9px;padding:7px 10px;border-top:1px solid rgba(37,48,68,.65);align-items:baseline;font-size:11px}.log-preview li:first-child{border-top:0}.log-preview time,.log-level{color:var(--soft)}.log-preview code{overflow-wrap:anywhere;color:#c5d1df}.redacted-note{color:var(--yellow);font-size:9px;text-transform:uppercase}.empty{margin:0;padding:14px;color:var(--muted);text-align:center}.log-empty{background:#080c11;border-radius:8px}
@media(max-width:900px){.diagnostics__grid{grid-template-columns:1fr}.diagnostic-card--logs{grid-column:auto}.diagnostics__heading{align-items:flex-start}.service-row{grid-template-columns:1fr 1fr}.service-row--header{display:none}.log-preview li{grid-template-columns:70px 1fr}.log-preview code{grid-column:1/-1}}
</style>
