<script setup lang="ts">
import { nextTick, ref, watch } from "vue";

import type { RuntimeLogEntry } from "../../../api/generated/types.gen";

const props = defineProps<{
  recentLogs: Array<RuntimeLogEntry>;
  logConnection: string;
}>();
const emit = defineEmits<{
  viewAll: [];
}>();

const logPreview = ref<HTMLElement>();
watch(
  () => props.recentLogs.at(-1)?.sequence,
  async () => {
    await nextTick();
    if (logPreview.value) {
      logPreview.value.scrollTop = logPreview.value.scrollHeight;
    }
  },
  { immediate: true },
);
</script>

<template>
  <article class="panel logs-panel">
    <header class="panel-head">
      <div>
        <p>Streaming output</p>
        <h2>Live logs</h2>
      </div>
      <div class="stream-state">
        <i :class="{ online: logConnection === 'connected' }"></i>
        {{ logConnection }}
        <button type="button" @click="emit('viewAll')">View all →</button>
      </div>
    </header>
    <div
      v-if="recentLogs.length"
      ref="logPreview"
      class="log-lines"
      aria-label="Recent project logs"
    >
      <div v-for="entry in recentLogs.slice(-14)" :key="entry.sequence">
        <time>{{ new Date(entry.timestamp).toLocaleTimeString() }}</time>
        <span>{{ entry.serviceId }}</span>
        <code :class="{ stderr: entry.stream === 'stderr' }">{{
          entry.message
        }}</code>
      </div>
    </div>
    <p v-else class="panel-state">No persisted or live log entries yet.</p>
  </article>
</template>
