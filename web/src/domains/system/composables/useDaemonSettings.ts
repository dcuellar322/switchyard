import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { computed, ref, watch } from 'vue'

import type { DaemonSettings } from '../../../api/generated/types.gen'
import { loadDaemonSettings, saveDaemonSettings } from '../settingsApi'

function clone(settings: DaemonSettings): DaemonSettings {
  return JSON.parse(JSON.stringify(settings)) as DaemonSettings
}

export function useDaemonSettings() {
  const queryClient = useQueryClient()
  const query = useQuery({ queryKey: ['daemon-settings'], queryFn: loadDaemonSettings })
  const draft = ref<DaemonSettings>()
  const baseline = ref('')

  watch(
    () => query.data.value?.settings,
    (settings) => {
      if (!settings) return
      draft.value = clone(settings)
      baseline.value = JSON.stringify(settings)
    },
    { immediate: true },
  )

  watch(
    () => draft.value?.appearance,
    (appearance) => {
      if (!appearance) return
      document.documentElement.dataset.compact = appearance.density === 'compact' ? 'true' : 'false'
      document.documentElement.dataset.timeDisplay = appearance.timeDisplay
      document.documentElement.dataset.theme = appearance.theme
    },
    { deep: true, immediate: true },
  )

  const save = useMutation({
    mutationFn: async () => {
      if (!draft.value) throw new Error('Settings have not loaded.')
      return saveDaemonSettings(clone(draft.value))
    },
    onSuccess: (status) => {
      queryClient.setQueryData(['daemon-settings'], status)
      draft.value = clone(status.settings)
      baseline.value = JSON.stringify(status.settings)
    },
  })

  const dirty = computed(() => Boolean(draft.value) && JSON.stringify(draft.value) !== baseline.value)

  function reset() {
    if (query.data.value) {
      draft.value = clone(query.data.value.settings)
    }
  }

  return { query, draft, dirty, save, reset }
}
