<script setup lang="ts">
import { computed, reactive, ref } from 'vue'

import type { WorkspaceDefinition, WorkspaceMemberRole, WorkspaceRecipe } from '../../../api/generated/types.gen'

type WorkspaceRecipeKind = WorkspaceRecipe['kind']

export interface WorkspaceMemberOption { id: string; name: string; location: string; environment: boolean }

const props = defineProps<{ members: Array<WorkspaceMemberOption>; saving: boolean }>()
const emit = defineEmits<{ save: [definition: WorkspaceDefinition]; cancel: [] }>()

const name = ref('')
const description = ref('')
const policy = ref<'rollback' | 'continue'>('rollback')
const selected = reactive(new Map<string, WorkspaceMemberRole>())
const dependencyProject = ref('')
const dependencyTarget = ref('')
const dependencies = ref<Array<{ projectId: string; dependsOnProjectId: string }>>([])
const recipes = ref<Array<{ id: string; name: string; kind: WorkspaceRecipeKind; projectId?: string; target?: string; arguments: Array<string>; order: number }>>([])
const recipeName = ref('')
const recipeKind = ref<WorkspaceRecipeKind>('open_terminal')
const recipeProject = ref('')
const recipeTarget = ref('')

const selectedIds = computed(() => [...selected.keys()])
const canSave = computed(() => name.value.trim() !== '' && selected.size > 0)
const memberName = (id: string): string => props.members.find((item) => item.id === id)?.name ?? id

function toggle(projectId: string, checked: boolean): void {
  if (checked) selected.set(projectId, 'application')
  else {
    selected.delete(projectId)
    dependencies.value = dependencies.value.filter((edge) => edge.projectId !== projectId && edge.dependsOnProjectId !== projectId)
  }
}

function addDependency(): void {
  if (!dependencyProject.value || !dependencyTarget.value || dependencyProject.value === dependencyTarget.value) return
  if (!dependencies.value.some((edge) => edge.projectId === dependencyProject.value && edge.dependsOnProjectId === dependencyTarget.value)) {
    dependencies.value.push({ projectId: dependencyProject.value, dependsOnProjectId: dependencyTarget.value })
  }
  dependencyProject.value = ''
  dependencyTarget.value = ''
}

function addRecipe(): void {
  if (!recipeName.value.trim()) return
  recipes.value.push({
    id: `recipe-${recipes.value.length + 1}`,
    name: recipeName.value.trim(),
    kind: recipeKind.value,
    projectId: recipeProject.value || undefined,
    target: recipeTarget.value.trim() || undefined,
    arguments: [],
    order: recipes.value.length,
  })
  recipeName.value = ''
  recipeTarget.value = ''
}

function submit(): void {
  if (!canSave.value) return
  const projectIds = selectedIds.value
  emit('save', {
    name: name.value.trim(),
    description: description.value.trim() || undefined,
    policy: policy.value,
    profile: 'full',
    members: projectIds.map((projectId, order) => ({ projectId, role: selected.get(projectId)!, order, healthGate: true, healthTimeoutSeconds: 120 })),
    dependencies: dependencies.value,
    recipes: recipes.value,
    profiles: [
      { id: 'full', name: 'Full workspace', projectIds, maxParallel: 4, lowMemory: false },
      { id: 'low-memory', name: 'Low memory', description: 'Dependency-safe sequential startup', projectIds, maxParallel: 1, lowMemory: true },
    ],
  })
}
</script>

<template>
  <form class="editor" @submit.prevent="submit">
    <header><div><p>Workspace builder</p><h2>Coordinate related projects</h2></div><button type="button" class="btn" @click="emit('cancel')">Cancel</button></header>
    <div class="editor-grid">
      <label>Name<input v-model="name" maxlength="160" required placeholder="Product development" /></label>
      <label>Failure behavior<select v-model="policy"><option value="rollback">Rollback started projects</option><option value="continue">Continue independent branches</option></select></label>
      <label class="wide">Description<textarea v-model="description" maxlength="2000" rows="2" placeholder="What this environment coordinates"></textarea></label>
    </div>
    <fieldset>
      <legend>Projects</legend><p>Select trusted projects and assign their role.</p>
      <div class="project-options">
        <label v-for="project in members" :key="project.id" class="project-option">
          <input type="checkbox" :checked="selected.has(project.id)" @change="toggle(project.id, ($event.target as HTMLInputElement).checked)" />
          <span><strong>{{ project.name }}<i v-if="project.environment">worktree</i></strong><small>{{ project.location }}</small></span>
          <select v-if="selected.has(project.id)" :value="selected.get(project.id)" aria-label="Workspace role" @change="selected.set(project.id, ($event.target as HTMLSelectElement).value as WorkspaceMemberRole)">
            <option value="application">Application</option><option value="dependency">Dependency</option><option value="tooling">Tooling</option>
          </select>
        </label>
      </div>
    </fieldset>
    <fieldset :disabled="selectedIds.length < 2">
      <legend>Dependencies</legend><p>A project starts only after its dependency passes the health gate.</p>
      <div class="inline-controls"><select v-model="dependencyProject"><option value="">Project…</option><option v-for="id in selectedIds" :key="id" :value="id">{{ memberName(id) }}</option></select><span>depends on</span><select v-model="dependencyTarget"><option value="">Dependency…</option><option v-for="id in selectedIds" :key="id" :value="id">{{ memberName(id) }}</option></select><button type="button" class="btn" @click="addDependency">Add edge</button></div>
      <ul><li v-for="(edge, index) in dependencies" :key="`${edge.projectId}-${edge.dependsOnProjectId}`">{{ memberName(edge.projectId) }} → {{ memberName(edge.dependsOnProjectId) }}<button type="button" @click="dependencies.splice(index, 1)">Remove</button></li></ul>
    </fieldset>
    <fieldset>
      <legend>Opening recipes</legend><p>Optional bounded shortcuts run only when explicitly requested.</p>
      <div class="inline-controls recipe-controls"><input v-model="recipeName" placeholder="Open web app" /><select v-model="recipeKind"><option value="open_url">URL</option><option value="open_terminal">Terminal</option><option value="open_editor">Editor</option><option value="start_agent">Agent</option></select><select v-model="recipeProject"><option value="">Workspace</option><option v-for="id in selectedIds" :key="id" :value="id">{{ memberName(id) }}</option></select><input v-model="recipeTarget" placeholder="URL or provider" /><button type="button" class="btn" @click="addRecipe">Add</button></div>
      <ul><li v-for="(recipe, index) in recipes" :key="recipe.id">{{ recipe.name }} · {{ recipe.kind.replace('_', ' ') }}<button type="button" @click="recipes.splice(index, 1)">Remove</button></li></ul>
    </fieldset>
    <footer><p>A full profile starts up to four independent projects in parallel. The included low-memory profile starts one at a time.</p><button class="btn primary" type="submit" :disabled="!canSave || saving">{{ saving ? 'Creating…' : 'Create workspace' }}</button></footer>
  </form>
</template>

<style scoped>
.editor { border: 1px solid var(--border); border-radius: 15px; background: var(--panel); overflow: hidden; }.editor > header, footer { display: flex; align-items: center; justify-content: space-between; gap: 16px; padding: 17px 19px; border-bottom: 1px solid var(--border); }.editor > header p { margin: 0 0 3px; color: var(--accent); font-size: 10px; font-weight: 800; letter-spacing: .12em; text-transform: uppercase; }h2 { margin: 0; font-size: 18px; }
.editor-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 13px; padding: 19px; }.wide { grid-column: 1 / -1; }label { display: grid; gap: 6px; color: var(--muted); font-size: 11px; }input, select, textarea { min-width: 0; padding: 9px 10px; border: 1px solid var(--border); border-radius: 8px; background: var(--panel-2); color: var(--text); font: inherit; }fieldset { margin: 0; padding: 17px 19px; border: 0; border-top: 1px solid var(--border); }legend { padding: 0; color: var(--text); font-size: 13px; font-weight: 700; }fieldset > p { margin: 4px 0 12px; color: var(--soft); font-size: 10px; }.project-options { display: grid; gap: 7px; }.project-option { grid-template-columns: auto 1fr minmax(110px, 150px); align-items: center; padding: 9px; border: 1px solid var(--border); border-radius: 9px; background: var(--panel-2); }.project-option span { display: grid; gap: 2px; }.project-option strong i { margin-left: 7px; padding: 2px 5px; border-radius: 99px; background: rgba(120,166,255,.12); color: var(--accent); font-size: 8px; font-style: normal; text-transform: uppercase; }.project-option small { color: var(--soft); overflow: hidden; text-overflow: ellipsis; }.inline-controls { display: grid; grid-template-columns: 1fr auto 1fr auto; align-items: center; gap: 8px; }.recipe-controls { grid-template-columns: 1.1fr .8fr .8fr 1fr auto; }ul { display: grid; gap: 5px; padding: 0; list-style: none; }li { display: flex; justify-content: space-between; padding: 7px 9px; border-radius: 7px; background: var(--panel-2); color: var(--muted); font-size: 10px; }li button { border: 0; background: none; color: var(--red); font-size: 10px; cursor: pointer; }footer { border-top: 1px solid var(--border); border-bottom: 0; }footer p { max-width: 620px; margin: 0; color: var(--soft); font-size: 10px; line-height: 1.5; }button:disabled { opacity: .5; }
@media (max-width: 760px) { .editor-grid { grid-template-columns: 1fr; }.wide { grid-column: auto; }.inline-controls, .recipe-controls { grid-template-columns: 1fr; }.inline-controls > span { display: none; }.project-option { grid-template-columns: auto 1fr; }.project-option select { grid-column: 2; }footer { align-items: flex-start; flex-direction: column; } }
</style>
