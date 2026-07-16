<script setup lang="ts">
import { useMutation, useQuery } from '@tanstack/vue-query'
import { computed, ref } from 'vue'

import type { ActionDefinition, Project } from '../../../api/generated/types.gen'
import { loadProjectActions, loadProjectGit, runProjectAction } from '../api'

const props = defineProps<{ project: Project }>()
const projectId = computed(() => props.project.id)
const git = useQuery({ queryKey: ['project-git', projectId], queryFn: () => loadProjectGit(projectId.value), refetchInterval: 3_000 })
const actions = useQuery({ queryKey: ['project-actions', projectId], queryFn: () => loadProjectActions(projectId.value), staleTime: 30_000 })
const queued = ref('')
const actionRun = useMutation({
  mutationFn: ({ action, confirm }: { action: ActionDefinition; confirm: boolean }) => runProjectAction(projectId.value, action.id, confirm),
  onSuccess: (operation) => { queued.value = operation.id },
})

const changeCount = computed(() => {
  const changes = git.data.value?.changes
  return changes ? changes.staged + changes.modified + changes.untracked + changes.conflicted : 0
})

function run(action: ActionDefinition) {
  const confirm = action.risk !== 'destructive' || window.confirm(`Run destructive action “${action.name}”?`)
  if (confirm) actionRun.mutate({ action, confirm: action.risk === 'destructive' })
}
</script>

<template>
  <div class="developer-grid">
    <article class="developer-card">
      <div class="card-heading"><div><p class="eyebrow">Repository</p><h3>Git</h3></div><span>Live · 3s</span></div>
      <p v-if="git.isPending.value" class="empty">Reading Git porcelain…</p>
      <div v-else-if="git.isError.value" class="message message--error" role="alert">Git state is unavailable. <button type="button" @click="git.refetch()">Retry</button></div>
      <p v-else-if="!git.data.value?.repository" class="empty">No Git repository at the trusted root.</p>
      <template v-else>
        <div class="git-summary"><strong>{{ git.data.value.branch ?? 'Detached HEAD' }}</strong><span :class="{ dirty: changeCount }">{{ changeCount ? `${changeCount} changes` : 'clean' }}</span></div>
        <dl><div><dt>Ahead / behind</dt><dd>+{{ git.data.value.ahead }} / -{{ git.data.value.behind }}</dd></div><div><dt>Stashes</dt><dd>{{ git.data.value.stashes }}</dd></div><div><dt>Worktrees</dt><dd>{{ git.data.value.worktrees.length }}</dd></div><div><dt>Operation</dt><dd>{{ git.data.value.operationState ?? 'none' }}</dd></div></dl>
        <p v-if="git.data.value.lastCommit" class="last-commit"><code>{{ git.data.value.lastCommit.shortHash }}</code> {{ git.data.value.lastCommit.subject }}</p>
      </template>
    </article>

    <article class="developer-card">
      <div class="card-heading"><div><p class="eyebrow">Trusted manifest</p><h3>Quick actions</h3></div><span>{{ actions.data.value?.actions.length ?? 0 }}</span></div>
      <p v-if="actions.isPending.value" class="empty">Resolving safe actions…</p>
      <div v-else-if="actions.isError.value" class="message message--error" role="alert">Actions are unavailable. <button type="button" @click="actions.refetch()">Retry</button></div>
      <div v-else class="action-grid">
        <button v-for="action in actions.data.value?.actions ?? []" :key="action.id" type="button" :disabled="actionRun.isPending.value" @click="run(action)">
          <strong>{{ action.name }}</strong><span>{{ action.type }} · {{ action.risk }}</span>
        </button>
      </div>
      <p v-if="queued" class="queued" role="status">Queued operation <code>{{ queued }}</code>.</p>
      <p v-if="actionRun.isError.value" class="message message--error" role="alert">The action could not be queued.</p>
    </article>
  </div>
</template>

<style scoped>
.developer-grid{display:grid;grid-template-columns:1fr 1fr;gap:14px;margin-top:14px}.developer-card{min-width:0;padding:18px;border:1px solid var(--border);border-radius:12px;background:#0f141c}.card-heading{display:flex;align-items:center;justify-content:space-between;margin-bottom:13px}.card-heading h3{margin:3px 0 0}.card-heading>span{color:var(--soft);font-size:11px;text-transform:uppercase}.eyebrow{margin:0;color:var(--accent);font-size:10px;text-transform:uppercase;letter-spacing:.14em;font-weight:800}.git-summary{display:flex;justify-content:space-between;padding:12px;border-radius:9px;background:#0b1017}.git-summary span{color:var(--green)}.git-summary .dirty{color:var(--yellow)}dl{display:grid;grid-template-columns:1fr 1fr;gap:8px;margin:10px 0}dl div{padding:9px;border:1px solid var(--border);border-radius:8px}dt{color:var(--soft);font-size:10px}dd{margin:4px 0 0}.last-commit{margin:0;color:var(--muted);overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.action-grid{display:grid;grid-template-columns:1fr 1fr;gap:8px}.action-grid button{display:grid;gap:4px;padding:10px;text-align:left;border:1px solid var(--border);border-radius:9px;background:#0b1017;color:var(--text)}.action-grid span{color:var(--muted);font-size:10px}.queued{color:var(--green)}.empty{padding:15px;color:var(--muted);text-align:center}@media(max-width:900px){.developer-grid{grid-template-columns:1fr}}
</style>
