<script setup lang="ts">
import { Activity, Search } from "@lucide/vue";
import { computed } from "vue";

import { formatBytes } from "../../lib/format";
import { useOperationStore } from "../../domains/operations/store";
import { useHostObservation } from "../../domains/system/composables/useHostObservation";

defineEmits<{ palette: [] }>();
const host = useHostObservation();
const operations = useOperationStore();
const activeCount = computed(
  () =>
    operations.operations.value.filter(
      (item) => item.state === "queued" || item.state === "running",
    ).length,
);
</script>

<template>
  <header class="topbar">
    <button
      id="command-palette-trigger"
      class="command-search"
      type="button"
      aria-haspopup="dialog"
      @click="$emit('palette')"
    >
      <Search :size="16" aria-hidden="true" /><span>Projects, commands, ports…</span
      ><kbd>⌘ K</kbd>
    </button>
    <div class="host-metrics" :aria-busy="host.isPending.value">
      <span class="metric-pill"
        >CPU
        <strong>{{
          host.data.value ? `${host.data.value.cpuPercent.toFixed(0)}%` : "—"
        }}</strong></span
      >
      <span class="metric-pill"
        >Memory
        <strong>{{
          host.data.value
            ? `${formatBytes(host.data.value.memoryUsedBytes)} / ${formatBytes(host.data.value.memoryTotalBytes, 0)}`
            : "—"
        }}</strong></span
      >
      <span
        class="metric-pill"
        :title="
          host.data.value?.docker.attribution === 'shared'
            ? 'Aggregate shared Docker storage; not project-exclusive'
            : 'Docker storage unavailable'
        "
        >Docker
        <strong>{{
          host.data.value?.docker.connected
            ? formatBytes(host.data.value.docker.storageBytes)
            : "offline"
        }}</strong></span
      >
      <button
        class="operation-button"
        type="button"
        :aria-label="
          activeCount ? `${activeCount} active operations` : 'Open operations'
        "
        @click="operations.toggle()"
      >
        <Activity :size="18" aria-hidden="true" /><span v-if="activeCount">{{ activeCount }}</span>
      </button>
    </div>
  </header>
</template>

<style scoped>
.topbar {
  position: sticky;
  top: 0;
  z-index: 20;
  height: 72px;
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 0 28px;
  border-bottom: 1px solid var(--border);
  background: rgba(10, 13, 18, 0.74);
  backdrop-filter: blur(18px);
}
.command-search {
  display: flex;
  align-items: center;
  gap: 9px;
  width: min(480px, 46vw);
  height: 38px;
  padding: 0 12px;
  border: 1px solid var(--border);
  border-radius: 10px;
  background: var(--panel);
  color: var(--soft);
  text-align: left;
}
.command-search kbd {
  margin-left: auto;
  padding: 2px 6px;
  border: 1px solid #344157;
  border-radius: 5px;
  background: #19202b;
  color: var(--soft);
  font-size: 10px;
}
.host-metrics {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-left: auto;
}
.metric-pill {
  padding: 7px 10px;
  border: 1px solid var(--border);
  border-radius: 9px;
  background: var(--panel);
  color: var(--muted);
  font-size: 11px;
  white-space: nowrap;
}
.metric-pill strong {
  margin-left: 4px;
  color: var(--text);
}
.operation-button {
  position: relative;
  width: 38px;
  height: 38px;
  border: 1px solid var(--border);
  border-radius: 9px;
  background: var(--panel-2);
  color: var(--text);
}
.operation-button span {
  position: absolute;
  top: -5px;
  right: -5px;
  display: grid;
  place-items: center;
  min-width: 18px;
  height: 18px;
  padding: 0 4px;
  border-radius: 9px;
  background: var(--accent);
  color: #07111d;
  font-size: 10px;
  font-weight: 800;
}
@media (max-width: 1050px) {
  .metric-pill:nth-child(2),
  .metric-pill:nth-child(3) {
    display: none;
  }
}
@media (max-width: 760px) {
  .topbar {
    padding: 0 16px;
  }
  .command-search {
    width: min(100%, 480px);
  }
  .metric-pill {
    display: none;
  }
}
</style>
