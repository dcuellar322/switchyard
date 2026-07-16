<script setup lang="ts">
import { computed, ref, watch } from 'vue'

import type { FleetCapability, FleetSnapshot, Machine, MachineAccessRequest, RemoteOperationRequest } from '../../../api/generated/types.gen'

const props = defineProps<{ machine: Machine; snapshot?: FleetSnapshot; pending: boolean; readOnly: boolean }>()
const emit = defineEmits<{
  probe: []; access: [request: MachineAccessRequest]; remove: [];
  run: [request: RemoteOperationRequest]; refreshSnapshot: [];
}>()
const knownCapabilities: Array<FleetCapability> = ['inventory.read', 'project.operate', 'environment.manage']
const grants = ref<Array<FleetCapability>>([])
const reviewed = ref(false)
const projectId = ref('')
const environmentId = ref('')
const action = ref<RemoteOperationRequest['action']>('start')
const runReviewed = ref(false)

watch(() => props.machine, (machine) => {
  grants.value = [...machine.grantedCapabilities]
  reviewed.value = false
}, { immediate: true })
watch(() => props.snapshot, (snapshot) => {
  if (!snapshot) return
  if (!snapshot.projects.some((project) => project.id === projectId.value)) projectId.value = snapshot.projects[0]?.id ?? ''
}, { immediate: true })
const selectedEnvironment = computed(() => props.snapshot?.environments.find((item) => item.id === environmentId.value))

function toggle(capability: FleetCapability) {
  if (capability === 'inventory.read') return
  grants.value = grants.value.includes(capability) ? grants.value.filter((item) => item !== capability) : [...grants.value, capability]
  reviewed.value = false
}
function saveAccess() {
  emit('access', { enabled: true, grantedCapabilities: grants.value, confirmRisk: reviewed.value })
}
function run() {
  const targetProject = selectedEnvironment.value?.projectId ?? projectId.value
  emit('run', {
    requestId: `ui_${window.crypto.randomUUID()}`, projectId: targetProject,
    environmentId: selectedEnvironment.value?.id, action: action.value, confirmRisk: runReviewed.value,
  })
  runReviewed.value = false
}
</script>

<template>
  <main class="detail">
    <section class="panel identity">
      <div class="panel-head"><div><p>Authenticated peer</p><h2>{{ machine.name }}</h2></div><span :class="`state state--${machine.state}`">{{ machine.state }}</span></div>
      <dl><div><dt>Endpoint</dt><dd>{{ machine.endpoint }}</dd></div><div><dt>Peer</dt><dd>{{ machine.peerId ?? 'Pending first identity' }}</dd></div><div><dt>Version</dt><dd>{{ machine.peerVersion ?? '—' }}</dd></div><div><dt>Platform</dt><dd>{{ [machine.os, machine.architecture].filter(Boolean).join(' / ') || '—' }}</dd></div><div class="fingerprint"><dt>Pinned server certificate</dt><dd><code>{{ machine.certificateFingerprint }}</code></dd></div></dl>
      <p v-if="machine.lastError" class="error" role="alert">{{ machine.lastError }}</p>
      <div class="actions"><button type="button" :disabled="pending" @click="emit('refreshSnapshot')">Refresh inventory</button><button v-if="!readOnly" type="button" :disabled="pending" @click="emit('probe')">Probe identity</button></div>
    </section>

    <section v-if="snapshot" class="panel inventory">
      <div class="panel-head"><div><p>Bounded inventory</p><h2>{{ snapshot.projects.length }} projects · {{ snapshot.environments.length }} environments</h2></div><time :datetime="snapshot.observedAt">{{ new Date(snapshot.observedAt).toLocaleString() }}</time></div>
      <div v-if="snapshot.projects.length" class="projects">
        <article v-for="project in snapshot.projects" :key="project.id" :class="{ degraded: project.degraded }"><div><strong>{{ project.displayName }}</strong><small>{{ project.slug }} · {{ project.runtime }}</small></div><span>{{ project.state }}</span><i>{{ project.health }}</i></article>
      </div>
      <p v-else class="empty">This peer has no trusted projects to share.</p>
    </section>

    <section v-if="!readOnly" class="panel access">
      <div class="panel-head"><div><p>Least privilege</p><h2>Controller grants</h2></div><span>{{ machine.enabled ? 'Enabled' : 'Disabled' }}</span></div>
      <label v-for="capability in knownCapabilities" :key="capability"><input type="checkbox" :checked="grants.includes(capability)" :disabled="capability === 'inventory.read' || !machine.capabilities.includes(capability)" @change="toggle(capability)"><span><strong>{{ capability }}</strong><small>{{ capability === 'inventory.read' ? 'Required bounded identity and inventory.' : capability === 'project.operate' ? 'Typed project lifecycle operations.' : 'Typed worktree-environment lifecycle operations.' }}</small></span></label>
      <label class="confirm"><input v-model="reviewed" type="checkbox"><span>I reviewed the peer-declared capabilities and this complete grant set.</span></label>
      <div class="actions"><button type="button" :disabled="pending || !reviewed" @click="saveAccess">Save reviewed grants</button><button type="button" class="danger" :disabled="pending" @click="emit('access', { enabled: false, grantedCapabilities: [], confirmRisk: true })">Disable and revoke</button><button type="button" class="danger" :disabled="pending" @click="emit('remove')">Remove locally</button></div>
    </section>

    <section v-if="!readOnly && snapshot?.projects.length && machine.grantedCapabilities.includes('project.operate')" class="panel operate">
      <div class="panel-head"><div><p>Audited mutation</p><h2>Remote lifecycle operation</h2></div></div>
      <div class="operation-fields"><label><span>Project</span><select v-model="projectId" :disabled="Boolean(environmentId)"><option v-for="project in snapshot.projects" :key="project.id" :value="project.id">{{ project.displayName }}</option></select></label><label><span>Environment (optional)</span><select v-model="environmentId"><option value="">Primary checkout</option><option v-for="environment in snapshot.environments" :key="environment.id" :value="environment.id">{{ environment.name }} · {{ environment.branch || 'detached' }}</option></select></label><label><span>Action</span><select v-model="action"><option value="start">Start</option><option value="stop">Stop</option><option value="restart">Restart</option><option value="rebuild">Rebuild</option></select></label></div>
      <label class="confirm"><input v-model="runReviewed" type="checkbox"><span>I reviewed the target machine, project, environment, and lifecycle impact.</span></label>
      <button type="button" :disabled="pending || !runReviewed || !projectId" @click="run">Submit typed {{ action }}</button>
    </section>
  </main>
</template>

<style scoped>
.detail{display:grid;gap:14px;min-width:0}.panel{padding:18px;border:1px solid var(--border);border-radius:14px;background:var(--panel)}.panel-head,.actions{display:flex;align-items:center;justify-content:space-between;gap:12px}.panel-head p{margin:0 0 4px;color:var(--accent);font-size:10px;font-weight:800;letter-spacing:.12em;text-transform:uppercase}.panel-head h2{margin:0;font-size:17px}.panel-head time,.panel-head>span{color:var(--soft);font-size:11px}.state{padding:4px 8px;border:1px solid var(--border);border-radius:99px;text-transform:capitalize}.state--online{color:var(--green)}.state--degraded,.state--pending{color:var(--yellow)}.state--offline{color:var(--red)}dl{display:grid;grid-template-columns:1fr 1fr;gap:9px;margin:15px 0}dl div{display:grid;gap:4px;padding:10px;border:1px solid var(--border);border-radius:9px;background:var(--panel-2)}.fingerprint{grid-column:1/-1}dt,.operation-fields span{color:var(--soft);font-size:9px;text-transform:uppercase;letter-spacing:.08em}dd{margin:0;overflow-wrap:anywhere}.projects{display:grid;gap:7px;margin-top:14px}.projects article{display:grid;grid-template-columns:minmax(0,1fr) 120px 90px;align-items:center;gap:10px;padding:11px;border:1px solid var(--border);border-radius:9px;background:var(--panel-2)}.projects article.degraded{border-color:rgba(255,115,115,.35)}.projects div{display:grid;gap:3px}.projects small,.projects span,.projects i,.empty{color:var(--soft);font-size:11px}.projects i{font-style:normal}.access>label,.confirm{display:flex;align-items:flex-start;gap:9px;margin:12px 0}.access label span{display:grid;gap:3px}.access label small,.confirm{color:var(--muted)}.actions{justify-content:flex-start}.danger{color:var(--red)}.error{padding:10px;border:1px solid rgba(255,115,115,.3);border-radius:8px;color:var(--red);background:rgba(255,115,115,.06)}.operation-fields{display:grid;grid-template-columns:1fr 1fr 150px;gap:10px;margin-top:14px}.operation-fields label{display:grid;gap:5px}.operation-fields select,button{padding:8px 10px;border:1px solid var(--border);border-radius:8px;background:var(--panel-2);color:var(--text)}button:disabled{opacity:.5}@media(max-width:760px){dl,.operation-fields{grid-template-columns:1fr}.fingerprint{grid-column:auto}.projects article{grid-template-columns:1fr auto}.projects i{grid-column:1/-1}.actions{align-items:stretch;flex-direction:column}}
</style>
