<script setup lang="ts">
import type { RuntimeLogEntry } from "../../../api/generated/types.gen";

defineProps<{ entries: Array<RuntimeLogEntry>; connection: string }>();
</script>

<template>
  <article class="panel">
    <header class="panel-head">
      <div>
        <p>{{ entries.length }} bounded entries</p>
        <h2>Project logs</h2>
      </div>
      <span class="stream-state"
        ><i :class="{ online: connection === 'connected' }"></i
        >{{ connection }}</span
      >
    </header>
    <div v-if="entries.length" class="log-lines log-lines--full">
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
