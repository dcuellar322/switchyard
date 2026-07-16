<script setup lang="ts">
import { useMutation, useQuery } from "@tanstack/vue-query";
import { computed, ref } from "vue";
import { RouterLink } from "vue-router";

import type { RuntimeAction } from "../../../api/generated/types.gen";
import { formatBytes, isActiveState } from "../../../lib/format";
import { loadPortRegistry } from "../../ports/api";
import {
  loadProjects,
  runProjectAction,
  runRuntimeAction,
} from "../../projects/api";
import { loadRecentProjects, markProjectAccess } from "../../projects/recent";
import { trackOperation } from "../../operations/store";
import { useHostObservation } from "../../system/composables/useHostObservation";
import { loadProjectSnapshots, type ProjectSnapshot } from "../api";
import DashboardProjectCard from "../components/DashboardProjectCard.vue";

withDefaults(defineProps<{ catalogOnly?: boolean }>(), { catalogOnly: false });
const projects = useQuery({
  queryKey: ["projects"],
  queryFn: loadProjects,
  refetchInterval: 15_000,
});
const projectList = computed(() => projects.data.value ?? []);
const snapshots = useQuery({
  queryKey: computed(() => [
    "dashboard-snapshots",
    ...projectList.value.map((project) => project.id),
  ]),
  queryFn: () => loadProjectSnapshots(projectList.value),
  enabled: computed(() => projectList.value.length > 0),
  refetchInterval: 15_000,
});
const ports = useQuery({
  queryKey: ["ports"],
  queryFn: loadPortRegistry,
  refetchInterval: 10_000,
});
const host = useHostObservation();
const search = ref("");
const statusFilter = ref("all");
const tagFilter = ref("all");
const sortOrder = ref("recent");
const operationError = ref("");
const pendingProject = ref("");

const runtimeMutation = useMutation({
  mutationFn: ({
    projectId,
    action,
  }: {
    projectId: string;
    action: RuntimeAction;
  }) => runRuntimeAction(projectId, action),
  onSuccess: (operation) => {
    trackOperation(operation);
  },
});
const terminalMutation = useMutation({
  mutationFn: (projectId: string) => runProjectAction(projectId, "terminal"),
  onSuccess: (operation) => {
    trackOperation(operation);
  },
});

const allSnapshots = computed<Array<ProjectSnapshot>>(
  () =>
    snapshots.data.value ??
    projectList.value.map((project) => ({
      project,
      warnings: ["Loading project observations"],
    })),
);
const tags = computed(() =>
  [...new Set(projectList.value.flatMap((project) => project.tags))].sort(),
);
const recent = computed(loadRecentProjects);
const visibleSnapshots = computed(() => {
  const query = search.value.trim().toLowerCase();
  const result = allSnapshots.value.filter((snapshot) => {
    const state = snapshot.runtime?.state ?? "unknown";
    const matchesSearch =
      !query ||
      `${snapshot.project.displayName} ${snapshot.project.slug} ${snapshot.project.tags.join(" ")}`
        .toLowerCase()
        .includes(query);
    const matchesStatus =
      statusFilter.value === "all" ||
      (statusFilter.value === "running"
        ? isActiveState(state)
        : state === statusFilter.value);
    const matchesTag =
      tagFilter.value === "all" ||
      snapshot.project.tags.includes(tagFilter.value);
    return matchesSearch && matchesStatus && matchesTag;
  });
  return result.sort((left, right) => {
    if (sortOrder.value === "name")
      return left.project.displayName.localeCompare(right.project.displayName);
    if (sortOrder.value === "status")
      return (left.runtime?.state ?? "unknown").localeCompare(
        right.runtime?.state ?? "unknown",
      );
    return (
      Date.parse(recent.value[right.project.id] ?? right.project.updatedAt) -
      Date.parse(recent.value[left.project.id] ?? left.project.updatedAt)
    );
  });
});
const runningCount = computed(
  () =>
    allSnapshots.value.filter((snapshot) =>
      isActiveState(snapshot.runtime?.state),
    ).length,
);
const serviceCount = computed(() =>
  allSnapshots.value.reduce(
    (total, snapshot) =>
      total +
      (snapshot.runtime?.services.filter(
        (service) => service.state === "running",
      ).length ?? 0),
    0,
  ),
);
const memoryBytes = computed(() =>
  allSnapshots.value.reduce(
    (total, snapshot) =>
      total +
      (snapshot.metrics?.reduce(
        (subtotal, metric) => subtotal + metric.memoryBytes,
        0,
      ) ?? 0),
    0,
  ),
);
const repoAttention = computed(
  () =>
    allSnapshots.value.filter((snapshot) => {
      const git = snapshot.git;
      return (
        git &&
        (git.behind > 0 ||
          git.changes.staged +
            git.changes.modified +
            git.changes.untracked +
            git.changes.conflicted >
            0)
      );
    }).length,
);
const partialCount = computed(
  () =>
    allSnapshots.value.filter((snapshot) => snapshot.warnings.length > 0)
      .length,
);

async function runLifecycle(snapshot: ProjectSnapshot, action: RuntimeAction) {
  pendingProject.value = snapshot.project.id;
  operationError.value = "";
  try {
    await runtimeMutation.mutateAsync({
      projectId: snapshot.project.id,
      action,
    });
  } catch (cause) {
    operationError.value =
      cause instanceof Error
        ? cause.message
        : "The lifecycle operation could not be queued.";
  } finally {
    pendingProject.value = "";
  }
}

async function openTerminal(snapshot: ProjectSnapshot) {
  pendingProject.value = snapshot.project.id;
  operationError.value = "";
  try {
    await terminalMutation.mutateAsync(snapshot.project.id);
  } catch (cause) {
    operationError.value =
      cause instanceof Error
        ? cause.message
        : "The terminal action could not be queued.";
  } finally {
    pendingProject.value = "";
  }
}
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
          {{ catalogOnly ? "Projects" : "Your development yard" }}
        </h1>
        <p>
          {{
            catalogOnly
              ? "Managed repositories, runtimes, and trusted actions."
              : "Your local development environment at a glance."
          }}
        </p>
      </div>
      <div class="page-actions">
        <RouterLink class="button" :to="{ name: 'discovery' }"
          >Scan for projects</RouterLink
        >
        <RouterLink class="button button--primary" :to="{ name: 'discovery' }"
          >＋ Add project</RouterLink
        >
      </div>
    </header>

    <div
      v-if="!catalogOnly"
      class="summary-grid"
      aria-label="Environment summary"
    >
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
          ports.data.value?.conflicts.length ?? "—"
        }}</strong
        ><small>{{
          ports.data.value?.conflicts.length
            ? "Action recommended"
            : "No managed overlap detected"
        }}</small>
      </article>
      <article class="summary-card">
        <span>Repos needing attention</span><strong>{{ repoAttention }}</strong
        ><small>{{
          partialCount
            ? `${partialCount} projects have partial data`
            : repoAttention
              ? "Review branch and working-tree state"
              : "Git snapshots are current"
        }}</small>
      </article>
    </div>

    <div
      v-if="host.data.value?.warnings.length"
      class="partial-banner"
      role="status"
    >
      Partial host data: {{ host.data.value.warnings.join(" ") }}
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

    <div
      v-if="projects.isPending.value"
      class="projects-grid"
      aria-live="polite"
    >
      <div v-for="index in 4" :key="index" class="project-skeleton">
        <span></span><span></span><span></span>
      </div>
    </div>
    <div
      v-else-if="projects.isError.value"
      class="state-panel state-panel--error"
      role="alert"
    >
      <strong>Project catalog unavailable</strong>
      <p>The daemon is connected, but the catalog query failed.</p>
      <button type="button" @click="projects.refetch()">Retry</button>
    </div>
    <div v-else-if="!projectList.length" class="empty-state">
      <div aria-hidden="true">◇</div>
      <h2>No projects registered</h2>
      <p>
        Scan a repository to review deterministic evidence before trusting it.
      </p>
      <RouterLink class="button button--primary" :to="{ name: 'discovery' }"
        >Scan your first project</RouterLink
      >
    </div>
    <div v-else-if="!visibleSnapshots.length" class="empty-state">
      <div aria-hidden="true">⌕</div>
      <h2>No projects match</h2>
      <p>Clear the search or broaden the status and tag filters.</p>
      <button
        class="button"
        type="button"
        @click="
          search = '';
          statusFilter = 'all';
          tagFilter = 'all';
        "
      >
        Clear filters
      </button>
    </div>
    <div v-else class="projects-grid" :aria-busy="snapshots.isFetching.value">
      <DashboardProjectCard
        v-for="snapshot in visibleSnapshots"
        :key="snapshot.project.id"
        :snapshot="snapshot"
        :pending="pendingProject === snapshot.project.id"
        @runtime="runLifecycle"
        @terminal="openTerminal"
        @open="markProjectAccess"
      />
    </div>
    <p
      v-if="snapshots.isStale.value && snapshots.data.value"
      class="stale-note"
    >
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
  align-items: center;
  justify-content: center;
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
  background: linear-gradient(
    145deg,
    rgba(20, 27, 37, 0.97),
    rgba(15, 20, 28, 0.97)
  );
}
.summary-card::after {
  content: "";
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
  height: 34px;
  padding: 0 10px;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--panel);
  color: var(--muted);
}
.toolbar input {
  width: 180px;
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
