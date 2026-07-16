<script setup lang="ts">
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { computed, ref, watch } from 'vue'

import type { PluginEnableRequest } from '../../../api/generated/types.gen'
import { activatePlugin, approvePlugin, deactivatePlugin, discoverPlugins, loadPluginLogs, loadPlugins, probePlugin } from '../api'

type Scope = PluginEnableRequest['grantedScopes'][number]
const queryClient = useQueryClient()
const selectedId = ref('')
const reviewed = ref(false)
const grants = ref<Array<Scope>>([])
const plugins = useQuery({ queryKey: ['plugins'], queryFn: loadPlugins, refetchInterval: 15_000 })
const selected = computed(() => plugins.data.value?.find((item) => item.id === selectedId.value))
const logs = useQuery({ queryKey: computed(() => ['plugin-logs', selectedId.value]), queryFn: () => loadPluginLogs(selectedId.value), enabled: computed(() => Boolean(selectedId.value)), refetchInterval: 10_000 })

watch(() => plugins.data.value, (items) => {
  const first = items?.[0]
  if (first && !items.some((item) => item.id === selectedId.value)) selectedId.value = first.id
}, { immediate: true })
watch(selected, (item) => {
  grants.value = item ? [...item.grantedScopes] : []
  reviewed.value = false
}, { immediate: true })

const refresh = useMutation({ mutationFn: discoverPlugins, onSuccess: (items) => queryClient.setQueryData(['plugins'], items) })
const trust = useMutation({ mutationFn: () => approvePlugin(selectedId.value, selected.value?.fingerprint ?? ''), onSuccess: updateSelected })
const enable = useMutation({ mutationFn: () => activatePlugin(selectedId.value, grants.value), onSuccess: updateSelected })
const disable = useMutation({ mutationFn: () => deactivatePlugin(selectedId.value), onSuccess: updateSelected })
const health = useMutation({ mutationFn: () => probePlugin(selectedId.value), onSuccess: updateSelected })

function updateSelected(item: NonNullable<typeof selected.value>) {
  queryClient.setQueryData<Array<typeof item>>(['plugins'], (current = []) => current.map((value) => value.id === item.id ? item : value))
}
function toggleScope(scope: Scope) {
  grants.value = grants.value.includes(scope) ? grants.value.filter((item) => item !== scope) : [...grants.value, scope]
}
const pending = computed(() => refresh.isPending.value || trust.isPending.value || enable.isPending.value || disable.isPending.value || health.isPending.value)
const mutationError = computed(() => refresh.error.value || trust.error.value || enable.error.value || disable.error.value || health.error.value)
</script>

<template>
  <section class="plugins-view" aria-labelledby="plugins-title">
    <header class="page-head">
      <div><p>Out-of-process adapters</p><h1 id="plugins-title">Plugins</h1><span>Review executable identity and grant only the capabilities each local tool needs.</span></div>
      <button type="button" :disabled="pending" @click="refresh.mutate()">{{ refresh.isPending.value ? 'Scanning…' : 'Refresh discovery' }}</button>
    </header>
    <p v-if="plugins.isError.value" class="state error" role="alert">Plugin registrations are unavailable. <button type="button" @click="plugins.refetch()">Retry</button></p>
    <p v-else-if="plugins.isPending.value" class="state" aria-live="polite">Loading plugin registrations…</p>
    <div v-else-if="plugins.data.value?.length" class="layout">
      <nav class="plugin-list" aria-label="Installed plugins">
        <button v-for="item in plugins.data.value" :key="item.id" type="button" :class="{ active: item.id === selectedId }" @click="selectedId = item.id">
          <span><strong>{{ item.name }}</strong><small>{{ item.id }} · {{ item.version }}</small></span>
          <i :class="`health health--${item.health}`" :title="item.health"></i>
        </button>
      </nav>
      <main v-if="selected" class="review">
        <section class="identity panel">
          <div class="panel-head"><div><p>Executable identity</p><h2>{{ selected.name }}</h2></div><span :class="`badge badge--${selected.trust}`">{{ selected.trust }}</span></div>
          <dl><div><dt>Protocol</dt><dd>{{ selected.protocolVersion }}</dd></div><div><dt>Manifest</dt><dd><code>{{ selected.manifestPath }}</code></dd></div><div class="fingerprint"><dt>SHA-256 package fingerprint</dt><dd><code>{{ selected.fingerprint }}</code></dd></div></dl>
          <p v-if="selected.lastError" class="inline-error" role="alert">{{ selected.lastError }}</p>
          <label v-if="selected.trust !== 'trusted'" class="confirmation"><input v-model="reviewed" type="checkbox"><span>I reviewed this exact fingerprint, package source, capabilities, and requested access.</span></label>
          <button v-if="selected.trust !== 'trusted'" type="button" :disabled="!reviewed || pending || !selected.available" @click="trust.mutate()">Trust exact fingerprint</button>
        </section>
        <section class="panel permissions">
          <div class="panel-head"><div><p>Least privilege</p><h2>Capability and scope review</h2></div><span>{{ selected.enabled ? 'Enabled' : 'Disabled' }}</span></div>
          <div class="capabilities"><article><small>Declared capabilities</small><ul><li v-for="capability in selected.capabilities" :key="capability">{{ capability }}</li></ul></article><article><small>Requested scopes</small><label v-for="scope in selected.requestedScopes" :key="scope"><input type="checkbox" :checked="grants.includes(scope)" :disabled="selected.enabled" @change="toggleScope(scope)"><span><strong>{{ scope }}</strong><small>{{ scope === 'project.operate' ? 'Allows only typed plugin actions queued as audited operations.' : scope === 'project.files.read' ? 'Reveals a trusted project root to the external process.' : 'Reveals bounded project identity metadata.' }}</small></span></label></article></div>
          <p class="notice">Plugins are separate local-user processes. Switchyard strips inherited secrets and mediates its own APIs, but trust still means approving locally installed code.</p>
          <div class="actions"><button v-if="!selected.enabled" type="button" :disabled="selected.trust !== 'trusted' || pending" @click="enable.mutate()">Enable reviewed grants</button><button v-else type="button" class="danger" :disabled="pending" @click="disable.mutate()">Disable and revoke</button><button type="button" :disabled="!selected.enabled || pending" @click="health.mutate()">Check health</button></div>
        </section>
        <section class="panel logs-panel">
          <div class="panel-head"><div><p>Supervision</p><h2>Health and logs</h2></div><span :class="`badge badge--${selected.health}`">{{ selected.health }}</span></div>
          <p v-if="selected.healthMessage">{{ selected.healthMessage }}</p>
          <p v-if="logs.isError.value" class="inline-error" role="alert">Plugin logs could not be read.</p>
          <ol v-else-if="logs.data.value?.length"><li v-for="entry in logs.data.value" :key="entry.id"><time :datetime="entry.createdAt">{{ new Date(entry.createdAt).toLocaleTimeString() }}</time><strong>{{ entry.level }}</strong><span>{{ entry.message }}</span></li></ol>
          <p v-else class="empty-log">No plugin process output has been captured.</p>
        </section>
        <p v-if="mutationError" class="state error" role="alert">{{ mutationError.message }}</p>
      </main>
    </div>
    <section v-else class="empty"><span aria-hidden="true">◇</span><h2>No plugins discovered</h2><p>Install a reviewed package under the local Switchyard data directory, then refresh discovery. Repository files are never executed during discovery.</p></section>
  </section>
</template>

<style scoped>
.plugins-view { max-width: 1450px; padding: 28px; margin: 0 auto; }.page-head,.panel-head,.actions { display:flex; align-items:center; justify-content:space-between; gap:16px }.page-head { align-items:flex-end; margin-bottom:20px }.page-head p,.panel-head p { margin:0 0 5px; color:var(--accent); font-size:10px; font-weight:800; letter-spacing:.13em; text-transform:uppercase }.page-head h1 { margin:0 0 5px; font-size:32px }.page-head span,.panel p,.notice { color:var(--muted) }.layout { display:grid; grid-template-columns:250px minmax(0,1fr); gap:16px }.plugin-list { display:grid; align-content:start; gap:6px; position:sticky; top:92px }.plugin-list button { display:flex; align-items:center; gap:10px; padding:12px; border:1px solid var(--border); border-radius:11px; background:var(--panel); color:var(--text); text-align:left; cursor:pointer }.plugin-list button.active { border-color:rgba(120,166,255,.5); background:rgba(120,166,255,.1); box-shadow:inset 2px 0 var(--accent) }.plugin-list span { display:grid; gap:3px; min-width:0 }.plugin-list small { color:var(--soft) }.health { width:8px; height:8px; margin-left:auto; border-radius:50%; background:var(--soft) }.health--healthy { background:var(--green) }.health--degraded { background:var(--yellow) }.health--unhealthy { background:var(--red) }.review { display:grid; gap:14px; min-width:0 }.panel { padding:18px; border:1px solid var(--border); border-radius:14px; background:var(--panel) }.panel h2 { margin:0; font-size:17px }.badge { padding:4px 8px; border:1px solid var(--border); border-radius:99px; color:var(--muted); text-transform:capitalize }.badge--trusted,.badge--healthy { color:var(--green) }.badge--changed,.badge--unhealthy { color:var(--red) }.badge--degraded { color:var(--yellow) }dl { display:grid; grid-template-columns:repeat(2,minmax(0,1fr)); gap:10px; margin:16px 0 }dl div { display:grid; gap:5px; padding:10px; border:1px solid var(--border); border-radius:9px; background:var(--panel-2) }.fingerprint { grid-column:1/-1 }dt,.capabilities small { color:var(--soft); font-size:9px; text-transform:uppercase; letter-spacing:.08em }dd { margin:0; overflow-wrap:anywhere }.confirmation,.permissions label { display:flex; align-items:flex-start; gap:9px; margin:12px 0; color:var(--muted) }.capabilities { display:grid; grid-template-columns:minmax(180px,.7fr) minmax(280px,1.3fr); gap:14px; margin-top:14px }.capabilities article { padding:13px; border:1px solid var(--border); border-radius:10px; background:var(--panel-2) }.capabilities ul { margin:10px 0 0; padding-left:18px }.permissions label span { display:grid; gap:3px }.permissions label small { color:var(--muted); text-transform:none; letter-spacing:0 }.notice { padding:11px; border-left:2px solid var(--yellow); background:rgba(241,199,91,.05); line-height:1.5 }.actions { justify-content:flex-start }.actions .danger { color:var(--red) }.logs-panel ol { display:grid; gap:6px; max-height:260px; margin:14px 0 0; padding:0; overflow:auto; list-style:none }.logs-panel li { display:grid; grid-template-columns:80px 60px minmax(0,1fr); gap:8px; padding:8px; border-bottom:1px solid var(--border); font-family:ui-monospace,monospace; font-size:10px }.logs-panel time,.logs-panel strong,.empty-log { color:var(--soft) }.state,.empty { padding:28px; border:1px solid var(--border); border-radius:14px; background:var(--panel) }.error,.inline-error { color:var(--red) }.inline-error { padding:10px; border:1px solid rgba(255,115,115,.3); border-radius:8px; background:rgba(255,115,115,.06) }.empty { display:grid; justify-items:center; gap:8px; padding:60px; text-align:center }.empty h2,.empty p { margin:0 }.empty p { max-width:600px; color:var(--muted); line-height:1.6 }button { padding:8px 11px; border:1px solid var(--border); border-radius:8px; background:var(--panel-2); color:var(--text); cursor:pointer }button:disabled { cursor:not-allowed; opacity:.5 }code { font-size:10px }@media(max-width:850px){.layout,.capabilities { grid-template-columns:1fr }.plugin-list { position:static }.page-head { align-items:flex-start; flex-direction:column }dl { grid-template-columns:1fr }.fingerprint { grid-column:auto }}
</style>
