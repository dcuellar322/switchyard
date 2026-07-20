<script setup lang="ts">
import { ArrowLeft, ExternalLink, Play, RefreshCw, Square, Terminal } from '@lucide/vue'
import { ref, watch } from 'vue'
import { RouterLink } from 'vue-router'

import type { ActionDefinition, Project, RuntimeAction } from '../../../api/generated/types.gen'
import { projectInitials, stateLabel } from '../../../lib/format'

const props = defineProps<{
  project: Project
  state: string
  stateTone: string
  active: boolean
  browserAction?: ActionDefinition
  actionPending: boolean
  lifecyclePending: boolean
  operationError: string
  partial: boolean
  dockerUnavailable: boolean
  availableProfiles: string[]
}>()
const emit = defineEmits<{
  action: [action: ActionDefinition | undefined]
  lifecycle: [action: RuntimeAction, profiles: string[]]
  terminal: []
}>()
const showStartOptions = ref(false)
const selectedProfiles = ref<string[]>([])

watch(
  () => props.active,
  (active) => {
    if (active) closeStartOptions()
  },
)

function requestLifecycle() {
  if (props.active) {
    emit('lifecycle', 'stop', [])
    return
  }
  if (props.availableProfiles.length === 0) {
    emit('lifecycle', 'start', [])
    return
  }
  showStartOptions.value = true
}

function startProject() {
  emit('lifecycle', 'start', [...selectedProfiles.value])
  closeStartOptions()
}

function closeStartOptions() {
  showStartOptions.value = false
  selectedProfiles.value = []
}

function profileLabel(profile: string) {
  return profile.replaceAll(/[-_.]+/g, ' ').replace(/\b\w/g, (letter) => letter.toUpperCase())
}
</script>

<template>
  <header class="project-hero">
    <div class="hero-identity">
      <RouterLink class="back" to="/projects" aria-label="Back to projects"
        ><ArrowLeft :size="17" aria-hidden="true"
      /></RouterLink>
      <div class="project-avatar" aria-hidden="true">
        {{ projectInitials(project.displayName) }}
      </div>
      <div>
        <div class="title-line">
          <h1 id="project-title">{{ project.displayName }}</h1>
          <span class="status" :class="`status--${stateTone}`"><i></i>{{ stateLabel(state) }}</span>
          <span
            v-if="partial"
            class="observation-state"
            role="status"
            title="One or more observations are unavailable. Cached project controls and evidence remain usable."
            >Partial data</span
          >
        </div>
        <p>{{ project.primaryLocation }}</p>
      </div>
    </div>
    <div class="hero-actions">
      <button
        v-if="browserAction"
        type="button"
        :disabled="actionPending"
        @click="$emit('action', browserAction)"
      >
        <ExternalLink :size="16" aria-hidden="true" />Open app
      </button>
      <button type="button" @click="$emit('terminal')">
        <Terminal :size="16" aria-hidden="true" />Terminal
      </button>
      <button
        v-if="active"
        type="button"
        :disabled="lifecyclePending"
        @click="$emit('lifecycle', 'restart', [])"
      >
        <RefreshCw :size="16" aria-hidden="true" />Restart
      </button>
      <button class="primary" type="button" :disabled="lifecyclePending" @click="requestLifecycle">
        <Square v-if="active" :size="14" fill="currentColor" aria-hidden="true" />
        <Play v-else :size="16" aria-hidden="true" />
        {{ active ? 'Stop' : 'Start' }}
      </button>
    </div>
  </header>
  <form
    v-if="showStartOptions"
    class="start-options"
    role="dialog"
    aria-labelledby="start-options-title"
    @submit.prevent="startProject"
    @keydown.esc="closeStartOptions"
  >
    <div>
      <strong id="start-options-title">Start {{ project.displayName }} services</strong>
      <p>Core services always start. Include any optional Compose profiles for this run.</p>
    </div>
    <fieldset>
      <legend>Optional profiles</legend>
      <label v-for="profile in availableProfiles" :key="profile">
        <input v-model="selectedProfiles" type="checkbox" :value="profile" />
        <span>{{ profileLabel(profile) }}</span>
      </label>
    </fieldset>
    <div class="start-options__actions">
      <button type="button" @click="closeStartOptions">Cancel</button>
      <button class="primary" type="submit" :disabled="lifecyclePending">
        <Play :size="15" aria-hidden="true" />Start services
      </button>
    </div>
  </form>
  <p v-if="operationError" class="message message--error" role="alert">
    {{ operationError }}
  </p>
  <p v-if="dockerUnavailable" class="message" role="status">
    Docker is unavailable. Catalog, Git, manifest, and persisted logs remain available.
  </p>
</template>
