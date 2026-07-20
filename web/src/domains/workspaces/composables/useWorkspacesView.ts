import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { computed, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import type { WorkspaceDefinition, WorkspaceFailurePolicy } from '../../../api/generated/types.gen'
import { loadAllEnvironments } from '../../environments/api'
import { trackOperation } from '../../operations/store'
import { loadProjects } from '../../projects/api'
import { loadWorkspace, loadWorkspaces, runWorkspace, saveWorkspace } from '../api'

const activeMemberStates = new Set([
  'starting',
  'checking_health',
  'running',
  'stopping',
  'stop_failed',
  'rolling_back',
  'rollback_failed',
])

export function useWorkspacesView() {
  const route = useRoute()
  const router = useRouter()
  const queryClient = useQueryClient()
  const creating = ref(false)
  const selectedId = ref(typeof route.query.workspace === 'string' ? route.query.workspace : '')
  const profileId = ref('')
  const policy = ref<WorkspaceFailurePolicy>('rollback')
  const workspaces = useQuery({
    queryKey: ['workspaces'],
    queryFn: loadWorkspaces,
    refetchInterval: 5_000,
  })
  const projects = useQuery({ queryKey: ['projects'], queryFn: loadProjects })
  const environments = useQuery({
    queryKey: computed(() => [
      'environments',
      ...(projects.data.value?.map((project) => project.id) ?? []),
    ]),
    queryFn: () => loadAllEnvironments(projects.data.value ?? []),
    enabled: computed(() => Boolean(projects.data.value)),
  })
  const selected = useQuery({
    queryKey: computed(() => ['workspace', selectedId.value]),
    queryFn: () => loadWorkspace(selectedId.value),
    enabled: computed(() => selectedId.value !== ''),
    refetchInterval: 2_000,
  })
  function selectWorkspace(id: string): void {
    selectedId.value = id
    void router.replace({ query: { ...route.query, workspace: id } })
  }
  const createMutation = useMutation({
    mutationFn: saveWorkspace,
    onSuccess: async (workspace) => {
      creating.value = false
      await queryClient.invalidateQueries({ queryKey: ['workspaces'] })
      selectWorkspace(workspace.id)
    },
  })
  const operationMutation = useMutation({
    mutationFn: ({
      action,
      runRecipes = false,
    }: {
      action: 'start' | 'stop'
      runRecipes?: boolean
    }) =>
      runWorkspace(selectedId.value, {
        action,
        profileId: profileId.value || undefined,
        policy: policy.value,
        runRecipes,
      }),
    onSuccess: async (operation) => {
      trackOperation(operation)
      await queryClient.invalidateQueries({ queryKey: ['workspace', selectedId.value] })
    },
  })
  const current = computed(() => selected.data.value)
  const workspaceActive = computed(() => {
    const workspace = current.value
    return Boolean(
      workspace &&
      (workspace.lastRun?.state === 'running' ||
        workspace.members.some((member) => activeMemberStates.has(member.status))),
    )
  })
  const workspaceCount = computed(() => workspaces.data.value?.length ?? 0)
  const memberOptions = computed(() => [
    ...(projects.data.value ?? []).map((project) => ({
      id: project.id,
      name: project.displayName,
      location: project.primaryLocation,
      environment: false,
    })),
    ...(environments.data.value ?? []).map((environment) => ({
      id: environment.id,
      name: environment.name,
      location: environment.path,
      environment: true,
    })),
  ])
  const memberNames = computed(() =>
    Object.fromEntries(memberOptions.value.map((item) => [item.id, item.name])),
  )
  watch(
    () => workspaces.data.value,
    (items) => {
      if (!selectedId.value && items?.length) selectWorkspace(items[0]!.id)
    },
    { immediate: true },
  )
  watch(current, (workspace) => {
    if (!workspace) return
    profileId.value = workspace.profile ?? workspace.profiles[0]?.id ?? ''
    policy.value = workspace.policy
  })
  function create(definition: WorkspaceDefinition): void {
    createMutation.mutate(definition)
  }
  return {
    creating,
    selectedId,
    profileId,
    policy,
    workspaces,
    selected,
    createMutation,
    operationMutation,
    current,
    workspaceActive,
    workspaceCount,
    memberOptions,
    memberNames,
    selectWorkspace,
    create,
  }
}
