<script setup lang="ts">
defineProps<{
  eyebrow: string;
  title: string;
  description: string;
  phase: string;
}>();
</script>

<template>
  <section class="feature-shell">
    <p>{{ eyebrow }}</p>
    <h1>{{ title }}</h1>
    <span>{{ description }}</span>
    <article>
      <strong>{{ phase }}</strong>
      <p>
        This route is part of the stable alpha navigation. Its specialized
        orchestration arrives in the named roadmap phase without mock controls
        or invented state.
      </p>
    </article>
  </section>
</template>

<style scoped>
.feature-shell {
  max-width: 900px;
  padding: 48px 28px;
  margin: 0 auto;
}
.feature-shell > p {
  margin: 0;
  color: var(--accent);
  font-size: 10px;
  text-transform: uppercase;
  letter-spacing: 0.14em;
  font-weight: 800;
}
.feature-shell h1 {
  margin: 7px 0;
  font-size: 38px;
}
.feature-shell > span {
  color: var(--muted);
  font-size: 16px;
}
.feature-shell article {
  margin-top: 28px;
  padding: 24px;
  border: 1px solid var(--border);
  border-radius: 14px;
  background: var(--panel);
}
.feature-shell article strong {
  color: var(--yellow);
}
.feature-shell article p {
  margin: 8px 0 0;
  color: var(--muted);
  line-height: 1.6;
}
</style>
