<script setup lang="ts">
import { RefreshCw } from "@lucide/vue";
import { useQuery } from "@tanstack/vue-query";
import { computed, ref } from "vue";
import { RouterLink, useRoute } from "vue-router";

import { loadProjects } from "../../projects/api";
import { loadProjectLogBatches } from "../api";

const route = useRoute();
const autoRefresh = ref(true);
const refreshInterval = ref(5_000);
const projects = useQuery({
  queryKey: ["projects"],
  queryFn: loadProjects,
  refetchInterval: 15_000,
});
const logs = useQuery({
  queryKey: computed(() => [
    "fleet-logs",
    ...(projects.data.value ?? []).map((project) => project.id),
  ]),
  queryFn: () => loadProjectLogBatches(projects.data.value ?? []),
  enabled: computed(() => Boolean(projects.data.value)),
  refetchInterval: computed(() => autoRefresh.value ? refreshInterval.value : false),
});
const search = ref("");
const projectFilter = ref("all");
const levelFilter = ref(
  typeof route.query.level === "string" ? route.query.level : "all",
);
const streamFilter = ref("all");

const projectNames = computed(
  () =>
    new Map(
      (projects.data.value ?? []).map((project) => [
        project.id,
        project.displayName,
      ]),
    ),
);
const levels = computed(() =>
  [
    ...new Set(
      (logs.data.value?.entries ?? [])
        .map((entry) => entry.level)
        .filter(Boolean),
    ),
  ].sort(),
);
const visible = computed(() => {
  const query = search.value.trim().toLowerCase();
  return (logs.data.value?.entries ?? [])
    .filter((entry) => {
      const matchesSearch =
        !query ||
        `${entry.message} ${entry.serviceId} ${projectNames.value.get(entry.projectId) ?? ""}`
          .toLowerCase()
          .includes(query);
      return (
        matchesSearch &&
        (projectFilter.value === "all" ||
          entry.projectId === projectFilter.value) &&
        (levelFilter.value === "all" ||
          entry.level.toLowerCase() === levelFilter.value.toLowerCase()) &&
        (streamFilter.value === "all" || entry.stream === streamFilter.value)
      );
    })
    .slice(0, 500);
});
</script>

<template>
  <section class="logs-view" aria-labelledby="logs-title">
    <header class="page-head">
      <div>
        <p>Fleet output</p>
        <h1 id="logs-title">Logs</h1>
        <span>Bounded persisted output across trusted projects.</span>
      </div>
      <div class="refresh-controls">
        <label class="auto-refresh">
          <input v-model="autoRefresh" type="checkbox" />
          <span>{{ autoRefresh ? "Auto refresh" : "Auto refresh off" }}</span>
        </label>
        <label v-if="autoRefresh">
          <span class="sr-only">Refresh interval</span>
          <select v-model.number="refreshInterval" aria-label="Refresh interval">
            <option :value="5_000">Every 5 seconds</option>
            <option :value="10_000">Every 10 seconds</option>
            <option :value="30_000">Every 30 seconds</option>
            <option :value="60_000">Every minute</option>
          </select>
        </label>
        <button
          type="button"
          :disabled="logs.isFetching.value"
          @click="logs.refetch()"
        >
          <RefreshCw :size="15" :class="{ spinning: logs.isFetching.value }" aria-hidden="true" />
          {{ logs.isFetching.value ? "Refreshing…" : "Refresh now" }}
        </button>
      </div>
    </header>
    <div class="filters" role="search">
      <label
        ><span class="sr-only">Search logs</span
        ><input
          v-model="search"
          placeholder="Search messages and services…" /></label
      ><label
        ><span class="sr-only">Project filter</span
        ><select v-model="projectFilter">
          <option value="all">All projects</option>
          <option
            v-for="project in projects.data.value ?? []"
            :key="project.id"
            :value="project.id"
          >
            {{ project.displayName }}
          </option>
        </select></label
      ><label
        ><span class="sr-only">Level filter</span
        ><select v-model="levelFilter">
          <option value="all">All levels</option>
          <option v-for="level in levels" :key="level" :value="level">
            {{ level }}
          </option>
          <option v-if="!levels.includes('error')" value="error">error</option>
        </select></label
      ><label
        ><span class="sr-only">Stream filter</span
        ><select v-model="streamFilter">
          <option value="all">All streams</option>
          <option value="stdout">stdout</option>
          <option value="stderr">stderr</option>
        </select></label
      ><span
        >{{ visible.length }} of
        {{ logs.data.value?.entries.length ?? 0 }} entries</span
      >
    </div>
    <p v-if="logs.data.value?.warnings.length" class="warning" role="status">
      Partial history: {{ logs.data.value.warnings.join(" · ") }}
    </p>
    <div
      v-if="projects.isPending.value || logs.isPending.value"
      class="state-panel"
      aria-live="polite"
    >
      Loading bounded project histories…
    </div>
    <div
      v-else-if="projects.isError.value || logs.isError.value"
      class="state-panel state-panel--error"
      role="alert"
    >
      <strong>Logs unavailable</strong>
      <p>The catalog or persisted log query failed.</p>
      <button type="button" @click="logs.refetch()">Retry</button>
    </div>
    <div v-else-if="!projects.data.value?.length" class="state-panel">
      <strong>No projects registered</strong>
      <p>Register a project before collecting runtime output.</p>
      <RouterLink to="/discovery">Scan a repository</RouterLink>
    </div>
    <div v-else-if="!visible.length" class="state-panel">
      <strong>No matching entries</strong>
      <p>Runtime output may be empty, or the current filters exclude it.</p>
      <button
        type="button"
        @click="
          search = '';
          projectFilter = 'all';
          levelFilter = 'all';
          streamFilter = 'all';
        "
      >
        Clear filters
      </button>
    </div>
    <div
      v-else
      class="log-console"
      role="log"
      aria-label="Fleet log entries"
      aria-live="off"
    >
      <div
        v-for="entry in visible"
        :key="`${entry.projectId}-${entry.sequence}`"
        class="log-row"
      >
        <time :datetime="entry.timestamp">{{
          new Date(entry.timestamp).toLocaleTimeString()
        }}</time
        ><RouterLink
          :to="{
            name: 'project',
            params: { projectId: entry.projectId },
            query: { tab: 'logs' },
          }"
          >{{
            projectNames.get(entry.projectId) ?? entry.projectId
          }}</RouterLink
        ><span>{{ entry.serviceId }}</span
        ><em :class="`level level--${entry.level.toLowerCase()}`">{{
          entry.level
        }}</em
        ><code :class="{ stderr: entry.stream === 'stderr' }">{{
          entry.message
        }}</code>
      </div>
    </div>
    <p
      v-if="logs.data.value && logs.data.value.entries.length >= 500"
      class="limit-note"
    >
      Showing the newest 500 entries to keep the interface responsive.
    </p>
  </section>
</template>

<style scoped>
.logs-view {
  width: min(100%, 1600px);
  margin: 0 auto;
  padding: 28px;
}
.page-head {
  display: flex;
  justify-content: space-between;
  gap: 18px;
  align-items: flex-start;
  margin-bottom: 20px;
}
.page-head p {
  margin: 0;
  color: var(--accent);
  font-size: 10px;
  font-weight: 800;
  letter-spacing: 0.13em;
  text-transform: uppercase;
}
.page-head h1 {
  margin: 5px 0;
  font-size: 27px;
}
.page-head span {
  color: var(--muted);
}
button,
.state-panel a {
  display: inline-flex;
  min-height: 38px;
  align-items: center;
  gap: 7px;
  padding: 9px 12px;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--panel-2);
  color: var(--text);
  text-decoration: none;
}
.refresh-controls {
  display: flex;
  align-items: center;
  gap: 8px;
}
.refresh-controls select {
  min-height: 38px;
  border: 1px solid var(--border);
  border-radius: 8px;
  background-color: var(--panel);
  color: var(--muted);
}
.auto-refresh {
  display: inline-flex;
  min-height: 38px;
  align-items: center;
  gap: 7px;
  padding: 0 10px;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--panel);
  color: var(--muted);
  white-space: nowrap;
}
.auto-refresh input { accent-color: var(--accent); }
.spinning { animation: refresh-spin 0.8s linear infinite; }
.filters {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 12px;
}
.filters label:first-child {
  flex: 1;
}
.filters input,
.filters select {
  width: 100%;
  height: 36px;
  padding: 0 10px;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--panel);
  color: var(--muted);
}
.filters > span {
  color: var(--soft);
  font-size: 10px;
  white-space: nowrap;
}
.warning,
.limit-note {
  padding: 9px 11px;
  border: 1px solid rgba(241, 199, 91, 0.24);
  border-radius: 8px;
  background: rgba(241, 199, 91, 0.06);
  color: var(--yellow);
}
.log-console {
  height: min(720px, calc(100vh - 230px));
  overflow: auto;
  border: 1px solid var(--border);
  border-radius: 10px;
  background: #070a0e;
  font:
    10px/1.5 ui-monospace,
    SFMono-Regular,
    Menlo,
    monospace;
}
.log-row {
  display: grid;
  grid-template-columns: 78px 115px 90px 54px minmax(0, 1fr);
  gap: 9px;
  padding: 4px 9px;
  border-top: 1px solid rgba(255, 255, 255, 0.025);
  align-items: start;
}
.log-row:first-child {
  border-top: 0;
}
.log-row time {
  color: var(--soft);
}
.log-row a {
  overflow: hidden;
  text-overflow: ellipsis;
  color: var(--accent);
  text-decoration: none;
}
.log-row > span {
  overflow: hidden;
  text-overflow: ellipsis;
  color: var(--muted);
}
.log-row em {
  font-style: normal;
  color: var(--soft);
}
.log-row .level--error,
.log-row .level--fatal {
  color: var(--red);
}
.log-row .level--warn,
.log-row .level--warning {
  color: var(--yellow);
}
.log-row code {
  overflow-wrap: anywhere;
  color: #b9c6d8;
}
.log-row code.stderr {
  color: #dfc17a;
}
.state-panel {
  padding: 42px;
  border: 1px solid var(--border);
  border-radius: 12px;
  background: var(--panel);
  text-align: center;
}
.state-panel p {
  color: var(--muted);
}
.state-panel--error {
  color: var(--red);
}
.sr-only {
  position: absolute;
  width: 1px;
  height: 1px;
  overflow: hidden;
  clip: rect(0, 0, 0, 0);
}
@media (max-width: 900px) {
  .filters {
    flex-wrap: wrap;
  }
  .filters label:first-child {
    flex-basis: 100%;
  }
  .log-row {
    grid-template-columns: 72px 1fr 70px;
  }
  .log-row > span,
  .log-row em {
    display: none;
  }
}
@media (max-width: 650px) {
  .logs-view {
    padding: 18px;
  }
  .page-head {
    display: grid;
  }
  .refresh-controls { flex-wrap: wrap; }
  .filters label {
    width: 100%;
  }
  .log-row {
    grid-template-columns: 65px 85px minmax(0, 1fr);
  }
}
@keyframes refresh-spin { to { transform: rotate(360deg); } }
</style>
