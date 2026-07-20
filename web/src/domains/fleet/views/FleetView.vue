<script setup lang="ts">
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { computed, ref, watch } from 'vue'

import type {
  Machine,
  MachineAccessRequest,
  MachineRegistrationRequest,
  RemoteOperationRequest,
} from '../../../api/generated/types.gen'
import {
  loadMachineSnapshot,
  loadMachines,
  refreshMachine,
  registerMachine,
  removeMachine,
  runMachineOperation,
  saveMachineAccess,
} from '../api'
import MachineDetailPanel from '../components/MachineDetailPanel.vue'
import MachineRegistrationPanel from '../components/MachineRegistrationPanel.vue'

withDefaults(defineProps<{ readOnly?: boolean }>(), { readOnly: false })
const queryClient = useQueryClient()
const selectedId = ref('')
const notice = ref('')
const machines = useQuery({
  queryKey: ['machines'],
  queryFn: loadMachines,
  refetchInterval: 15_000,
})
const selected = computed(() => machines.data.value?.find((item) => item.id === selectedId.value))
const snapshot = useQuery({
  queryKey: computed(() => ['machine-snapshot', selectedId.value]),
  queryFn: () => loadMachineSnapshot(selectedId.value),
  enabled: computed(() => Boolean(selectedId.value && selected.value?.enabled)),
  refetchInterval: 20_000,
})

watch(
  () => machines.data.value,
  (items) => {
    const first = items?.[0]
    if (first && !items.some((item) => item.id === selectedId.value)) selectedId.value = first.id
  },
  { immediate: true },
)

function replaceMachine(machine: Machine) {
  queryClient.setQueryData<Array<Machine>>(['machines'], (current = []) =>
    current.map((item) => (item.id === machine.id ? machine : item)),
  )
}
const create = useMutation({
  mutationFn: registerMachine,
  onSuccess: (machine) => {
    queryClient.setQueryData<Array<Machine>>(['machines'], (current = []) => [...current, machine])
    selectedId.value = machine.id
  },
})
const probe = useMutation({ mutationFn: refreshMachine, onSuccess: replaceMachine })
const access = useMutation({
  mutationFn: ({ id, request }: { id: string; request: MachineAccessRequest }) =>
    saveMachineAccess(id, request),
  onSuccess: replaceMachine,
})
const remove = useMutation({
  mutationFn: removeMachine,
  onSuccess: (_, id) => {
    queryClient.setQueryData<Array<Machine>>(['machines'], (current = []) =>
      current.filter((item) => item.id !== id),
    )
    selectedId.value = ''
  },
})
const operate = useMutation({
  mutationFn: ({ id, request }: { id: string; request: RemoteOperationRequest }) =>
    runMachineOperation(id, request),
  onSuccess: (receipt) => {
    notice.value = `Remote operation ${receipt.operationId} accepted as ${receipt.state}.`
  },
})
const pending = computed(
  () =>
    create.isPending.value ||
    probe.isPending.value ||
    access.isPending.value ||
    remove.isPending.value ||
    operate.isPending.value,
)
const mutationError = computed(
  () =>
    create.error.value ||
    probe.error.value ||
    access.error.value ||
    remove.error.value ||
    operate.error.value,
)
</script>

<template>
  <section class="fleet-view" aria-labelledby="fleet-title">
    <header class="page-head">
      <div>
        <p>{{ readOnly ? 'Responsive companion' : 'Optional peer federation' }}</p>
        <h1 id="fleet-title">{{ readOnly ? 'Switchyard companion' : 'Machines' }}</h1>
        <span>{{
          readOnly
            ? 'Authenticated read-only project health across explicitly configured machines.'
            : 'Direct authenticated peers with narrow inventory and reviewed lifecycle grants.'
        }}</span>
      </div>
      <RouterLink v-if="readOnly" to="/fleet">Manage on desktop</RouterLink>
    </header>
    <aside class="local-first">
      <strong>Local-only remains the default.</strong
      ><span
        >No account or hosted service is required. Tunnels provide transport; Switchyard still
        enforces mTLS identity, capability grants, confirmation, and audit.</span
      >
    </aside>
    <MachineRegistrationPanel
      v-if="!readOnly"
      :pending="pending"
      @submit="(request: MachineRegistrationRequest) => create.mutate(request)"
    />
    <p v-if="machines.isError.value" class="state error" role="alert">
      Remote machine inventory is unavailable.
      <button type="button" @click="machines.refetch()">Retry</button>
    </p>
    <p v-else-if="machines.isPending.value" class="state" aria-live="polite">
      Loading configured remote machines…
    </p>
    <div v-else-if="machines.data.value?.length" class="layout">
      <nav class="machine-list" aria-label="Remote machines">
        <button
          v-for="machine in machines.data.value"
          :key="machine.id"
          type="button"
          :class="{ active: machine.id === selectedId }"
          @click="selectedId = machine.id"
        >
          <i :class="`dot dot--${machine.state}`" aria-hidden="true"></i
          ><span
            ><strong>{{ machine.name }}</strong
            ><small>{{ machine.os || 'unobserved' }} · {{ machine.state }}</small></span
          >
        </button>
      </nav>
      <div>
        <p v-if="snapshot.isError.value" class="state error" role="alert">
          The selected peer inventory is unavailable.
          <button type="button" @click="snapshot.refetch()">Retry</button>
        </p>
        <MachineDetailPanel
          v-if="selected"
          :machine="selected"
          :snapshot="snapshot.data.value"
          :pending="pending"
          :read-only="readOnly"
          @probe="probe.mutate(selectedId)"
          @refresh-snapshot="snapshot.refetch()"
          @access="(request) => access.mutate({ id: selectedId, request })"
          @remove="remove.mutate(selectedId)"
          @run="(request) => operate.mutate({ id: selectedId, request })"
        />
        <p v-if="notice" class="state success" role="status">{{ notice }}</p>
        <p v-if="mutationError" class="state error" role="alert">{{ mutationError.message }}</p>
      </div>
    </div>
    <section v-else class="empty">
      <span aria-hidden="true">⌁</span>
      <h2>No remote machines configured</h2>
      <p>
        Switchyard is fully functional in local-only mode. Add a peer only when you need direct
        inventory or typed operations across machines.
      </p>
    </section>
  </section>
</template>

<style scoped>
.fleet-view {
  max-width: 1450px;
  margin: 0 auto;
  padding: 28px;
}
.page-head {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 16px;
}
.page-head p {
  margin: 0 0 5px;
  color: var(--accent);
  font-size: 10px;
  font-weight: 800;
  letter-spacing: 0.13em;
  text-transform: uppercase;
}
.page-head h1 {
  margin: 0 0 5px;
  font-size: 32px;
}
.page-head span,
.local-first span,
.empty p {
  color: var(--muted);
}
.page-head a {
  padding: 8px 11px;
  border: 1px solid var(--border);
  border-radius: 8px;
  color: var(--text);
  text-decoration: none;
}
.local-first {
  display: flex;
  gap: 8px;
  margin-bottom: 16px;
  padding: 12px 14px;
  border: 1px solid rgba(84, 212, 154, 0.25);
  border-radius: 11px;
  background: rgba(84, 212, 154, 0.05);
}
.layout {
  display: grid;
  grid-template-columns: 250px minmax(0, 1fr);
  gap: 16px;
}
.machine-list {
  display: grid;
  align-content: start;
  gap: 6px;
  position: sticky;
  top: 92px;
}
.machine-list button {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 12px;
  border: 1px solid var(--border);
  border-radius: 11px;
  background: var(--panel);
  color: var(--text);
  text-align: left;
}
.machine-list button.active {
  border-color: rgba(120, 166, 255, 0.5);
  background: rgba(120, 166, 255, 0.1);
  box-shadow: inset 2px 0 var(--accent);
}
.machine-list span {
  display: grid;
  gap: 3px;
}
.machine-list small {
  color: var(--soft);
}
.dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--soft);
}
.dot--online {
  background: var(--green);
  box-shadow: 0 0 10px rgba(84, 212, 154, 0.5);
}
.dot--degraded,
.dot--pending {
  background: var(--yellow);
}
.dot--offline {
  background: var(--red);
}
.state,
.empty {
  margin: 0 0 14px;
  padding: 20px;
  border: 1px solid var(--border);
  border-radius: 14px;
  background: var(--panel);
}
.error {
  color: var(--red);
}
.success {
  margin-top: 14px;
  color: var(--green);
}
.state button {
  padding: 5px 8px;
  border: 1px solid var(--border);
  border-radius: 7px;
  background: var(--panel-2);
  color: var(--text);
}
.empty {
  display: grid;
  justify-items: center;
  gap: 8px;
  padding: 60px;
  text-align: center;
}
.empty h2,
.empty p {
  margin: 0;
}
.empty p {
  max-width: 620px;
  line-height: 1.6;
}
@media (max-width: 850px) {
  .layout {
    grid-template-columns: 1fr;
  }
  .machine-list {
    position: static;
    grid-template-columns: repeat(auto-fit, minmax(190px, 1fr));
  }
  .page-head,
  .local-first {
    align-items: flex-start;
    flex-direction: column;
  }
}
</style>
