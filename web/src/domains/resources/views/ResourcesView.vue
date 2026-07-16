<script setup lang="ts">
import { useQuery } from '@tanstack/vue-query'
import { computed, ref, watch } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'

import { formatBytes } from '../../../lib/format'
import ResourceConsumersTable from '../components/ResourceConsumersTable.vue'
import ResourceHistoryPanel from '../components/ResourceHistoryPanel.vue'
import StorageIntelligence from '../components/StorageIntelligence.vue'
import { loadCleanupPreview, loadMetricHistory, loadResourceOverview, loadStorageInventory } from '../api'

const overview = useQuery({ queryKey: ['resource-overview'], queryFn: loadResourceOverview, refetchInterval: 10_000 })
const storage = useQuery({ queryKey: ['storage-inventory'], queryFn: loadStorageInventory, refetchInterval: 120_000 })
const route = useRoute()
const router = useRouter()
const routeProject = computed(() => typeof route.query.project === 'string' ? route.query.project : '')
const selectedProject = ref(routeProject.value)
const selectedService = ref('')
const range = ref<'1h' | '24h' | '7d'>('1h')
const cleanup = ref<Awaited<ReturnType<typeof loadCleanupPreview>>>()
const cleanupPending = ref(false)
const cleanupError = ref('')

watch(() => overview.data.value?.projects, (projects) => {
	if (!projects) return
	const firstProject = projects?.[0]
	if (!selectedProject.value && firstProject) selectedProject.value = firstProject.projectId
  if (selectedProject.value && !projects.some((project) => project.projectId === selectedProject.value)) {
    selectConsumer(firstProject?.projectId ?? '', '')
  }
}, { immediate: true })

watch(routeProject, (projectId) => {
  if (!projectId || projectId === selectedProject.value) return
  selectedProject.value = projectId
  selectedService.value = ''
})

const history = useQuery({
  queryKey: computed(() => ['resource-history', selectedProject.value, selectedService.value, range.value]),
  queryFn: () => loadMetricHistory(selectedProject.value, selectedService.value, range.value),
  enabled: computed(() => Boolean(selectedProject.value)),
  refetchInterval: 30_000,
})

const projects = computed(() => overview.data.value?.projects ?? [])
const totals = computed(() => projects.value.reduce((result, project) => ({
	cpu: result.cpu + (project.metric.cpuAvailable ? project.metric.cpuPercent : 0),
	memory: result.memory + (project.metric.memoryAvailable ? project.metric.memoryBytes : 0),
	processes: result.processes + project.metric.processCount,
	unavailable: result.unavailable + (!project.metric.cpuAvailable || !project.metric.memoryAvailable ? 1 : 0),
}), { cpu: 0, memory: 0, processes: 0, unavailable: 0 }))
const stale = computed(() => overview.data.value ? Date.now() - new Date(overview.data.value.observedAt).getTime() > 30_000 : false)
const footprintBytes = computed(() => {
  const value = overview.data.value?.footprint
  return value ? value.databaseBytes + value.databaseWalBytes + value.databaseShmBytes + value.logBytes : 0
})

function selectConsumer(projectId: string, serviceId: string) {
  if (selectedProject.value !== projectId) {
    cleanup.value = undefined
    cleanupError.value = ''
  }
  selectedProject.value = projectId
  selectedService.value = serviceId
  if (routeProject.value !== projectId) {
    void router.replace({ query: { ...route.query, project: projectId || undefined } })
  }
}

async function previewCleanup(projectId: string) {
  cleanupPending.value = true
  cleanupError.value = ''
  try {
    cleanup.value = await loadCleanupPreview(projectId)
  } catch (cause) {
    cleanupError.value = cause instanceof Error ? cause.message : 'Cleanup preview failed.'
  } finally {
    cleanupPending.value = false
  }
}

async function refreshResources() {
	await Promise.all([overview.refetch(), storage.refetch()])
}
</script>

<template>
  <section class="resources-view" aria-labelledby="resources-title">
    <header class="page-head">
      <div><p class="eyebrow">Observed capacity</p><h1 id="resources-title">Resources</h1><span>Historical project metrics and storage facts with honest attribution.</span></div>
			<button type="button" :disabled="overview.isFetching.value" @click="refreshResources">{{ overview.isFetching.value ? 'Refreshing…' : 'Refresh observations' }}</button>
    </header>

    <div v-if="overview.isPending.value" class="loading" aria-live="polite"><span></span><span></span><span></span></div>
    <div v-else-if="overview.isError.value && !overview.data.value" class="state-panel" role="alert"><strong>Resource intelligence unavailable</strong><p>Reconnect to the local daemon and retry.</p><button type="button" @click="overview.refetch()">Retry</button></div>
    <template v-else-if="overview.data.value">
      <p v-if="stale" class="banner warning" role="status">Showing the last observation from {{ new Date(overview.data.value.observedAt).toLocaleTimeString() }} while collection recovers.</p>
      <p v-if="overview.data.value.warnings.length" class="banner" role="status">Partial observation: {{ overview.data.value.warnings.join(' ') }}</p>
      <div class="summary-grid">
		<article><span>Managed CPU</span><strong>{{ totals.cpu.toFixed(1) }}%</strong><small>Percent of one core · {{ totals.processes }} processes/containers</small></article>
		<article><span>Managed memory</span><strong>{{ formatBytes(totals.memory) }}</strong><small>Latest available aggregates<span v-if="totals.unavailable"> · {{ totals.unavailable }} unavailable</span></small></article>
        <article><span>Docker storage</span><strong>{{ formatBytes(overview.data.value.storage.bytes) }}</strong><small>{{ overview.data.value.storage.classification }} · ≈ {{ formatBytes(overview.data.value.storage.reclaimableBytes) }} reclaimable</small></article>
        <article><span>Switchyard footprint</span><strong>{{ formatBytes(footprintBytes) }}</strong><small>{{ overview.data.value.footprint.metricRows }} metric rows · {{ overview.data.value.footprint.logSegments }} log segments</small></article>
      </div>

      <section v-if="projects.some((project) => project.warnings.length)" class="budget-warnings" aria-labelledby="budget-title"><div><p class="eyebrow">Sustained thresholds</p><h2 id="budget-title">Budget warnings</h2></div><ul><template v-for="project in projects" :key="project.projectId"><li v-for="warning in project.warnings" :key="warning.code"><strong>{{ project.name }} · {{ warning.resource }}</strong><span>{{ warning.message }} Limit {{ warning.limit.toLocaleString() }} {{ warning.unit }}; observed {{ warning.observed.toLocaleString() }}.</span></li></template></ul></section>

      <div v-if="!projects.length" class="state-panel"><strong>No trusted projects</strong><p>Approve a project manifest to begin bounded resource history.</p><RouterLink to="/projects">Open project onboarding</RouterLink></div>
      <template v-else>
        <ResourceConsumersTable :projects="projects" :selected-project="selectedProject" :selected-service="selectedService" @select="selectConsumer" />
        <ResourceHistoryPanel class="spaced" :projects="projects" :project-id="selectedProject" :service-id="selectedService" :range="range" :history="history.data.value" :pending="history.isPending.value" :error="history.isError.value" @select-project="(value) => selectConsumer(value, '')" @select-service="(value) => selectedService = value" @select-range="(value) => range = value" @retry="history.refetch()" />
      </template>

      <StorageIntelligence class="spaced" :inventory="storage.data.value" :projects="projects" :project-id="selectedProject" :preview="cleanup" :loading="storage.isPending.value" :pending="cleanupPending" :inventory-error="storage.isError.value" :preview-error="cleanupError" @select-project="(value) => selectConsumer(value, '')" @preview="previewCleanup" @retry="storage.refetch()" />

      <article class="panel retention spaced"><div><p class="eyebrow">Local retention</p><h2>Bounded by configuration</h2></div><dl><div><dt>Sampling</dt><dd>Every {{ overview.data.value.retention.sampleIntervalSeconds }}s for active projects</dd></div><div><dt>Exact</dt><dd>{{ Math.round(overview.data.value.retention.rawSeconds / 3600) }} hour</dd></div><div><dt>1-minute</dt><dd>{{ Math.round(overview.data.value.retention.minuteSeconds / 3600) }} hours</dd></div><div><dt>15-minute</dt><dd>{{ Math.round(overview.data.value.retention.quarterHourSeconds / 86400) }} days</dd></div><div><dt>History response</dt><dd>≤ {{ overview.data.value.retention.maximumHistoryPoints }} points</dd></div><div><dt>Logs</dt><dd>{{ Math.round(overview.data.value.retention.logSeconds / 86400) }} days or {{ formatBytes(overview.data.value.retention.logBytes) }}</dd></div></dl><p>Configure these process-owned bounds with the daemon metric and log retention flags.</p></article>
    </template>
  </section>
</template>

<style scoped>
.resources-view{width:min(100%,1500px);margin:0 auto;padding:28px}.page-head{display:flex;justify-content:space-between;align-items:flex-start;gap:18px;margin-bottom:22px}.page-head h1{margin:5px 0;font-size:27px}.page-head span{color:var(--muted)}.eyebrow{margin:0;color:var(--accent);font-size:10px;font-weight:800;letter-spacing:.13em;text-transform:uppercase}button,.state-panel a{padding:9px 12px;border:1px solid var(--border);border-radius:8px;background:var(--panel-2);color:var(--text);text-decoration:none}.summary-grid{display:grid;grid-template-columns:repeat(4,1fr);gap:12px;margin-bottom:16px}.summary-grid article,.panel,.budget-warnings{padding:16px;border:1px solid var(--border);border-radius:13px;background:linear-gradient(145deg,var(--panel),#0e131a)}.summary-grid span,.summary-grid small{display:block;color:var(--muted);font-size:10px}.summary-grid strong{display:block;margin:7px 0 4px;font-size:22px}.banner{padding:10px 12px;border:1px solid rgba(120,166,255,.25);border-radius:8px;background:rgba(120,166,255,.06);color:var(--accent)}.banner.warning{border-color:rgba(241,199,91,.25);background:rgba(241,199,91,.06);color:var(--yellow)}.budget-warnings{display:grid;grid-template-columns:220px 1fr;gap:20px;margin-bottom:16px;border-color:rgba(241,199,91,.25)}.budget-warnings h2{margin:4px 0}.budget-warnings ul{display:grid;gap:8px;margin:0;padding:0;list-style:none}.budget-warnings li{display:grid;gap:3px;padding:9px;border-radius:7px;background:rgba(241,199,91,.05)}.budget-warnings li strong{color:var(--yellow)}.budget-warnings li span,.retention p{color:var(--muted);font-size:10px}.spaced{margin-top:16px}.retention{display:grid;grid-template-columns:180px 1fr;gap:20px}.retention h2{margin:4px 0}.retention dl{display:grid;grid-template-columns:repeat(3,1fr);gap:8px;margin:0}.retention dl div{padding:9px;border:1px solid var(--border);border-radius:7px}.retention dt{color:var(--muted);font-size:9px;text-transform:uppercase}.retention dd{margin:4px 0 0}.retention p{grid-column:2}.loading{display:grid;gap:8px}.loading span{height:80px;border-radius:9px;background:var(--panel);animation:pulse 1.2s infinite}.state-panel{padding:40px;border:1px solid var(--border);border-radius:13px;background:var(--panel);text-align:center}.state-panel p{color:var(--muted)}@keyframes pulse{50%{opacity:.55}}@media(max-width:1000px){.summary-grid{grid-template-columns:1fr 1fr}.retention{grid-template-columns:1fr}.retention p{grid-column:auto}}@media(max-width:700px){.resources-view{padding:18px}.page-head,.budget-warnings{display:grid}.summary-grid,.retention dl{grid-template-columns:1fr}}
</style>
