<script setup lang="ts">
import { Plus, ScanSearch } from '@lucide/vue'
import { RouterLink } from 'vue-router'

import { formatBytes } from '../../../lib/format'
import DashboardProjectCard from '../components/DashboardProjectCard.vue'
import { useDashboard } from '../composables/useDashboard'

withDefaults(defineProps<{ catalogOnly?: boolean }>(), { catalogOnly: false })
const {
  projects,
  projectList,
  snapshots,
  ports,
  host,
  search,
  statusFilter,
  tagFilter,
  sortOrder,
  operationError,
  pendingProject,
  tags,
  visibleSnapshots,
  runningCount,
  serviceCount,
  memoryBytes,
  repoAttention,
  partialCount,
  runLifecycle,
  clearFilters,
  markProjectAccess,
} = useDashboard()
</script>

<template>
  <section
    class="dashboard-view"
    :aria-labelledby="catalogOnly ? 'projects-title' : 'dashboard-title'"
  >
    <header class="page-head">
      <div>
        <p v-if="catalogOnly" class="eyebrow">Project catalog</p>
        <h1 :id="catalogOnly ? 'projects-title' : 'dashboard-title'">
          {{ catalogOnly ? 'Projects' : 'Your development yard' }}
        </h1>
        <p>
          {{
            catalogOnly
              ? 'Managed repositories, runtimes, and trusted actions.'
              : 'Your local development environment at a glance.'
          }}
        </p>
      </div>
      <div class="page-actions">
        <RouterLink class="button" :to="{ name: 'discovery' }"
          ><ScanSearch :size="16" aria-hidden="true" />Scan for projects</RouterLink
        >
        <RouterLink class="button button--primary" :to="{ name: 'discovery' }"
          ><Plus :size="17" aria-hidden="true" />Add project</RouterLink
        >
      </div>
    </header>

    <div v-if="!catalogOnly" class="summary-grid" aria-label="Environment summary">
      <article class="summary-card">
        <span>Running projects</span><strong>{{ runningCount }}</strong
        ><small>{{ serviceCount }} services currently running</small>
      </article>
      <article class="summary-card">
        <span>Memory in use</span><strong>{{ formatBytes(memoryBytes) }}</strong
        ><small>Across managed projects</small>
      </article>
      <article class="summary-card">
        <span>Port conflicts</span
        ><strong :class="{ conflict: ports.data.value?.conflicts.length }">{{
          ports.data.value?.conflicts.length ?? '—'
        }}</strong
        ><small>{{
          ports.data.value?.conflicts.length ? 'Action recommended' : 'No managed overlap detected'
        }}</small>
      </article>
      <article class="summary-card">
        <span>Repos needing attention</span><strong>{{ repoAttention }}</strong
        ><small>{{
          partialCount
            ? `${partialCount} projects have partial data`
            : repoAttention
              ? 'Review branch and working-tree state'
              : 'Git snapshots are current'
        }}</small>
      </article>
    </div>

    <div v-if="host.data.value?.warnings.length" class="partial-banner" role="status">
      Partial host data: {{ host.data.value.warnings.join(' ') }}
    </div>
    <div v-if="operationError" class="error-banner" role="alert">
      {{ operationError }}
    </div>

    <div class="toolbar">
      <div class="section-title">
        <h2>Projects</h2>
        <span>{{ projectList.length }} registered repositories</span>
      </div>
      <label class="search-filter"
        ><span class="sr-only">Search projects</span
        ><input v-model="search" placeholder="Filter projects…"
      /></label>
      <label
        ><span class="sr-only">Status filter</span
        ><select v-model="statusFilter">
          <option value="all">All status</option>
          <option value="running">Running</option>
          <option value="stopped">Stopped</option>
          <option value="degraded">Degraded</option>
          <option value="unknown">Unavailable</option>
        </select></label
      >
      <label
        ><span class="sr-only">Tag filter</span
        ><select v-model="tagFilter">
          <option value="all">All tags</option>
          <option v-for="tag in tags" :key="tag" :value="tag">{{ tag }}</option>
        </select></label
      >
      <label
        ><span class="sr-only">Sort projects</span
        ><select v-model="sortOrder">
          <option value="recent">Sort: Recent</option>
          <option value="name">Sort: Name</option>
          <option value="status">Sort: Status</option>
        </select></label
      >
    </div>

    <div v-if="projects.isPending.value" class="projects-grid" aria-live="polite">
      <div v-for="index in 4" :key="index" class="project-skeleton">
        <span></span><span></span><span></span>
      </div>
    </div>
    <div v-else-if="projects.isError.value" class="state-panel state-panel--error" role="alert">
      <strong>Project catalog unavailable</strong>
      <p>The daemon is connected, but the catalog query failed.</p>
      <button type="button" @click="projects.refetch()">Retry</button>
    </div>
    <div v-else-if="!projectList.length" class="empty-state">
      <div aria-hidden="true">◇</div>
      <h2>No projects registered</h2>
      <p>Scan a repository to review deterministic evidence before trusting it.</p>
      <RouterLink class="button button--primary" :to="{ name: 'discovery' }"
        >Scan your first project</RouterLink
      >
    </div>
    <div v-else-if="!visibleSnapshots.length" class="empty-state">
      <div aria-hidden="true">⌕</div>
      <h2>No projects match</h2>
      <p>Clear the search or broaden the status and tag filters.</p>
      <button class="button" type="button" @click="clearFilters">Clear filters</button>
    </div>
    <div v-else class="projects-grid" :aria-busy="snapshots.isFetching.value">
      <DashboardProjectCard
        v-for="snapshot in visibleSnapshots"
        :key="snapshot.project.id"
        :snapshot="snapshot"
        :pending="pendingProject === snapshot.project.id"
        @runtime="runLifecycle"
        @open="markProjectAccess"
      />
    </div>
    <p v-if="snapshots.isStale.value && snapshots.data.value" class="stale-note">
      Showing cached project observations while a refresh is pending.
    </p>
  </section>
</template>

<style scoped>
.dashboard-view {
  width: min(100%, 1600px);
  margin: 0 auto;
  padding: 28px;
}
.page-head {
  display: flex;
  justify-content: space-between;
  gap: 18px;
  align-items: flex-start;
  margin-bottom: 24px;
}
.page-head h1 {
  margin: 0 0 7px;
  font-size: 26px;
  letter-spacing: -0.035em;
}
.page-head p {
  margin: 0;
  color: var(--muted);
}
.eyebrow {
  margin: 0 0 6px !important;
  color: var(--accent) !important;
  font-size: 10px;
  text-transform: uppercase;
  letter-spacing: 0.13em;
  font-weight: 800;
}
.page-actions {
  display: flex;
  gap: 8px;
}
.button {
  display: inline-flex;
  min-height: 38px;
  align-items: center;
  justify-content: center;
  gap: 7px;
  padding: 8px 11px;
  border: 1px solid var(--border);
  border-radius: 9px;
  background: var(--panel-2);
  color: var(--text);
  text-decoration: none;
}
.button--primary {
  border: 0;
  background: linear-gradient(135deg, #79a8ff, #8b8aff);
  color: #07111d;
  font-weight: 750;
}
.summary-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 13px;
  margin-bottom: 24px;
}
.summary-card {
  position: relative;
  overflow: hidden;
  padding: 16px;
  border: 1px solid var(--border);
  border-radius: 14px;
  background: linear-gradient(145deg, rgba(20, 27, 37, 0.97), rgba(15, 20, 28, 0.97));
}
.summary-card::after {
  content: '';
  position: absolute;
  top: -38px;
  right: -22px;
  width: 70px;
  height: 70px;
  border-radius: 50%;
  background: rgba(120, 166, 255, 0.09);
}
.summary-card span,
.summary-card small {
  display: block;
  color: var(--muted);
  font-size: 11px;
}
.summary-card strong {
  display: block;
  margin-top: 8px;
  font-size: 24px;
  letter-spacing: -0.03em;
}
.summary-card small {
  margin-top: 5px;
  color: var(--soft);
  font-size: 10px;
}
.summary-card .conflict {
  color: var(--red);
}
.partial-banner,
.error-banner {
  margin-bottom: 14px;
  padding: 10px 12px;
  border: 1px solid rgba(241, 199, 91, 0.3);
  border-radius: 9px;
  background: rgba(241, 199, 91, 0.08);
  color: var(--yellow);
}
.error-banner {
  border-color: rgba(255, 115, 115, 0.3);
  background: rgba(255, 115, 115, 0.08);
  color: var(--red);
}
.toolbar {
  display: flex;
  align-items: center;
  gap: 9px;
  margin-bottom: 14px;
}
.section-title {
  margin-right: auto;
}
.section-title h2 {
  margin: 0 0 4px;
  font-size: 18px;
}
.section-title span {
  color: var(--muted);
}
.toolbar input,
.toolbar select {
  height: 38px;
  padding: 0 10px;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--panel);
  color: var(--muted);
}
.toolbar input {
  width: clamp(240px, 24vw, 360px);
}
.projects-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 14px;
}
.project-skeleton {
  height: 228px;
  padding: 20px;
  border: 1px solid var(--border);
  border-radius: 15px;
  background: var(--panel);
  animation: pulse 1.4s ease-in-out infinite;
}
.project-skeleton span {
  display: block;
  width: 40%;
  height: 12px;
  margin-bottom: 24px;
  border-radius: 6px;
  background: #202a38;
}
.project-skeleton span:nth-child(2) {
  width: 100%;
  height: 52px;
}
.project-skeleton span:nth-child(3) {
  width: 70%;
  height: 28px;
}
.state-panel,
.empty-state {
  padding: 42px 24px;
  border: 1px solid var(--border);
  border-radius: 14px;
  background: var(--panel);
  text-align: center;
  color: var(--muted);
}
.state-panel strong,
.empty-state h2 {
  color: var(--text);
}
.empty-state > div {
  font-size: 28px;
  color: var(--accent);
}
.empty-state .button {
  width: max-content;
  margin: 0 auto;
}
.stale-note {
  color: var(--yellow);
  font-size: 11px;
}
.sr-only {
  position: absolute;
  width: 1px;
  height: 1px;
  padding: 0;
  margin: -1px;
  overflow: hidden;
  clip: rect(0, 0, 0, 0);
  white-space: nowrap;
  border: 0;
}
@keyframes pulse {
  50% {
    opacity: 0.55;
  }
}
@media (max-width: 1050px) {
  .summary-grid {
    grid-template-columns: repeat(2, 1fr);
  }
  .projects-grid {
    grid-template-columns: 1fr;
  }
  .toolbar {
    flex-wrap: wrap;
  }
  .section-title {
    width: 100%;
  }
}
@media (max-width: 760px) {
  .dashboard-view {
    padding: 18px;
  }
  .page-head {
    display: grid;
  }
  .summary-grid {
    grid-template-columns: 1fr 1fr;
  }
  .toolbar label,
  .toolbar input,
  .toolbar select {
    width: 100%;
  }
}
@media (max-width: 480px) {
  .summary-grid {
    grid-template-columns: 1fr;
  }
  .page-actions {
    display: grid;
  }
}
</style>
