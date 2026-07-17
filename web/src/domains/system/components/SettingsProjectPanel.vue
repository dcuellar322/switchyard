<script setup lang="ts">
import { computed } from 'vue'

import type { DaemonSettings } from '../../../api/generated/types.gen'

const settings = defineModel<DaemonSettings>({ required: true })
const excluded = computed({
  get: () => settings.value.ports.excluded.join(', '),
  set: (value: string) => {
    settings.value.ports.excluded = [...new Set(value.split(',').map((item) => Number.parseInt(item.trim(), 10)).filter(Number.isInteger))].sort((a, b) => a - b)
  },
})

function addRoot() {
  settings.value.projectRoots.push('')
}

function removeRoot(index: number) {
  if (settings.value.projectRoots.length > 1) settings.value.projectRoots.splice(index, 1)
}
</script>

<template>
  <article class="settings-panel settings-panel--wide">
    <div class="settings-panel__head"><div><p>Repository safety</p><h2>Project roots</h2></div><span>Live</span></div>
    <p class="settings-help">Deterministic discovery stays inside these canonical directories. A one-time outside-root scan always requires explicit approval.</p>
    <div class="settings-list">
      <div v-for="(_root, index) in settings.projectRoots" :key="index" class="settings-row">
        <label :for="`project-root-${index}`">Approved root {{ index + 1 }}</label>
        <input :id="`project-root-${index}`" v-model.trim="settings.projectRoots[index]" required autocomplete="off" placeholder="/Users/you/dev" />
        <button type="button" :disabled="settings.projectRoots.length === 1" :aria-label="`Remove project root ${index + 1}`" @click="removeRoot(index)">Remove</button>
      </div>
    </div>
    <button class="settings-secondary" type="button" :disabled="settings.projectRoots.length >= 32" @click="addRoot">Add project root</button>
    <fieldset class="settings-fieldset">
      <legend>Preferred port range</legend>
      <label>Start<input v-model.number="settings.ports.rangeStart" type="number" min="1024" max="65535" required /></label>
      <label>End<input v-model.number="settings.ports.rangeEnd" type="number" min="1024" max="65535" required /></label>
      <label class="span-two">Excluded ports <small>Comma-separated, within the preferred range.</small><input v-model="excluded" inputmode="numeric" placeholder="15001, 15432" /></label>
    </fieldset>
  </article>
</template>
