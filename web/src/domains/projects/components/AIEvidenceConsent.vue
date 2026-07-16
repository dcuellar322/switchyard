<script setup lang="ts">
import type { AiEvidencePreview, AiProviderDescriptor } from '../../../api/generated/types.gen'

defineProps<{
  providers: Array<AiProviderDescriptor>
  preview?: AiEvidencePreview
  selectedProvider: string
  consented: boolean
  pending: boolean
}>()

const emit = defineEmits<{
  preview: []
  start: []
  'update:selectedProvider': [value: string]
  'update:consented': [value: boolean]
}>()
</script>

<template>
  <article class="ai-card" aria-labelledby="ai-onboarding-title">
    <div class="ai-card__heading">
      <div>
        <p class="eyebrow">Optional assisted onboarding</p>
        <h2 id="ai-onboarding-title">Resolve ambiguous setup with a constrained provider</h2>
      </div>
      <span class="boundary">No repository access · no tools · no auto-approval</span>
    </div>
    <p class="explanation">Switchyard sends only the redacted evidence shown below. Deterministic facts keep priority, and every provider change must cite evidence.</p>
    <label for="ai-provider">Proposal provider</label>
    <select id="ai-provider" :value="selectedProvider" :disabled="pending" @change="emit('update:selectedProvider', ($event.target as HTMLSelectElement).value)">
      <option value="">Choose a provider</option>
      <option v-for="provider in providers" :key="provider.id" :value="provider.id" :disabled="!provider.available">
        {{ provider.name }}{{ provider.model ? ` · ${provider.model}` : '' }}{{ provider.available ? '' : ' · unavailable' }}
      </option>
    </select>
    <ul v-if="providers.some((provider) => !provider.available)" class="provider-notes">
      <li v-for="provider in providers.filter((item) => !item.available)" :key="provider.id"><strong>{{ provider.name }}</strong>: {{ provider.reason }}</li>
    </ul>
    <div class="ai-actions">
      <button class="button--secondary" type="button" :disabled="pending || !selectedProvider" @click="emit('preview')">Preview exact evidence</button>
    </div>

    <section v-if="preview" class="receipt" aria-labelledby="evidence-receipt-title">
      <div class="receipt__summary">
        <div><span>Payload</span><strong>{{ preview.bundle.encodedBytes.toLocaleString() }} bytes</strong></div>
        <div><span>Findings</span><strong>{{ preview.bundle.evidence.length }}</strong></div>
        <div><span>Redactions</span><strong>{{ preview.bundle.redactionCount }}</strong></div>
        <div><span>Digest</span><code>{{ preview.sha256.slice(0, 12) }}…</code></div>
      </div>
      <h3 id="evidence-receipt-title">Evidence crossing the provider boundary</h3>
      <div class="evidence-list">
        <details v-for="item in preview.bundle.evidence" :key="item.id">
          <summary><strong>{{ item.kind }}</strong><code>{{ item.sourcePath }}:{{ item.location.startLine }}</code><span>{{ Math.round(item.confidence * 100) }}%</span></summary>
          <pre v-if="item.excerpt">{{ item.excerpt }}</pre>
          <pre>{{ JSON.stringify(item.data, null, 2) }}</pre>
          <p v-for="warning in item.warnings" :key="warning" class="warning">{{ warning }}</p>
        </details>
      </div>
      <details class="exact-payload">
        <summary>Inspect exact immutable JSON payload</summary>
        <pre>{{ JSON.stringify(preview.encoded, null, 2) }}</pre>
      </details>
      <label class="consent">
        <input type="checkbox" :checked="consented" @change="emit('update:consented', ($event.target as HTMLInputElement).checked)" />
        I reviewed this exact redacted payload and authorize sending it to the selected provider.
      </label>
      <div class="ai-actions"><button type="button" :disabled="pending || !consented" @click="emit('start')">Generate reviewable proposal</button></div>
    </section>
  </article>
</template>

<style scoped>
.ai-card{grid-column:1/-1;padding:22px;border:1px solid rgba(83,174,255,.32);border-radius:14px;background:linear-gradient(145deg,rgba(18,31,46,.96),rgba(14,19,26,.96))}.ai-card__heading{display:flex;justify-content:space-between;gap:18px}.ai-card h2{margin:5px 0 10px;font-size:20px}.eyebrow{margin:0;color:var(--accent);text-transform:uppercase;letter-spacing:.14em;font-size:10px;font-weight:800}.boundary{height:min-content;padding:6px 9px;border:1px solid var(--border);border-radius:999px;color:var(--soft);font-size:11px;white-space:nowrap}.explanation,.provider-notes{color:var(--muted);font-size:13px}.provider-notes{padding-left:20px}label{display:block;margin:16px 0 8px;font-weight:700}select{width:100%;padding:10px 12px;color:var(--text);border:1px solid #344157;border-radius:9px;background:#0b1017}.ai-actions{display:flex;justify-content:flex-end;margin-top:14px}button{padding:10px 15px;border:0;border-radius:9px;color:#07111f;background:var(--accent);font-weight:800;cursor:pointer}button:disabled{opacity:.48;cursor:not-allowed}.button--secondary{border:1px solid #40506a;color:var(--text);background:transparent}.receipt{margin-top:18px;padding-top:18px;border-top:1px solid var(--border)}.receipt__summary{display:grid;grid-template-columns:repeat(4,1fr);gap:8px}.receipt__summary div{display:grid;gap:5px;padding:10px;border:1px solid var(--border);border-radius:8px;background:#0d1219}.receipt__summary span{color:var(--soft);font-size:11px}.evidence-list{display:grid;gap:7px}.evidence-list details,.exact-payload{border:1px solid var(--border);border-radius:8px;background:#0d1219}.evidence-list summary{display:grid;grid-template-columns:1fr 2fr auto;gap:12px;padding:10px 12px;cursor:pointer}.evidence-list details>pre,.evidence-list details>p,.exact-payload pre{margin:0;padding:10px 12px;border-top:1px solid var(--border);white-space:pre-wrap;overflow:auto;color:var(--cyan);font-size:11px}.exact-payload{margin-top:10px}.exact-payload summary{padding:10px 12px;cursor:pointer}.warning{color:var(--yellow)!important}.consent{display:flex;gap:10px;align-items:flex-start;padding:12px;border-radius:8px;background:rgba(84,212,154,.07);font-size:13px}.consent input{margin-top:2px}@media(max-width:760px){.ai-card__heading{display:grid}.boundary{width:max-content;white-space:normal}.receipt__summary{grid-template-columns:1fr 1fr}.evidence-list summary{grid-template-columns:1fr}}
</style>
