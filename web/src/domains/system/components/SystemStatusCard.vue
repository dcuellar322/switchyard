<script setup lang="ts">
defineProps<{
  label: string
  value: string
  detail: string
  tone?: 'ready' | 'muted'
}>()
</script>

<template>
  <article class="status-card">
    <p>{{ label }}</p>
    <strong :class="{ ready: tone === 'ready' }">{{ value }}</strong>
    <small>{{ detail }}</small>
  </article>
</template>

<style scoped>
.status-card {
  min-height: 148px;
  display: grid;
  align-content: center;
  gap: 9px;
  padding: 20px;
  border: 1px solid var(--border);
  border-radius: 14px;
  background: linear-gradient(145deg, rgba(20, 27, 37, 0.97), rgba(15, 20, 28, 0.97));
}

p,
small {
  margin: 0;
  color: var(--muted);
}

strong {
  font-size: 24px;
  letter-spacing: -0.03em;
}

.ready {
  color: var(--green);
}
</style>
