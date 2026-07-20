import { computed, ref, watch } from 'vue'

import type {
  FleetCapability,
  FleetSnapshot,
  Machine,
  MachineAccessRequest,
  RemoteOperationRequest,
} from '../../../api/generated/types.gen'

export interface MachineDetailProps {
  machine: Machine
  snapshot?: FleetSnapshot
  pending: boolean
  readOnly: boolean
}

interface MachineDetailEmit {
  (event: 'access', request: MachineAccessRequest): void
  (event: 'run', request: RemoteOperationRequest): void
}

export function useMachineDetail(props: MachineDetailProps, emit: MachineDetailEmit) {
  const knownCapabilities: Array<FleetCapability> = [
    'inventory.read',
    'project.operate',
    'environment.manage',
  ]
  const grants = ref<Array<FleetCapability>>([])
  const reviewed = ref(false)
  const projectId = ref('')
  const environmentId = ref('')
  const action = ref<RemoteOperationRequest['action']>('start')
  const runReviewed = ref(false)
  watch(
    () => props.machine,
    (machine) => {
      grants.value = [...machine.grantedCapabilities]
      reviewed.value = false
    },
    { immediate: true },
  )
  watch(
    () => props.snapshot,
    (snapshot) => {
      if (!snapshot) return
      if (!snapshot.projects.some((project) => project.id === projectId.value)) {
        projectId.value = snapshot.projects[0]?.id ?? ''
      }
    },
    { immediate: true },
  )
  const selectedEnvironment = computed(() =>
    props.snapshot?.environments.find((item) => item.id === environmentId.value),
  )
  function toggle(capability: FleetCapability) {
    if (capability === 'inventory.read') return
    grants.value = grants.value.includes(capability)
      ? grants.value.filter((item) => item !== capability)
      : [...grants.value, capability]
    reviewed.value = false
  }
  function saveAccess() {
    emit('access', {
      enabled: true,
      grantedCapabilities: grants.value,
      confirmRisk: reviewed.value,
    })
  }
  function run() {
    const targetProject = selectedEnvironment.value?.projectId ?? projectId.value
    emit('run', {
      requestId: `ui_${window.crypto.randomUUID()}`,
      projectId: targetProject,
      environmentId: selectedEnvironment.value?.id,
      action: action.value,
      confirmRisk: runReviewed.value,
    })
    runReviewed.value = false
  }
  return {
    knownCapabilities,
    grants,
    reviewed,
    projectId,
    environmentId,
    action,
    runReviewed,
    selectedEnvironment,
    toggle,
    saveAccess,
    run,
  }
}
