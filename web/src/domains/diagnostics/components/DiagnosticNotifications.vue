<script setup lang="ts">
import type { DiagnosticNotification } from '../../../api/generated/types.gen'

defineProps<{
  notifications: Array<DiagnosticNotification>
  error: boolean
  pending: boolean
}>()
defineEmits<{ acknowledge: [id: string] }>()
</script>

<template>
  <section class="panel">
    <div class="section-heading">
      <div>
        <p class="eyebrow">Local alerts</p>
        <h2>Notifications</h2>
      </div>
      <span>{{ notifications.length }}</span>
    </div>
    <div v-if="error" class="error">Notifications unavailable.</div>
    <article v-for="item in notifications" :key="item.id" class="notification">
      <strong>{{ item.title }}</strong
      ><span>{{ item.occurrences }}× · {{ item.code }}</span>
      <p>{{ item.detail }}</p>
      <button class="quiet" :disabled="pending" @click="$emit('acknowledge', item.id)">
        Acknowledge
      </button>
    </article>
    <p v-if="notifications.length === 0" class="muted">
      No unreviewed crash, port, resource, or dependency alerts.
    </p>
  </section>
</template>

<style scoped>
.section-heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}
.section-heading h2 {
  margin: 3px 0 0;
  font-size: 18px;
}
.section-heading > span,
.muted {
  color: var(--muted);
  font-size: 12px;
}
.eyebrow {
  margin: 0;
  color: var(--accent);
  font-size: 10px;
  font-weight: 800;
  letter-spacing: 0.14em;
  text-transform: uppercase;
}
.error {
  padding: 11px 14px;
  border: 1px solid rgba(255, 117, 117, 0.35);
  border-radius: 9px;
  background: rgba(255, 117, 117, 0.08);
  color: #ff9b9b;
}
.notification {
  padding: 12px 0;
  border-bottom: 1px solid var(--border);
}
.notification > span {
  display: block;
  margin-top: 3px;
  color: var(--soft);
  font-size: 10px;
}
.notification p {
  margin: 8px 0;
  color: var(--muted);
  line-height: 1.45;
}
button {
  min-height: 36px;
  padding: 0 12px;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: transparent;
  color: var(--muted);
  cursor: pointer;
}
button:disabled {
  opacity: 0.45;
  cursor: not-allowed;
}
</style>
