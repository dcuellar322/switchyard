<script setup lang="ts">
import type { WorkspaceExecution } from '../../../api/generated/types.gen'

defineProps<{ execution: WorkspaceExecution; names: Record<string, string> }>()

function projectName(names: Record<string, string>, projectId: string): string {
  return names[projectId] ?? projectId
}

function duration(start?: string, finish?: string): string {
  if (!start) return 'Not started'
  if (!finish) return 'In progress'
  const milliseconds = Math.max(0, Date.parse(finish) - Date.parse(start))
  return milliseconds < 1_000 ? `${milliseconds} ms` : `${(milliseconds / 1_000).toFixed(1)} s`
}
</script>

<template>
  <section class="progress-panel" aria-labelledby="workspace-progress-title">
    <header>
      <div>
        <p>Latest execution</p>
        <h2 id="workspace-progress-title">{{ execution.kind }} · {{ execution.state.replaceAll('_', ' ') }}</h2>
      </div>
      <span :class="`summary summary--${execution.state}`">{{ execution.policy }} policy</span>
    </header>
    <p v-if="execution.errorMessage" class="execution-error" role="status">{{ execution.errorMessage }}</p>
    <ol>
      <li v-for="project in execution.projects" :key="project.projectId">
        <i :class="`marker marker--${project.status}`" aria-hidden="true"></i>
        <div>
          <strong>{{ projectName(names, project.projectId) }}</strong>
          <span>{{ project.message || project.status.replaceAll('_', ' ') }}</span>
        </div>
        <small>{{ duration(project.startedAt, project.finishedAt) }}</small>
      </li>
    </ol>
    <footer>
      <span>Started {{ new Date(execution.startedAt).toLocaleString() }}</span>
      <span v-if="execution.removeData" class="data-warning">Runtime data removal was explicitly requested</span>
      <span v-else>Project data preserved</span>
    </footer>
  </section>
</template>

<style scoped>
.progress-panel { border: 1px solid var(--border); border-radius: 15px; background: var(--panel); overflow: hidden; }
header { display: flex; align-items: center; justify-content: space-between; padding: 17px 19px; border-bottom: 1px solid var(--border); }
header p { margin: 0 0 3px; color: var(--accent); font-size: 10px; font-weight: 800; letter-spacing: .12em; text-transform: uppercase; }
h2 { margin: 0; font-size: 17px; text-transform: capitalize; }.summary { padding: 5px 8px; border: 1px solid var(--border); border-radius: 99px; color: var(--muted); font-size: 10px; }.summary--partially_succeeded, .summary--failed { color: var(--yellow); }
.execution-error { margin: 0; padding: 11px 19px; border-bottom: 1px solid rgba(255,115,115,.22); background: rgba(255,115,115,.08); color: #ffaaaa; font-size: 11px; line-height: 1.45; }
ol { display: grid; margin: 0; padding: 8px 19px; list-style: none; }
li { display: grid; grid-template-columns: 14px 1fr auto; align-items: center; gap: 10px; padding: 11px 0; border-bottom: 1px solid rgba(37,48,68,.7); }li:last-child { border: 0; }
.marker { width: 9px; height: 9px; border: 2px solid var(--soft); border-radius: 50%; }.marker--running, .marker--stopped, .marker--rolled_back { border-color: var(--green); background: var(--green); }.marker--starting, .marker--checking_health, .marker--stopping, .marker--rolling_back { border-color: var(--yellow); }.marker--start_failed, .marker--stop_failed, .marker--rollback_failed { border-color: var(--red); background: var(--red); }
li strong, li span { display: block; }li strong { font-size: 12px; }li span { margin-top: 3px; color: var(--muted); font-size: 10px; }li small { color: var(--soft); font-family: var(--mono); font-size: 10px; }
footer { display: flex; justify-content: space-between; gap: 12px; padding: 12px 19px; border-top: 1px solid var(--border); color: var(--soft); font-size: 10px; }.data-warning { color: var(--red); }
</style>
