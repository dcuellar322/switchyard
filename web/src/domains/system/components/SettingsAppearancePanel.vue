<script setup lang="ts">
import type { DaemonSettings } from '../../../api/generated/types.gen'

const settings = defineModel<DaemonSettings>({ required: true })
</script>

<template>
  <article class="settings-panel">
    <div class="settings-panel__head">
      <div>
        <p>Every local client</p>
        <h2>Appearance</h2>
      </div>
      <span>Live</span>
    </div>
    <div class="settings-fields settings-fields--single">
      <label
        >Density<select v-model="settings.appearance.density">
          <option value="comfortable">Comfortable</option>
          <option value="compact">Compact</option>
        </select></label
      >
      <label
        >Timestamp style<select v-model="settings.appearance.timeDisplay">
          <option value="relative">Relative where supported</option>
          <option value="absolute">Absolute local time</option>
        </select></label
      >
      <label
        >Theme<select v-model="settings.appearance.theme">
          <option value="dark">Switchyard dark</option>
          <option value="high-contrast">High contrast</option>
        </select></label
      >
    </div>
  </article>
</template>
