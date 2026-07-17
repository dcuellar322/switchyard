<script setup lang="ts">
import { ArrowDownToLine } from "@lucide/vue";
import { nextTick, ref, watch } from "vue";

import type { RuntimeLogEntry } from "../../../api/generated/types.gen";

const props = defineProps<{ entries: Array<RuntimeLogEntry>; connection: string }>();
const logContainer = ref<HTMLElement>();
const followNewest = ref(true);

function updateFollowState() {
  const element = logContainer.value;
  if (!element) return;
  followNewest.value = element.scrollHeight - element.scrollTop - element.clientHeight < 48;
}

async function scrollToNewest() {
  followNewest.value = true;
  await nextTick();
  if (logContainer.value) logContainer.value.scrollTop = logContainer.value.scrollHeight;
}

watch(
  () => props.entries.at(-1)?.sequence,
  async () => {
    if (followNewest.value) await scrollToNewest();
  },
  { immediate: true },
);
</script>

<template>
  <article class="panel">
    <header class="panel-head">
      <div>
        <p>{{ entries.length }} bounded entries</p>
        <h2>Project logs</h2>
      </div>
      <div class="log-controls">
        <button type="button" :aria-pressed="followNewest" @click="scrollToNewest">
          <ArrowDownToLine :size="14" aria-hidden="true" />{{ followNewest ? "Following newest" : "Jump to newest" }}
        </button>
        <span class="stream-state"
          ><i :class="{ online: connection === 'connected' }"></i
          >{{ connection }}</span
        >
      </div>
    </header>
    <div v-if="entries.length" ref="logContainer" class="log-lines log-lines--full" @scroll.passive="updateFollowState">
      <div v-for="entry in entries" :key="entry.sequence">
        <time>{{ new Date(entry.timestamp).toLocaleTimeString() }}</time>
        <span>{{ entry.serviceId }}</span>
        <code :class="{ stderr: entry.stream === 'stderr' }">{{
          entry.message
        }}</code>
      </div>
    </div>
    <p v-else class="panel-state">No log entries match this project yet.</p>
  </article>
</template>
