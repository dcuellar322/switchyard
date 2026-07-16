<script setup lang="ts">
import { RouterLink } from "vue-router";

defineProps<{ projectId: string }>();
</script>

<template>
  <article class="panel future-panel">
    <strong>Project storage intelligence</strong>
    <p>
      Inspect containers, images, volumes, and build cache attributed to this
      project. Shared, estimated, and unknown values stay labeled, and cleanup
      remains a non-executable preview.
    </p>
    <RouterLink :to="{ name: 'resources', query: { project: projectId } }"
      >Inspect project storage →</RouterLink
    >
  </article>
</template>
