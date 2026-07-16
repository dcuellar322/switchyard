<script setup lang="ts">
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { computed, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import type { WorkspaceDefinition, WorkspaceFailurePolicy } from '../../../api/generated/types.gen'
import { loadAllEnvironments } from '../../environments/api'
import { loadProjects } from '../../projects/api'
import { trackOperation } from '../../operations/store'
import { loadWorkspace, loadWorkspaces, runWorkspace, saveWorkspace } from '../api'
import WorkspaceEditor from '../components/WorkspaceEditor.vue'
import WorkspaceGraph from '../components/WorkspaceGraph.vue'
import WorkspaceProgress from '../components/WorkspaceProgress.vue'

const route = useRoute()
const router = useRouter()
const queryClient = useQueryClient()
const creating = ref(false)
const selectedId = ref(typeof route.query.workspace === 'string' ? route.query.workspace : '')
const profileId = ref('')
const policy = ref<WorkspaceFailurePolicy>('rollback')

const workspaces = useQuery({ queryKey: ['workspaces'], queryFn: loadWorkspaces, refetchInterval: 5_000 })
const projects = useQuery({ queryKey: ['projects'], queryFn: loadProjects })
const environments = useQuery({
  queryKey: computed(() => ['environments', ...(projects.data.value?.map((project) => project.id) ?? [])]),
  queryFn: () => loadAllEnvironments(projects.data.value ?? []),
  enabled: computed(() => Boolean(projects.data.value)),
})
const selected = useQuery({
  queryKey: computed(() => ['workspace', selectedId.value]),
  queryFn: () => loadWorkspace(selectedId.value),
  enabled: computed(() => selectedId.value !== ''),
  refetchInterval: 2_000,
})

const createMutation = useMutation({
  mutationFn: saveWorkspace,
  onSuccess: async (workspace) => {
    creating.value = false
    await queryClient.invalidateQueries({ queryKey: ['workspaces'] })
    selectWorkspace(workspace.id)
  },
})
const operationMutation = useMutation({
  mutationFn: ({ action, runRecipes = false }: { action: 'start' | 'stop'; runRecipes?: boolean }) =>
    runWorkspace(selectedId.value, { action, profileId: profileId.value || undefined, policy: policy.value, runRecipes }),
  onSuccess: async (operation) => {
    trackOperation(operation)
    await queryClient.invalidateQueries({ queryKey: ['workspace', selectedId.value] })
  },
})

const current = computed(() => selected.data.value)
const running = computed(() => current.value?.lastRun?.state === 'running')
const workspaceCount = computed(() => workspaces.data.value?.length ?? 0)
const memberOptions = computed(() => [
  ...(projects.data.value ?? []).map((project) => ({ id: project.id, name: project.displayName, location: project.primaryLocation, environment: false })),
  ...(environments.data.value ?? []).map((environment) => ({ id: environment.id, name: environment.name, location: environment.path, environment: true })),
])
const memberNames = computed(() => Object.fromEntries(memberOptions.value.map((item) => [item.id, item.name])))

watch(() => workspaces.data.value, (items) => {
  if (!selectedId.value && items?.length) selectWorkspace(items[0]!.id)
}, { immediate: true })
watch(current, (workspace) => {
  if (!workspace) return
  profileId.value = workspace.profile ?? workspace.profiles[0]?.id ?? ''
  policy.value = workspace.policy
})

function selectWorkspace(id: string): void {
  selectedId.value = id
  void router.replace({ query: { ...route.query, workspace: id } })
}

function create(definition: WorkspaceDefinition): void {
  createMutation.mutate(definition)
}
</script>

<template>
  <section class="workspaces-view">
    <header class="page-head">
      <div><p class="eyebrow">Coordinated environments</p><h1>Workspaces</h1><span>Dependency-aware project startup, health gates, and parallel feature environments.</span></div>
      <button class="btn primary" type="button" @click="creating = !creating">{{ creating ? 'Close builder' : '＋ New workspace' }}</button>
    </header>

    <div v-if="workspaces.isError.value" class="state-panel" role="alert"><strong>Workspace registry unavailable</strong><span>{{ workspaces.error.value?.message }}</span><button class="btn" @click="workspaces.refetch()">Retry</button></div>
    <WorkspaceEditor v-else-if="creating" :members="memberOptions" :saving="createMutation.isPending.value" @save="create" @cancel="creating = false" />
    <div v-if="createMutation.isError.value" class="inline-error" role="alert">{{ createMutation.error.value?.message }}</div>

    <div v-if="workspaces.isPending.value" class="state-panel"><strong>Loading workspace graphs…</strong><span>Reading durable coordination state.</span></div>
    <div v-else-if="workspaceCount === 0 && !creating" class="empty-state">
      <span aria-hidden="true">▥</span><h2>Create your first workspace</h2><p>Group related projects, define dependency edges, and choose rollback or continue behavior before anything starts.</p><button class="btn primary" @click="creating = true">Build a workspace</button>
    </div>

    <div v-else-if="workspaceCount > 0" class="workspace-layout">
      <aside class="workspace-list" aria-label="Workspaces">
        <button v-for="workspace in workspaces.data.value" :key="workspace.id" type="button" :class="{ active: workspace.id === selectedId }" @click="selectWorkspace(workspace.id)">
          <span><strong>{{ workspace.name }}</strong><small>{{ workspace.members.length }} projects · {{ workspace.policy }}</small></span>
          <i :class="`run-state run-state--${workspace.lastRun?.state ?? 'idle'}`" :title="workspace.lastRun?.state ?? 'idle'"></i>
        </button>
      </aside>

      <main v-if="current" class="workspace-detail">
        <section class="workspace-hero">
          <div><p>{{ current.policy }} on failure · revision {{ current.revision }}</p><h2>{{ current.name }}</h2><span>{{ current.description || 'No workspace description.' }}</span></div>
          <div class="controls">
            <label>Profile<select v-model="profileId"><option v-for="profile in current.profiles" :key="profile.id" :value="profile.id">{{ profile.name }}{{ profile.lowMemory ? ' · low memory' : '' }}</option></select></label>
            <label>Failure policy<select v-model="policy"><option value="rollback">Rollback</option><option value="continue">Continue</option></select></label>
            <button class="btn primary" :disabled="running || operationMutation.isPending.value" @click="operationMutation.mutate({ action: 'start' })">▶ Start</button>
            <button class="btn danger" :disabled="running || operationMutation.isPending.value" @click="operationMutation.mutate({ action: 'stop' })">■ Stop all</button>
          </div>
        </section>
        <div v-if="operationMutation.isError.value" class="inline-error" role="alert">{{ operationMutation.error.value?.message }}</div>
        <div class="workspace-summary">
          <article><small>Projects</small><strong>{{ current.members.length }}</strong><span>{{ current.dependencies.length }} dependency edges</span></article>
          <article><small>Parallel limit</small><strong>{{ current.profiles.find((item) => item.id === profileId)?.maxParallel ?? 4 }}</strong><span>{{ current.profiles.find((item) => item.id === profileId)?.lowMemory ? 'Low-memory profile' : 'Independent branches' }}</span></article>
          <article><small>Opening recipes</small><strong>{{ current.recipes.length }}</strong><button v-if="current.recipes.length" type="button" :disabled="running" @click="operationMutation.mutate({ action: 'start', runRecipes: true })">Start and open →</button><span v-else>No automatic launches</span></article>
        </div>
        <WorkspaceGraph :workspace="current" :names="memberNames" />
        <WorkspaceProgress v-if="current.lastRun" :execution="current.lastRun" :names="memberNames" />
        <section v-if="current.recipes.length" class="recipe-panel"><header><div><p>After startup</p><h2>Opening recipes</h2></div><span>Explicit launch only</span></header><ul><li v-for="recipe in current.recipes" :key="recipe.id"><strong>{{ recipe.name }}</strong><span>{{ recipe.kind.replace('_', ' ') }}<template v-if="recipe.target"> · {{ recipe.target }}</template></span></li></ul></section>
      </main>
      <main v-else class="state-panel"><strong>Loading workspace…</strong><span>Refreshing its graph and latest operation.</span></main>
    </div>
  </section>
</template>

<style scoped>
.workspaces-view { max-width: 1500px; padding: 28px; margin: 0 auto; }.page-head { display: flex; align-items: flex-end; justify-content: space-between; gap: 24px; margin-bottom: 20px; }.eyebrow, .workspace-hero p, .recipe-panel header p { margin: 0 0 5px; color: var(--accent); font-size: 10px; font-weight: 800; letter-spacing: .13em; text-transform: uppercase; }.page-head h1 { margin: 0 0 5px; font-size: 32px; letter-spacing: -.035em; }.page-head span, .workspace-hero span { color: var(--muted); font-size: 12px; }.workspace-layout { display: grid; grid-template-columns: 238px minmax(0, 1fr); gap: 16px; }.workspace-list { align-self: start; display: grid; gap: 6px; position: sticky; top: 92px; }.workspace-list button { display: flex; align-items: center; gap: 10px; width: 100%; padding: 12px; border: 1px solid var(--border); border-radius: 11px; background: var(--panel); color: var(--text); text-align: left; cursor: pointer; }.workspace-list button.active { border-color: rgba(120,166,255,.5); background: rgba(120,166,255,.1); box-shadow: inset 2px 0 var(--accent); }.workspace-list span { display: grid; gap: 4px; min-width: 0; }.workspace-list small { color: var(--soft); }.run-state { width: 8px; height: 8px; margin-left: auto; border-radius: 50%; background: var(--soft); }.run-state--running { background: var(--yellow); box-shadow: 0 0 10px rgba(241,199,91,.5); }.run-state--succeeded { background: var(--green); }.run-state--failed, .run-state--partially_succeeded { background: var(--red); }.workspace-detail { display: grid; gap: 14px; min-width: 0; }.workspace-hero { display: flex; align-items: center; justify-content: space-between; gap: 20px; padding: 18px 20px; border: 1px solid var(--border); border-radius: 15px; background: linear-gradient(135deg, rgba(120,166,255,.1), var(--panel) 46%); }.workspace-hero h2 { margin: 0 0 5px; font-size: 24px; }.controls { display: flex; align-items: end; gap: 8px; }.controls label { display: grid; gap: 4px; color: var(--soft); font-size: 9px; text-transform: uppercase; letter-spacing: .08em; }.controls select { max-width: 155px; padding: 8px; border: 1px solid var(--border); border-radius: 8px; background: var(--panel-2); color: var(--text); }.workspace-summary { display: grid; grid-template-columns: repeat(3, 1fr); gap: 10px; }.workspace-summary article { display: grid; gap: 5px; padding: 14px 16px; border: 1px solid var(--border); border-radius: 12px; background: var(--panel); }.workspace-summary small { color: var(--soft); text-transform: uppercase; letter-spacing: .08em; }.workspace-summary strong { font-size: 20px; }.workspace-summary span, .workspace-summary button { color: var(--muted); font-size: 10px; }.workspace-summary button { width: fit-content; padding: 0; border: 0; background: none; color: var(--accent); cursor: pointer; }.recipe-panel { border: 1px solid var(--border); border-radius: 15px; background: var(--panel); overflow: hidden; }.recipe-panel header { display: flex; align-items: center; justify-content: space-between; padding: 15px 18px; border-bottom: 1px solid var(--border); }.recipe-panel h2 { margin: 0; font-size: 16px; }.recipe-panel header > span { color: var(--soft); font-size: 10px; }.recipe-panel ul { display: grid; grid-template-columns: repeat(auto-fit, minmax(190px, 1fr)); gap: 8px; margin: 0; padding: 14px 18px; list-style: none; }.recipe-panel li { display: grid; gap: 4px; padding: 10px; border: 1px solid var(--border); border-radius: 9px; background: var(--panel-2); }.recipe-panel li span { color: var(--soft); font-size: 10px; }.state-panel, .empty-state { display: grid; justify-items: start; gap: 8px; padding: 28px; border: 1px solid var(--border); border-radius: 15px; background: var(--panel); }.state-panel span, .empty-state p { color: var(--muted); }.empty-state { justify-items: center; padding: 60px 28px; text-align: center; }.empty-state > span { color: var(--accent); font-size: 30px; }.empty-state h2, .empty-state p { margin: 0; }.empty-state p { max-width: 560px; line-height: 1.6; }.inline-error { padding: 10px 13px; border: 1px solid rgba(255,115,115,.3); border-radius: 9px; background: rgba(255,115,115,.08); color: #ffaaaa; font-size: 11px; }
@media (max-width: 1100px) { .workspace-layout { grid-template-columns: 1fr; }.workspace-list { position: static; grid-template-columns: repeat(auto-fit, minmax(180px, 1fr)); }.workspace-hero { align-items: flex-start; flex-direction: column; }.controls { flex-wrap: wrap; }.workspace-summary { grid-template-columns: 1fr; } }@media (max-width: 700px) { .workspaces-view { padding: 20px 16px; }.page-head { align-items: flex-start; flex-direction: column; }.controls { display: grid; width: 100%; grid-template-columns: 1fr 1fr; }.controls select, .controls button { width: 100%; max-width: none; } }
</style>
