<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'

import { loadProjects } from '../../projects/api'
import {
  acknowledgeNotification,
  diagnoseProject,
  evaluateRecipes,
  loadLatestDiagnosis,
  loadNotifications,
  loadProviders,
  loadRecipes,
  reviewHypothesis,
  runSuggestedAction,
  saveRecipe,
  setRecipeEnabled,
} from '../api'

const queryClient = useQueryClient()
const selectedProject = ref('')
const selectedProvider = ref('')
const recipeName = ref('Respond to repeated crashes')
const recipeTrigger = ref<'REPEATED_CRASH' | 'PORT_CONFLICT' | 'RESOURCE_PRESSURE' | 'UNHEALTHY_DEPENDENCY'>('REPEATED_CRASH')
const recipeAction = ref('')
const notice = ref('')

const projects = useQuery({ queryKey: ['projects'], queryFn: loadProjects })
watch(() => projects.data.value, (values) => {
  if (!selectedProject.value && values?.length) selectedProject.value = values[0]!.id
}, { immediate: true })
const providers = useQuery({ queryKey: ['ai-providers'], queryFn: loadProviders })
const diagnosis = useQuery({
  queryKey: computed(() => ['diagnosis', selectedProject.value]),
  queryFn: () => loadLatestDiagnosis(selectedProject.value),
  enabled: computed(() => Boolean(selectedProject.value)),
  retry: false,
})
const recipes = useQuery({
  queryKey: computed(() => ['automation-recipes', selectedProject.value]),
  queryFn: () => loadRecipes(selectedProject.value),
  enabled: computed(() => Boolean(selectedProject.value)),
})
const notifications = useQuery({
  queryKey: computed(() => ['diagnostic-notifications', selectedProject.value]),
  queryFn: () => loadNotifications(selectedProject.value),
  enabled: computed(() => Boolean(selectedProject.value)),
})
const availableProviders = computed(() => providers.data.value?.filter((item) => item.available) ?? [])
const suggestedActions = computed(() => {
  const values = diagnosis.data.value?.hypotheses.flatMap((item) => item.suggestedActions) ?? []
  return [...new Map(values.map((item) => [item.actionId, item])).values()]
})
watch(suggestedActions, (values) => {
  if (!recipeAction.value && values.length) recipeAction.value = values[0]!.actionId
}, { immediate: true })

const run = useMutation({
  mutationFn: () => diagnoseProject(selectedProject.value, selectedProvider.value || undefined),
  onSuccess: (value) => {
    queryClient.setQueryData(['diagnosis', selectedProject.value], value)
    void queryClient.invalidateQueries({ queryKey: ['diagnostic-notifications', selectedProject.value] })
    notice.value = 'Fresh bounded evidence collected. Deterministic rules ran before optional AI.'
  },
})
const feedback = useMutation({ mutationFn: (input: { hypothesisId: string; verdict: 'accurate' | 'false_positive' }) => reviewHypothesis(diagnosis.data.value!.id, input.hypothesisId, input.verdict), onSuccess: () => { notice.value = 'Feedback stored locally. Nothing was sent as telemetry.' } })
const action = useMutation({ mutationFn: (actionId: string) => runSuggestedAction(diagnosis.data.value!.id, actionId), onSuccess: (value) => { notice.value = `Approved action queued as ${value.id}.` } })
const createRecipe = useMutation({ mutationFn: () => saveRecipe({ projectId: selectedProject.value, name: recipeName.value, triggerCode: recipeTrigger.value, actionId: recipeAction.value, cooldownSeconds: 3600, maxRunsPerDay: 3 }), onSuccess: () => { void queryClient.invalidateQueries({ queryKey: ['automation-recipes', selectedProject.value] }); notice.value = 'Recipe saved disabled for separate review.' } })
const toggleRecipe = useMutation({ mutationFn: (input: { id: string; enabled: boolean }) => setRecipeEnabled(input.id, input.enabled), onSuccess: () => { void queryClient.invalidateQueries({ queryKey: ['automation-recipes', selectedProject.value] }) } })
const evaluate = useMutation({ mutationFn: () => evaluateRecipes(selectedProject.value), onSuccess: (ids) => { notice.value = ids.length ? `${ids.length} safe operation(s) dispatched.` : 'No enabled recipe was due.' } })
const acknowledge = useMutation({ mutationFn: (id: string) => acknowledgeNotification(id), onSuccess: () => { void queryClient.invalidateQueries({ queryKey: ['diagnostic-notifications', selectedProject.value] }); notice.value = 'Diagnostic notification acknowledged locally.' } })

function percent(value: number) { return `${Math.round(value * 100)}%` }
</script>

<template>
  <main class="diagnostics page-shell">
    <header class="page-heading">
      <div><p class="eyebrow">Intelligent diagnosis</p><h1>Evidence first. Automation on a leash.</h1><p>Known failures are resolved deterministically. Optional AI can only cite the bounded evidence and existing approved actions shown here.</p></div>
      <div class="controls">
        <label>Project<select v-model="selectedProject"><option v-for="project in projects.data.value" :key="project.id" :value="project.id">{{ project.displayName }}</option></select></label>
        <label>Optional AI<select v-model="selectedProvider"><option value="">Deterministic only</option><option v-for="provider in availableProviders" :key="provider.id" :value="provider.id">{{ provider.name }}</option></select></label>
        <button class="primary" :disabled="!selectedProject || run.isPending.value" @click="run.mutate()">{{ run.isPending.value ? 'Collecting…' : 'Run diagnosis' }}</button>
      </div>
    </header>

    <p v-if="notice" class="notice" role="status">{{ notice }}</p>
    <p v-if="projects.isError.value" class="error" role="alert">The trusted project catalog is unavailable. Diagnosis cannot start until the daemon reconnects.</p>
    <p v-if="providers.isError.value" class="notice" role="status">Optional AI providers are unavailable. Deterministic diagnosis remains fully available.</p>
    <p v-if="run.isError.value || diagnosis.isError.value" class="error" role="alert">Diagnostic evidence is unavailable. Verify runtime connectivity and try again.</p>
    <p v-if="feedback.isError.value || action.isError.value || createRecipe.isError.value || toggleRecipe.isError.value || evaluate.isError.value || acknowledge.isError.value" class="error" role="alert">The reviewed diagnostic change was not accepted. Inspect the current permissions and recipe limits, then try again.</p>
    <section v-if="diagnosis.isLoading.value" class="panel empty">Collecting the latest durable result…</section>
    <section v-else-if="!diagnosis.data.value" class="panel empty"><strong>No diagnosis yet.</strong><span>Choose a trusted project and run deterministic analysis.</span></section>
    <template v-else>
      <section class="receipt panel">
        <div><span class="status-dot"></span><strong>{{ diagnosis.data.value.hypotheses.length }} hypotheses</strong><small>{{ diagnosis.data.value.bundleBytes.toLocaleString() }} bytes · {{ diagnosis.data.value.evidence.length }} evidence items</small></div>
        <span class="badge">{{ diagnosis.data.value.deterministic ? 'Deterministic' : `AI assisted · ${diagnosis.data.value.model ?? diagnosis.data.value.provider}` }}</span>
      </section>
      <div class="grid">
        <section class="findings panel" aria-label="Diagnostic hypotheses">
          <div class="section-heading"><div><p class="eyebrow">Ranked findings</p><h2>Hypotheses and evidence</h2></div><span>{{ new Date(diagnosis.data.value.generatedAt).toLocaleString() }}</span></div>
          <article v-for="item in diagnosis.data.value.hypotheses" :key="item.id" class="finding" :class="`finding--${item.severity}`">
            <div class="finding-head"><span class="badge">{{ item.source }}</span><strong>{{ item.title }}</strong><b>{{ percent(item.confidence) }}</b></div>
            <p>{{ item.summary }}</p><small>Evidence: {{ item.evidenceIds.join(', ') }}</small>
            <div class="finding-actions"><button v-for="suggestion in item.suggestedActions" :key="suggestion.actionId" :disabled="action.isPending.value" @click="action.mutate(suggestion.actionId)">{{ suggestion.name }}</button><button class="quiet" @click="feedback.mutate({ hypothesisId: item.id, verdict: 'accurate' })">Accurate</button><button class="quiet" @click="feedback.mutate({ hypothesisId: item.id, verdict: 'false_positive' })">False positive</button></div>
          </article>
          <div v-if="diagnosis.data.value.hypotheses.length === 0" class="empty"><strong>No known failure pattern.</strong><span>The bounded evidence remains available for review.</span></div>
          <details><summary>Review bounded evidence</summary><ul><li v-for="item in diagnosis.data.value.evidence" :key="item.id"><code>{{ item.id }}</code> {{ item.summary }} <span v-if="item.untrusted">untrusted data</span><span v-if="item.redacted">redacted</span></li></ul></details>
        </section>

        <aside class="side-stack">
          <section class="panel safety"><p class="eyebrow">Safety envelope</p><h2>Nothing hidden</h2><ul><li>Logs and repository text are inert, redacted data.</li><li>Suggested buttons reference accepted actions only.</li><li>Cleanup is a non-executable dry run: {{ diagnosis.data.value.cleanupPreview.candidates }} candidates, {{ diagnosis.data.value.cleanupPreview.estimatedBytes.toLocaleString() }} bytes.</li><li>No source edits or deletion run automatically.</li></ul></section>
          <section class="panel"><div class="section-heading"><div><p class="eyebrow">Local alerts</p><h2>Notifications</h2></div><span>{{ notifications.data.value?.length ?? 0 }}</span></div><div v-if="notifications.isError.value" class="error">Notifications unavailable.</div><article v-for="item in notifications.data.value" :key="item.id" class="notification"><strong>{{ item.title }}</strong><span>{{ item.occurrences }}× · {{ item.code }}</span><p>{{ item.detail }}</p><button class="quiet" :disabled="acknowledge.isPending.value" @click="acknowledge.mutate(item.id)">Acknowledge</button></article><p v-if="notifications.data.value?.length === 0" class="muted">No unreviewed crash, port, resource, or dependency alerts.</p></section>
        </aside>
      </div>

      <section class="panel automations">
        <div class="section-heading"><div><p class="eyebrow">Saved automation</p><h2>Explicit triggers and limits</h2></div><button :disabled="evaluate.isPending.value" @click="evaluate.mutate()">Evaluate now</button></div>
        <div class="recipe-form"><input v-model="recipeName" aria-label="Recipe name"><select v-model="recipeTrigger" aria-label="Recipe trigger"><option value="REPEATED_CRASH">Repeated crash</option><option value="PORT_CONFLICT">Port conflict</option><option value="RESOURCE_PRESSURE">Resource pressure</option><option value="UNHEALTHY_DEPENDENCY">Unhealthy dependency</option></select><select v-model="recipeAction" aria-label="Approved action"><option value="">Choose an approved suggestion</option><option v-for="item in suggestedActions" :key="item.actionId" :value="item.actionId">{{ item.name }} · {{ item.risk }}</option></select><button :disabled="!recipeAction || createRecipe.isPending.value" @click="createRecipe.mutate()">Save disabled</button></div>
        <div v-if="recipes.isError.value" class="error">Automation recipes unavailable.</div>
        <article v-for="recipe in recipes.data.value" :key="recipe.id" class="recipe"><div><strong>{{ recipe.name }}</strong><span>{{ recipe.triggerCode }} → {{ recipe.actionId }}</span></div><small>Cooldown {{ recipe.cooldownSeconds / 60 }}m · {{ recipe.runsToday }}/{{ recipe.maxRunsPerDay }} today</small><button :class="{ danger: recipe.enabled }" @click="toggleRecipe.mutate({ id: recipe.id, enabled: !recipe.enabled })">{{ recipe.enabled ? 'Disable' : 'Enable' }}</button></article>
        <p v-if="recipes.data.value?.length === 0" class="muted">No recipes saved. Recipes are created disabled and require a separate enable decision.</p>
      </section>
    </template>
  </main>
</template>

<style scoped>
.diagnostics{display:grid;gap:18px}.page-heading{display:flex;justify-content:space-between;gap:28px;align-items:end}.page-heading h1{max-width:720px;margin:4px 0 8px;font-size:clamp(28px,4vw,44px);letter-spacing:-.04em}.page-heading p{max-width:720px;color:var(--muted)}.eyebrow{margin:0;color:var(--accent);font-size:10px;font-weight:800;letter-spacing:.14em;text-transform:uppercase}.controls{display:flex;align-items:end;gap:8px}.controls label{display:grid;gap:5px;color:var(--soft);font-size:10px;text-transform:uppercase}.controls select,.recipe-form select,.recipe-form input{min-height:38px;padding:0 10px;border:1px solid var(--border);border-radius:8px;background:var(--panel-strong);color:var(--text)}button{min-height:36px;padding:0 12px;border:1px solid var(--border);border-radius:8px;background:var(--panel-strong);color:var(--text);cursor:pointer}.primary{border-color:rgba(120,166,255,.55);background:linear-gradient(135deg,var(--accent),var(--accent-2));color:#07111f;font-weight:800}button:disabled{opacity:.45;cursor:not-allowed}.panel{padding:18px;border:1px solid var(--border);border-radius:14px;background:var(--panel)}.notice,.error{margin:0;padding:11px 14px;border-radius:9px}.notice{border:1px solid rgba(86,211,159,.3);background:rgba(86,211,159,.08);color:var(--success)}.error{border:1px solid rgba(255,117,117,.35);background:rgba(255,117,117,.08);color:#ff9b9b}.empty{display:grid;gap:5px;place-items:center;min-height:150px;color:var(--muted)}.receipt{display:flex;align-items:center;justify-content:space-between}.receipt>div{display:flex;align-items:center;gap:10px}.receipt small{color:var(--muted)}.status-dot{width:9px;height:9px;border-radius:50%;background:var(--success);box-shadow:0 0 16px var(--success)}.badge{padding:3px 7px;border:1px solid var(--border);border-radius:99px;color:var(--accent);font-size:10px;text-transform:uppercase}.grid{display:grid;grid-template-columns:minmax(0,1.65fr) minmax(280px,.75fr);gap:18px}.side-stack{display:grid;align-content:start;gap:18px}.section-heading,.finding-head,.recipe{display:flex;align-items:center;justify-content:space-between;gap:12px}.section-heading h2,.safety h2{margin:3px 0 0;font-size:18px}.section-heading>span,.muted{color:var(--muted);font-size:12px}.finding{margin-top:12px;padding:15px;border:1px solid var(--border);border-left:3px solid var(--soft);border-radius:10px;background:rgba(255,255,255,.015)}.finding--error{border-left-color:#ff7575}.finding--warning{border-left-color:#ffca6a}.finding p,.notification p{margin:8px 0;color:var(--muted);line-height:1.45}.finding small{color:var(--soft)}.finding-head strong{margin-right:auto}.finding-head b{font-size:12px}.finding-actions{display:flex;gap:7px;flex-wrap:wrap;margin-top:12px}.quiet{background:transparent;color:var(--muted)}details{margin-top:14px;color:var(--muted)}details li{margin:7px 0}details span{margin-left:7px;color:var(--warning);font-size:10px;text-transform:uppercase}.safety ul{display:grid;gap:10px;padding-left:18px;color:var(--muted);line-height:1.4}.notification{padding:12px 0;border-bottom:1px solid var(--border)}.notification>span{display:block;margin-top:3px;color:var(--soft);font-size:10px}.recipe-form{display:grid;grid-template-columns:1.1fr .8fr 1fr auto;gap:8px;margin:16px 0}.recipe{padding:12px 0;border-top:1px solid var(--border)}.recipe>div{display:grid;gap:4px}.recipe span,.recipe small{color:var(--muted);font-size:11px}.danger{border-color:rgba(255,117,117,.35);color:#ff9b9b}@media(max-width:1050px){.page-heading{align-items:stretch;flex-direction:column}.controls{flex-wrap:wrap}.grid{grid-template-columns:1fr}.recipe-form{grid-template-columns:1fr 1fr}}@media(max-width:700px){.controls,.recipe-form{display:grid;grid-template-columns:1fr}.receipt,.recipe{align-items:flex-start;flex-direction:column}}
</style>
