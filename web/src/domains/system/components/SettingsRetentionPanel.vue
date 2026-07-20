<script setup lang="ts">
import { computed } from 'vue'

import type { DaemonSettings } from '../../../api/generated/types.gen'

const settings = defineModel<DaemonSettings>({ required: true })
const logDays = computed({
  get: () => settings.value.retention.logAgeSeconds / 86_400,
  set: (value: number) => {
    settings.value.retention.logAgeSeconds = Math.round(value * 86_400)
  },
})
const logMiB = computed({
  get: () => settings.value.retention.logMaximumBytes / (1 << 20),
  set: (value: number) => {
    settings.value.retention.logMaximumBytes = Math.round(value * (1 << 20))
  },
})
const rawHours = computed({
  get: () => settings.value.retention.metricRawSeconds / 3_600,
  set: (value: number) => {
    settings.value.retention.metricRawSeconds = Math.round(value * 3_600)
  },
})
const minuteHours = computed({
  get: () => settings.value.retention.metricMinuteSeconds / 3_600,
  set: (value: number) => {
    settings.value.retention.metricMinuteSeconds = Math.round(value * 3_600)
  },
})
const quarterDays = computed({
  get: () => settings.value.retention.metricQuarterHourSeconds / 86_400,
  set: (value: number) => {
    settings.value.retention.metricQuarterHourSeconds = Math.round(value * 86_400)
  },
})
</script>

<template>
  <article class="settings-panel">
    <div class="settings-panel__head">
      <div>
        <p>Bounded local storage</p>
        <h2>Retention</h2>
      </div>
      <span>Restart</span>
    </div>
    <p class="settings-help">
      Changes apply together on the next daemon start so collectors never disagree about their
      active bounds.
    </p>
    <div class="settings-fields">
      <label
        >Log age <small>days</small
        ><input v-model.number="logDays" type="number" min="1" max="365" required
      /></label>
      <label
        >Log disk cap <small>MiB</small
        ><input v-model.number="logMiB" type="number" min="1" max="1048576" required
      /></label>
      <label
        >Exact metrics <small>hours</small
        ><input v-model.number="rawHours" type="number" min="1" max="8760" required
      /></label>
      <label
        >1-minute metrics <small>hours</small
        ><input v-model.number="minuteHours" type="number" min="1" max="8760" required
      /></label>
      <label
        >15-minute metrics <small>days</small
        ><input v-model.number="quarterDays" type="number" min="1" max="365" required
      /></label>
      <label
        >Maximum chart points<input
          v-model.number="settings.retention.maximumMetricHistoryPoints"
          type="number"
          min="100"
          max="10000"
          required
      /></label>
    </div>
  </article>
</template>
