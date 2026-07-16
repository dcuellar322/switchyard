<script setup lang="ts">
import { useMutation, useQuery } from '@tanstack/vue-query'
import { computed } from 'vue'

import { loadPortRegistry, suggestPort } from '../api'

const registry = useQuery({ queryKey: ['ports'], queryFn: loadPortRegistry, refetchInterval: 5_000 })
const suggestion = useMutation({ mutationFn: () => suggestPort() })
const facts = computed(() => registry.data.value?.facts ?? [])
const conflicts = computed(() => registry.data.value?.conflicts ?? [])
const conflictingPorts = computed(() => new Set(conflicts.value.map((conflict) => conflict.port)))
const managedPorts = computed(() => new Set(facts.value.filter((fact) => fact.projectId).map((fact) => fact.port)).size)
</script>

<template>
  <section class="ports-view" aria-labelledby="ports-title">
    <header class="page-heading">
      <div><p class="eyebrow">Host intelligence</p><h1 id="ports-title">Port registry</h1><p>Reserved, configured, and currently bound local ports with their evidence.</p></div>
      <button type="button" :disabled="suggestion.isPending.value" @click="suggestion.mutate()">{{ suggestion.isPending.value ? 'Checking…' : 'Find next free port' }}</button>
    </header>

    <div v-if="registry.isPending.value" class="state-panel" aria-live="polite">Inspecting manifests, runtimes, and OS listeners…</div>
    <div v-else-if="registry.isError.value" class="state-panel state-panel--error" role="alert"><strong>Port registry unavailable</strong><button type="button" @click="registry.refetch()">Retry</button></div>
    <template v-else>
      <div class="summary-grid">
        <article><span>Managed ports</span><strong>{{ managedPorts }}</strong><small>Across trusted projects</small></article>
        <article><span>Current facts</span><strong>{{ facts.length }}</strong><small>Declarations, leases, listeners</small></article>
        <article :class="{ danger: conflicts.length }"><span>Conflicts</span><strong>{{ conflicts.length }}</strong><small>{{ conflicts.length ? 'Action recommended' : 'No overlap detected' }}</small></article>
      </div>

      <p v-if="suggestion.data.value" class="suggestion" role="status"><strong>{{ suggestion.data.value.port }}</strong>/tcp is free in the preferred 15000–19999 range.</p>
      <p v-else-if="suggestion.isError.value" class="message message--error" role="alert">No free port could be suggested. Refresh the registry and try again.</p>
      <div v-for="warning in registry.data.value?.warnings ?? []" :key="warning" class="warning">Partial data: {{ warning }}</div>

      <article class="panel">
        <div class="panel-heading"><div><p class="eyebrow">Visual map</p><h2>Observed ports</h2></div><span>Live refresh · 5s</span></div>
        <div v-if="facts.length" class="port-map">
          <div v-for="fact in facts" :key="fact.id" class="port-block" :class="{ conflict: conflictingPorts.has(fact.port) }">
            <strong>{{ fact.port }}<span v-if="conflictingPorts.has(fact.port)"> ⚠</span></strong>
            <small>{{ fact.projectName ?? 'Unknown process' }} · {{ fact.serviceId ?? fact.kind }}</small>
          </div>
          <div v-if="suggestion.data.value" class="port-block port-block--free"><strong>{{ suggestion.data.value.port }}</strong><small>Suggested next free</small></div>
        </div>
        <p v-else class="empty">No port evidence is available yet.</p>
      </article>

      <article class="panel">
        <div class="panel-heading"><div><p class="eyebrow">Evidence</p><h2>All port facts</h2></div><span>{{ registry.data.value?.observedAt ? new Date(registry.data.value.observedAt).toLocaleTimeString() : '—' }}</span></div>
        <div v-if="facts.length" class="port-table" role="table" aria-label="All port facts">
          <div class="port-row port-row--header" role="row"><span>Port</span><span>Project</span><span>Service</span><span>Source</span><span>State</span></div>
          <div v-for="fact in facts" :key="`row-${fact.id}`" class="port-row" role="row">
            <strong :class="{ conflict: conflictingPorts.has(fact.port) }">{{ fact.port }}/{{ fact.protocol }}</strong><span>{{ fact.projectName ?? 'Unknown process' }}</span><span>{{ fact.serviceId ?? '—' }}</span><span :title="fact.evidence">{{ fact.source }}</span><em>{{ conflictingPorts.has(fact.port) ? 'conflict' : fact.kind }}</em>
          </div>
        </div>
        <p v-else class="empty">Register and trust a project to protect its declared ports.</p>
      </article>
    </template>
  </section>
</template>

<style scoped>
.ports-view{max-width:1240px;padding:42px 28px;margin:0 auto}.page-heading,.panel-heading{display:flex;justify-content:space-between;gap:20px;align-items:flex-start}.page-heading{margin-bottom:24px}.eyebrow{margin:0;color:var(--accent);font-size:10px;text-transform:uppercase;letter-spacing:.14em;font-weight:800}h1{margin:5px 0 8px;font-size:42px}.page-heading p:last-child{margin:0;color:var(--muted)}button{padding:10px 14px;border:0;border-radius:9px;background:var(--accent);color:#07111f;font-weight:800}.summary-grid{display:grid;grid-template-columns:repeat(3,1fr);gap:12px;margin-bottom:15px}.summary-grid article,.panel{padding:18px;border:1px solid var(--border);border-radius:13px;background:var(--panel)}.summary-grid article{display:grid;gap:5px}.summary-grid span,.summary-grid small,.panel-heading>span{color:var(--muted)}.summary-grid strong{font-size:28px}.summary-grid .danger strong,.conflict{color:var(--red)}.suggestion,.warning,.state-panel{padding:12px 14px;border:1px solid var(--border);border-radius:10px;background:rgba(84,212,154,.07);color:var(--green)}.warning{margin:8px 0;background:rgba(241,199,91,.07);color:var(--yellow)}.state-panel--error{color:var(--red)}.panel{margin-top:15px}.panel-heading{align-items:center;margin-bottom:14px}.panel-heading h2{margin:4px 0 0}.port-map{display:grid;grid-template-columns:repeat(auto-fill,minmax(145px,1fr));gap:8px}.port-block{padding:12px;border:1px solid var(--border);border-radius:9px;background:#0c1118}.port-block strong{display:block;margin-bottom:4px}.port-block small{color:var(--muted)}.port-block.conflict{border-color:rgba(255,115,115,.5);background:rgba(255,115,115,.06)}.port-block--free{border-style:dashed;color:var(--green)}.port-table{border:1px solid var(--border);border-radius:9px;overflow:hidden}.port-row{display:grid;grid-template-columns:100px 1.3fr 1fr 110px 100px;gap:12px;padding:10px 12px;border-top:1px solid var(--border);align-items:center}.port-row:first-child{border-top:0}.port-row--header{background:#0c1118;color:var(--soft);font-size:10px;text-transform:uppercase}.port-row span{color:var(--muted)}.port-row em{font-style:normal;color:var(--soft)}.empty{padding:18px;color:var(--muted);text-align:center}@media(max-width:800px){.summary-grid{grid-template-columns:1fr}.page-heading{display:grid}.port-row{grid-template-columns:90px 1fr}.port-row--header{display:none}}
</style>
