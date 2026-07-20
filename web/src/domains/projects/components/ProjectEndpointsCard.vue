<script setup lang="ts">
import { ExternalLink } from '@lucide/vue'

import type { ActionDefinition } from '../../../api/generated/types.gen'

defineProps<{
  endpoints: Array<ActionDefinition>
  actionPending: boolean
}>()
const emit = defineEmits<{
  runAction: [action: ActionDefinition]
}>()

function endpointLabel(action: ActionDefinition) {
  return action.target?.replace(/^https?:\/\//, '') ?? action.name
}
</script>

<template>
  <article class="panel endpoints-panel">
    <header class="panel-head">
      <div>
        <p>Project systems</p>
        <h2>Endpoints</h2>
      </div>
      <span>{{ endpoints.length }}</span>
    </header>
    <div class="endpoint-list">
      <button
        v-for="endpoint in endpoints"
        :key="endpoint.id"
        type="button"
        :disabled="actionPending"
        :title="endpoint.target"
        @click="emit('runAction', endpoint)"
      >
        <span>
          <strong>{{ endpoint.name }}</strong>
          <small>{{ endpointLabel(endpoint) }}</small>
        </span>
        <ExternalLink :size="15" aria-hidden="true" />
      </button>
    </div>
  </article>
</template>
