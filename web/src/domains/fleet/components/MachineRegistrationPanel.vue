<script setup lang="ts">
import { reactive, ref } from 'vue'

import type { FleetCapability, MachineRegistrationRequest } from '../../../api/generated/types.gen'

defineProps<{ pending: boolean }>()
const emit = defineEmits<{ submit: [request: MachineRegistrationRequest] }>()
const open = ref(false)
const reviewed = ref(false)
const form = reactive({ name: '', endpoint: '', fingerprint: '', ca: '', certificate: '', key: '' })

function submit() {
  const grants: Array<FleetCapability> = ['inventory.read']
  emit('submit', {
    name: form.name,
    endpoint: form.endpoint,
    certificateFingerprint: form.fingerprint,
    caCertificatePath: form.ca,
    clientCertificatePath: form.certificate,
    clientKeyPath: form.key,
    grantedCapabilities: grants,
    confirmRisk: reviewed.value,
  })
}
</script>

<template>
  <section class="registration panel">
    <button v-if="!open" type="button" @click="open = true">Add remote machine</button>
    <form v-else @submit.prevent="submit">
      <div class="panel-head">
        <div>
          <p>Explicit trust</p>
          <h2>Register an mTLS peer</h2>
        </div>
        <button type="button" @click="open = false">Cancel</button>
      </div>
      <p class="notice">
        Switchyard stores certificate file references locally. Private keys, repository paths, logs,
        and secrets never enter remote inventory.
      </p>
      <div class="fields">
        <label
          ><span>Name</span><input v-model="form.name" required maxlength="128" autocomplete="off"
        /></label>
        <label
          ><span>HTTPS endpoint</span
          ><input
            v-model="form.endpoint"
            required
            type="url"
            placeholder="https://127.0.0.1:19618"
            autocomplete="off"
        /></label>
        <label class="wide"
          ><span>Server SHA-256 fingerprint</span
          ><input v-model="form.fingerprint" required minlength="64" autocomplete="off"
        /></label>
        <label
          ><span>Peer CA path</span
          ><input v-model="form.ca" required placeholder="/absolute/path/ca.pem" autocomplete="off"
        /></label>
        <label
          ><span>Client certificate path</span
          ><input
            v-model="form.certificate"
            required
            placeholder="/absolute/path/client.pem"
            autocomplete="off"
        /></label>
        <label class="wide"
          ><span>Client private-key path</span
          ><input
            v-model="form.key"
            required
            placeholder="/absolute/path/client-key.pem"
            autocomplete="off"
        /></label>
      </div>
      <label class="confirm"
        ><input v-model="reviewed" type="checkbox" /><span
          >I reviewed the endpoint, exact certificate pin, credential references, and initial
          inventory-only grant.</span
        ></label
      >
      <button type="submit" :disabled="pending || !reviewed">
        {{ pending ? 'Authenticating…' : 'Register and probe' }}
      </button>
    </form>
  </section>
</template>

<style scoped>
.panel {
  padding: 16px;
  border: 1px solid var(--border);
  border-radius: 14px;
  background: var(--panel);
}
.registration {
  margin-bottom: 16px;
}
.panel-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}
.panel-head p {
  margin: 0 0 4px;
  color: var(--accent);
  font-size: 10px;
  font-weight: 800;
  letter-spacing: 0.12em;
  text-transform: uppercase;
}
.panel-head h2 {
  margin: 0;
  font-size: 17px;
}
.notice {
  padding: 10px;
  border-left: 2px solid var(--yellow);
  background: rgba(241, 199, 91, 0.05);
  color: var(--muted);
  line-height: 1.5;
}
.fields {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 10px;
}
.fields label {
  display: grid;
  gap: 5px;
}
.fields span {
  color: var(--soft);
  font-size: 10px;
}
.wide {
  grid-column: 1/-1;
}
.fields input {
  width: 100%;
  padding: 9px;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--panel-2);
  color: var(--text);
}
.confirm {
  display: flex;
  align-items: flex-start;
  gap: 9px;
  margin: 14px 0;
  color: var(--muted);
  line-height: 1.4;
}
button {
  padding: 8px 11px;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--panel-2);
  color: var(--text);
}
button:disabled {
  opacity: 0.5;
}
@media (max-width: 700px) {
  .fields {
    grid-template-columns: 1fr;
  }
  .wide {
    grid-column: auto;
  }
}
</style>
