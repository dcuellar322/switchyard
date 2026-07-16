<script setup lang="ts">
import { useQuery } from "@tanstack/vue-query";
import { computed } from "vue";
import { RouterLink } from "vue-router";

import { formatBytes } from "../../../lib/format";
import { loadProjectSnapshots } from "../../dashboard/api";
import { loadProjects } from "../../projects/api";
import { useHostObservation } from "../../system/composables/useHostObservation";

const projects = useQuery({
  queryKey: ["projects"],
  queryFn: loadProjects,
  refetchInterval: 15_000,
});
const snapshots = useQuery({
  queryKey: computed(() => [
    "resource-snapshots",
    ...(projects.data.value ?? []).map((project) => project.id),
  ]),
  queryFn: () => loadProjectSnapshots(projects.data.value ?? []),
  enabled: computed(() => Boolean(projects.data.value)),
  refetchInterval: 10_000,
});
const host = useHostObservation();

const rows = computed(() =>
  (snapshots.data.value ?? [])
    .map((snapshot) => ({
      project: snapshot.project,
      driver: snapshot.runtime?.driver,
      state: snapshot.runtime?.state,
      cpu:
        snapshot.metrics?.reduce((sum, item) => sum + item.cpuPercent, 0) ?? 0,
      memory:
        snapshot.metrics?.reduce((sum, item) => sum + item.memoryBytes, 0) ?? 0,
      memoryLimit:
        snapshot.metrics?.reduce((sum, item) => sum + item.memoryLimit, 0) ?? 0,
      rx:
        snapshot.metrics?.reduce((sum, item) => sum + item.networkRxBytes, 0) ??
        0,
      tx:
        snapshot.metrics?.reduce((sum, item) => sum + item.networkTxBytes, 0) ??
        0,
      warnings: snapshot.warnings,
    }))
    .sort((left, right) => right.memory - left.memory),
);
const totals = computed(() =>
  rows.value.reduce(
    (value, row) => ({
      cpu: value.cpu + row.cpu,
      memory: value.memory + row.memory,
      rx: value.rx + row.rx,
      tx: value.tx + row.tx,
    }),
    { cpu: 0, memory: 0, rx: 0, tx: 0 },
  ),
);
const maxMemory = computed(() =>
  Math.max(...rows.value.map((row) => row.memory), 1),
);
</script>

<template>
  <section class="resources-view" aria-labelledby="resources-title">
    <header class="page-head">
      <div>
        <p class="eyebrow">Observed capacity</p>
        <h1 id="resources-title">Resources</h1>
        <span>Current runtime samples and explicitly shared host usage.</span>
      </div>
      <button
        type="button"
        :disabled="snapshots.isFetching.value"
        @click="snapshots.refetch()"
      >
        {{ snapshots.isFetching.value ? "Refreshing…" : "Refresh samples" }}
      </button>
    </header>

    <div class="summary-grid" :aria-busy="host.isPending.value">
      <article>
        <span>Host CPU</span
        ><strong>{{
          host.data.value ? `${host.data.value.cpuPercent.toFixed(1)}%` : "—"
        }}</strong
        ><small>Whole-machine observation</small>
      </article>
      <article>
        <span>Host memory</span
        ><strong>{{
          host.data.value ? formatBytes(host.data.value.memoryUsedBytes) : "—"
        }}</strong
        ><small>{{
          host.data.value
            ? `of ${formatBytes(host.data.value.memoryTotalBytes)}`
            : "Observation unavailable"
        }}</small>
      </article>
      <article>
        <span>Managed memory</span
        ><strong>{{ formatBytes(totals.memory) }}</strong
        ><small>Current service samples</small>
      </article>
      <article>
        <span>Shared Docker storage</span
        ><strong>{{
          host.data.value?.docker.connected
            ? formatBytes(host.data.value.docker.storageBytes ?? 0)
            : "Unavailable"
        }}</strong
        ><small>{{
          host.data.value?.docker.connected
            ? `${formatBytes(host.data.value.docker.reclaimableBytes ?? 0)} reclaimable · not project-exclusive`
            : "Other resource views remain usable"
        }}</small>
      </article>
    </div>

    <p v-if="host.data.value?.warnings.length" class="warning" role="status">
      Partial host observation: {{ host.data.value.warnings.join(" ") }}
    </p>
    <div
      v-if="projects.isPending.value || snapshots.isPending.value"
      class="loading"
      aria-live="polite"
    >
      <span></span><span></span><span></span>
    </div>
    <div v-else-if="projects.isError.value" class="state-panel" role="alert">
      <strong>Resource catalog unavailable</strong>
      <p>Reconnect to the daemon and retry.</p>
      <button type="button" @click="projects.refetch()">Retry</button>
    </div>
    <div v-else-if="!rows.length" class="state-panel">
      <strong>No project samples</strong>
      <p>
        Start a trusted project to collect CPU, memory, and network
        observations.
      </p>
      <RouterLink to="/projects">Open projects</RouterLink>
    </div>
    <article v-else class="panel">
      <header class="panel-head">
        <div>
          <p class="eyebrow">Managed runtimes</p>
          <h2>Project consumption</h2>
        </div>
        <span
          >{{ totals.cpu.toFixed(1) }}% sampled CPU · ↓
          {{ formatBytes(totals.rx) }} / ↑ {{ formatBytes(totals.tx) }}</span
        >
      </header>
      <div
        class="resource-table"
        role="table"
        aria-label="Project resource samples"
      >
        <div class="resource-row resource-row--head" role="row">
          <span>Project</span><span>Runtime</span><span>CPU</span
          ><span>Memory</span><span>Network</span>
        </div>
        <RouterLink
          v-for="row in rows"
          :key="row.project.id"
          class="resource-row"
          :to="{ name: 'project', params: { projectId: row.project.id } }"
          role="row"
        >
          <span
            ><strong>{{ row.project.displayName }}</strong
            ><small v-if="row.warnings.length">Partial observation</small></span
          ><span
            >{{ row.driver ?? "—" }} · {{ row.state ?? "unavailable" }}</span
          ><span>{{ row.cpu.toFixed(1) }}%</span
          ><span class="memory-cell"
            ><strong>{{ formatBytes(row.memory) }}</strong
            ><i
              ><b
                :style="{ width: `${(row.memory / maxMemory) * 100}%` }"
              ></b></i></span
          ><span
            >↓ {{ formatBytes(row.rx) }} · ↑ {{ formatBytes(row.tx) }}</span
          >
        </RouterLink>
      </div>
    </article>
    <p v-if="snapshots.isStale.value && snapshots.data.value" class="stale">
      Showing cached samples while the next observation is collected.
    </p>
  </section>
</template>

<style scoped>
.resources-view {
  width: min(100%, 1500px);
  margin: 0 auto;
  padding: 28px;
}
.page-head,
.panel-head {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 18px;
}
.page-head {
  margin-bottom: 22px;
}
.eyebrow {
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
.page-head span,
.panel-head > span {
  color: var(--muted);
}
button,
.state-panel a {
  padding: 9px 12px;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--panel-2);
  color: var(--text);
  text-decoration: none;
}
.summary-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 12px;
  margin-bottom: 16px;
}
.summary-grid article,
.panel {
  padding: 16px;
  border: 1px solid var(--border);
  border-radius: 13px;
  background: linear-gradient(145deg, var(--panel), #0e131a);
}
.summary-grid span,
.summary-grid small {
  display: block;
  color: var(--muted);
  font-size: 10px;
}
.summary-grid strong {
  display: block;
  margin: 7px 0 4px;
  font-size: 22px;
}
.warning,
.stale {
  padding: 10px 12px;
  border: 1px solid rgba(241, 199, 91, 0.25);
  border-radius: 8px;
  background: rgba(241, 199, 91, 0.06);
  color: var(--yellow);
}
.panel-head {
  align-items: center;
  margin-bottom: 14px;
}
.panel-head h2 {
  margin: 4px 0 0;
}
.resource-table {
  overflow: hidden;
  border: 1px solid var(--border);
  border-radius: 9px;
}
.resource-row {
  display: grid;
  grid-template-columns: 1.2fr 0.9fr 0.55fr 1fr 1fr;
  gap: 13px;
  align-items: center;
  padding: 11px 13px;
  border-top: 1px solid var(--border);
  color: var(--muted);
  text-decoration: none;
}
.resource-row:first-child {
  border-top: 0;
}
.resource-row:hover:not(.resource-row--head) {
  background: rgba(120, 166, 255, 0.04);
}
.resource-row--head {
  background: #0b1017;
  color: var(--soft);
  font-size: 9px;
  text-transform: uppercase;
}
.resource-row > span:first-child {
  display: grid;
  gap: 2px;
}
.resource-row strong {
  color: var(--text);
}
.resource-row small {
  color: var(--yellow);
  font-size: 9px;
}
.memory-cell {
  display: grid;
  gap: 5px;
}
.memory-cell i {
  height: 4px;
  border-radius: 3px;
  background: #232e3e;
}
.memory-cell b {
  display: block;
  height: 100%;
  border-radius: 3px;
  background: var(--accent);
}
.loading {
  display: grid;
  gap: 8px;
}
.loading span {
  height: 60px;
  border-radius: 9px;
  background: var(--panel);
  animation: pulse 1.2s infinite;
}
.state-panel {
  padding: 40px;
  border: 1px solid var(--border);
  border-radius: 13px;
  background: var(--panel);
  text-align: center;
}
.state-panel p {
  color: var(--muted);
}
@keyframes pulse {
  50% {
    opacity: 0.55;
  }
}
@media (max-width: 1000px) {
  .summary-grid {
    grid-template-columns: 1fr 1fr;
  }
  .resource-row {
    grid-template-columns: 1.2fr 0.8fr 0.7fr 1fr;
  }
  .resource-row > span:last-child {
    display: none;
  }
}
@media (max-width: 700px) {
  .resources-view {
    padding: 18px;
  }
  .page-head,
  .panel-head {
    display: grid;
  }
  .summary-grid {
    grid-template-columns: 1fr;
  }
  .resource-row {
    grid-template-columns: 1fr 1fr;
  }
  .resource-row--head {
    display: none;
  }
}
</style>
