import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { computed, ref, watch } from 'vue'

import type { PluginEnableRequest } from '../../../api/generated/types.gen'
import {
  activatePlugin,
  approvePlugin,
  deactivatePlugin,
  discoverPlugins,
  loadPluginLogs,
  loadPlugins,
  probePlugin,
} from '../api'

type Scope = PluginEnableRequest['grantedScopes'][number]

export function usePluginsView() {
  const queryClient = useQueryClient()
  const selectedId = ref('')
  const reviewed = ref(false)
  const grants = ref<Array<Scope>>([])
  const plugins = useQuery({ queryKey: ['plugins'], queryFn: loadPlugins, refetchInterval: 15_000 })
  const selected = computed(() => plugins.data.value?.find((item) => item.id === selectedId.value))
  const logs = useQuery({
    queryKey: computed(() => ['plugin-logs', selectedId.value]),
    queryFn: () => loadPluginLogs(selectedId.value),
    enabled: computed(() => Boolean(selectedId.value)),
    refetchInterval: 10_000,
  })

  watch(
    () => plugins.data.value,
    (items) => {
      const first = items?.[0]
      if (first && !items.some((item) => item.id === selectedId.value)) selectedId.value = first.id
    },
    { immediate: true },
  )
  watch(
    selected,
    (item) => {
      grants.value = item ? [...item.grantedScopes] : []
      reviewed.value = false
    },
    { immediate: true },
  )

  function updateSelected(item: NonNullable<typeof selected.value>) {
    queryClient.setQueryData<Array<typeof item>>(['plugins'], (current = []) =>
      current.map((value) => (value.id === item.id ? item : value)),
    )
  }
  const refresh = useMutation({
    mutationFn: discoverPlugins,
    onSuccess: (items) => queryClient.setQueryData(['plugins'], items),
  })
  const trust = useMutation({
    mutationFn: () => approvePlugin(selectedId.value, selected.value?.fingerprint ?? ''),
    onSuccess: updateSelected,
  })
  const enable = useMutation({
    mutationFn: () => activatePlugin(selectedId.value, grants.value),
    onSuccess: updateSelected,
  })
  const disable = useMutation({
    mutationFn: () => deactivatePlugin(selectedId.value),
    onSuccess: updateSelected,
  })
  const health = useMutation({
    mutationFn: () => probePlugin(selectedId.value),
    onSuccess: updateSelected,
  })

  function toggleScope(scope: Scope) {
    grants.value = grants.value.includes(scope)
      ? grants.value.filter((item) => item !== scope)
      : [...grants.value, scope]
  }
  const pending = computed(() =>
    [refresh, trust, enable, disable, health].some((mutation) => mutation.isPending.value),
  )
  const mutationError = computed(
    () =>
      refresh.error.value ||
      trust.error.value ||
      enable.error.value ||
      disable.error.value ||
      health.error.value,
  )
  return {
    selectedId,
    reviewed,
    grants,
    plugins,
    selected,
    logs,
    refresh,
    trust,
    enable,
    disable,
    health,
    toggleScope,
    pending,
    mutationError,
  }
}
