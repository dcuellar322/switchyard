<script setup lang="ts">
import { projectTabs, type ProjectTab } from "../projectTabs";

defineProps<{ active: ProjectTab }>();
const emit = defineEmits<{ select: [tab: ProjectTab] }>();

function onKeydown(event: KeyboardEvent, index: number) {
  if (!["ArrowLeft", "ArrowRight", "Home", "End"].includes(event.key)) return;
  event.preventDefault();
  let next = index;
  if (event.key === "ArrowRight") next = (index + 1) % projectTabs.length;
  if (event.key === "ArrowLeft")
    next = (index - 1 + projectTabs.length) % projectTabs.length;
  if (event.key === "Home") next = 0;
  if (event.key === "End") next = projectTabs.length - 1;
  const tab = projectTabs[next];
  if (tab) emit("select", tab);
  requestAnimationFrame(() => document.getElementById(`tab-${tab}`)?.focus());
}
</script>

<template>
  <nav class="tabs" role="tablist" aria-label="Project sections">
    <button
      v-for="(tab, index) in projectTabs"
      :id="`tab-${tab}`"
      :key="tab"
      type="button"
      role="tab"
      :aria-selected="active === tab"
      :aria-controls="`panel-${tab}`"
      :tabindex="active === tab ? 0 : -1"
      @click="emit('select', tab)"
      @keydown="onKeydown($event, index)"
    >
      {{ tab }}
    </button>
  </nav>
</template>
