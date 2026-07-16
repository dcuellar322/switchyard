<script setup lang="ts">
import type { AiManifestEnhancement, Operation } from '../../../api/generated/types.gen'

defineProps<{ operation?: Operation; run?: AiManifestEnhancement; cancelling: boolean }>()
defineEmits<{ cancel: [] }>()
</script>

<template>
  <article v-if="operation || run" class="review" aria-live="polite">
    <div class="review__heading">
      <div><p class="eyebrow">Provider operation</p><h2>Assisted proposal review</h2></div>
      <span class="state" :class="`state--${run?.state ?? operation?.state}`">{{ run?.state ?? operation?.state }}</span>
    </div>
    <p v-if="operation && ['queued', 'running'].includes(operation.state)" class="message">The provider is working against the approved evidence bundle. You can cancel without changing the deterministic proposal.</p>
    <button v-if="operation && ['queued', 'running'].includes(operation.state)" class="button--danger" type="button" :disabled="cancelling" @click="$emit('cancel')">{{ cancelling ? 'Cancelling…' : 'Cancel provider run' }}</button>
    <p v-if="run?.errorMessage" class="message message--error" role="alert">{{ run.errorMessage }}</p>

    <template v-if="run?.state === 'succeeded'">
      <div class="dry-run" :class="{ 'dry-run--ready': run.dryRun.valid }">
        <strong>{{ run.dryRun.valid ? 'Dry-run passed' : 'Dry-run needs review' }}</strong>
        <span>Schema {{ run.dryRun.schemaValid ? 'valid' : 'invalid' }}</span>
        <span>Evidence {{ run.dryRun.evidenceBacked ? 'backed' : 'filtered' }}</span>
        <span>Repository {{ run.dryRun.repositorySafe ? 'safe' : 'unsafe' }}</span>
      </div>
      <p v-for="warning in [...run.warnings, ...run.dryRun.warnings]" :key="warning" class="message">{{ warning }}</p>
      <p v-for="error in run.dryRun.errors" :key="error" class="message message--error">{{ error }}</p>
      <section v-if="run.fields.length">
        <h3>Field provenance</h3>
        <div class="field-table" role="table" aria-label="Assisted field provenance">
          <div v-for="field in run.fields" :key="`${field.path}-${field.source}`" class="field-row" role="row">
            <code>{{ field.path }}</code><span :class="`source source--${field.source}`">{{ field.source }}</span><strong>{{ Math.round(field.confidence * 100) }}%</strong>
            <p>{{ field.rationale || field.warnings.join(' ') }}</p><small>{{ field.evidenceIds.join(', ') || 'deterministic rule' }}</small>
          </div>
        </div>
      </section>
      <section v-if="run.conflicts.length">
        <h3>Conflicts resolved safely</h3>
        <div v-for="conflict in run.conflicts" :key="conflict.path" class="conflict">
          <code>{{ conflict.path }}</code><strong>Kept deterministic value</strong>
          <span>Provider proposed <code>{{ JSON.stringify(conflict.proposedValue) }}</code></span>
        </div>
      </section>
      <details class="receipt"><summary>Provider receipt · {{ run.bundleSha256.slice(0, 12) }}…</summary><pre>{{ JSON.stringify(run.bundle, null, 2) }}</pre></details>
    </template>
  </article>
</template>

<style scoped>
.review{grid-column:1/-1;padding:22px;border:1px solid var(--border);border-radius:14px;background:linear-gradient(145deg,rgba(21,27,36,.96),rgba(14,19,26,.96))}.review__heading{display:flex;justify-content:space-between;gap:18px}.review h2{margin:5px 0 14px;font-size:20px}.review h3{margin:20px 0 10px;font-size:15px}.eyebrow{margin:0;color:var(--accent);text-transform:uppercase;letter-spacing:.14em;font-size:10px;font-weight:800}.state{height:min-content;padding:5px 9px;border-radius:999px;color:var(--yellow);background:rgba(241,199,91,.1);text-transform:capitalize}.state--succeeded{color:var(--green);background:rgba(84,212,154,.1)}.state--failed,.state--cancelled{color:var(--red);background:rgba(255,115,115,.08)}.message{padding:10px 12px;border-radius:8px;background:rgba(241,199,91,.08);color:var(--yellow)}.message--error{background:rgba(255,115,115,.08);color:var(--red)}.button--danger{padding:9px 13px;border:1px solid rgba(255,115,115,.4);border-radius:8px;color:var(--red);background:transparent;font-weight:800}.dry-run{display:flex;flex-wrap:wrap;gap:10px;margin:12px 0;padding:12px;border:1px solid rgba(241,199,91,.3);border-radius:9px}.dry-run--ready{border-color:rgba(84,212,154,.35)}.dry-run span{color:var(--muted);font-size:12px}.field-table{display:grid;gap:7px}.field-row{display:grid;grid-template-columns:1.2fr auto 55px;gap:10px;padding:11px;border:1px solid var(--border);border-radius:8px;background:#0d1219}.field-row p,.field-row small{grid-column:1/-1;margin:0;color:var(--muted);font-size:12px}.source{padding:2px 7px;border-radius:999px;color:var(--cyan);background:rgba(83,174,255,.1);font-size:10px}.source--rejected{color:var(--red);background:rgba(255,115,115,.08)}.conflict{display:grid;grid-template-columns:1fr auto;gap:7px;padding:10px;border-left:3px solid var(--yellow);background:#0d1219}.conflict span{grid-column:1/-1;color:var(--muted);font-size:12px}.receipt{margin-top:14px;border:1px solid var(--border);border-radius:8px}.receipt summary{padding:10px;cursor:pointer}.receipt pre{max-height:340px;margin:0;padding:10px;border-top:1px solid var(--border);overflow:auto;white-space:pre-wrap;color:var(--cyan);font-size:11px}
</style>
