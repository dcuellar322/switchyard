<script setup lang="ts">
import type { EffectiveManifest } from '../../../api/generated/types.gen'

defineProps<{
  manifest?: EffectiveManifest
  pending: boolean
  error: boolean
}>()
</script>

<template>
  <article class="panel">
    <header class="panel-head">
      <div>
        <p>{{ manifest?.sources.length ?? 0 }} sources</p>
        <h2>Effective manifest</h2>
      </div>
      <span>Read-only provenance</span>
    </header>
    <p v-if="pending" class="panel-state">Resolving manifest provenance…</p>
    <p v-else-if="error" class="panel-state message--error" role="alert">
      Effective manifest is unavailable.
    </p>
    <pre v-else-if="manifest"><code>{{ JSON.stringify(manifest.manifest, null, 2) }}</code></pre>
    <p v-else class="panel-state">No effective manifest is available.</p>
  </article>
</template>
