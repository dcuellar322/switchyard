<script setup lang="ts">
import { RouterLink } from "vue-router";

import type {
  ActionDefinition,
  Project,
  RuntimeAction,
} from "../../../api/generated/types.gen";
import { projectInitials, stateLabel } from "../../../lib/format";

defineProps<{
  project: Project;
  state: string;
  stateTone: string;
  active: boolean;
  browserAction?: ActionDefinition;
  terminalAction?: ActionDefinition;
  actionPending: boolean;
  lifecyclePending: boolean;
  operationError: string;
  partial: boolean;
  dockerUnavailable: boolean;
}>();
defineEmits<{
  action: [action: ActionDefinition | undefined];
  lifecycle: [action: RuntimeAction];
}>();
</script>

<template>
  <header class="project-hero">
    <div class="hero-identity">
      <RouterLink class="back" to="/projects" aria-label="Back to projects"
        >←</RouterLink
      >
      <div class="project-avatar" aria-hidden="true">
        {{ projectInitials(project.displayName) }}
      </div>
      <div>
        <div class="title-line">
          <h1 id="project-title">{{ project.displayName }}</h1>
          <span class="status" :class="`status--${stateTone}`"
            ><i></i>{{ stateLabel(state) }}</span
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
        ↗ Open app
      </button>
      <button
        type="button"
        :disabled="actionPending || !terminalAction"
        @click="$emit('action', terminalAction)"
      >
        ⌘ Terminal
      </button>
      <button
        v-if="active"
        type="button"
        :disabled="lifecyclePending"
        @click="$emit('lifecycle', 'restart')"
      >
        ↻ Restart
      </button>
      <button
        class="primary"
        type="button"
        :disabled="lifecyclePending"
        @click="$emit('lifecycle', active ? 'stop' : 'start')"
      >
        {{ active ? "■ Stop" : "▶ Start" }}
      </button>
    </div>
  </header>
  <p v-if="operationError" class="message message--error" role="alert">
    {{ operationError }}
  </p>
  <p v-if="partial" class="message" role="status">
    Some observations are unavailable. Available project controls and evidence
    remain usable.
  </p>
  <p v-if="dockerUnavailable" class="message" role="status">
    Docker is unavailable. Catalog, Git, manifest, and persisted logs remain
    available.
  </p>
</template>
