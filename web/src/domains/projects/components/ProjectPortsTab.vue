<script setup lang="ts">
import { RouterLink } from 'vue-router'

import type { PortFact } from '../../../api/generated/types.gen'

defineProps<{ ports: Array<PortFact> }>()
</script>

<template>
  <article class="panel">
    <header class="panel-head">
      <div>
        <p>Registry evidence</p>
        <h2>Project ports</h2>
      </div>
      <RouterLink to="/ports">Open global registry →</RouterLink>
    </header>
    <div class="fact-grid">
      <div v-for="fact in ports" :key="fact.id">
        <dt>{{ fact.port }}/{{ fact.protocol }}</dt>
        <dd>{{ fact.serviceId ?? fact.kind }} · {{ fact.source }}</dd>
      </div>
      <p v-if="!ports.length" class="panel-state">No declared, reserved, or observed ports.</p>
    </div>
  </article>
</template>
