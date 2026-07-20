import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { computed, ref } from 'vue'

import type { TeamBundle, TeamPublisher } from '../../../api/generated/types.gen'
import {
  installBundle,
  loadCuratedPlugins,
  loadEffectivePolicy,
  loadTeamBundles,
  loadTeamPublishers,
  trustPublisher,
} from '../api'

export function useTeamView() {
  const queryClient = useQueryClient()
  const publishers = useQuery({ queryKey: ['team-publishers'], queryFn: loadTeamPublishers })
  const bundles = useQuery({ queryKey: ['team-bundles'], queryFn: loadTeamBundles })
  const policy = useQuery({ queryKey: ['team-policy'], queryFn: loadEffectivePolicy })
  const registry = useQuery({ queryKey: ['curated-plugins'], queryFn: loadCuratedPlugins })
  const publisherName = ref('')
  const publicKey = ref('')
  const publisherReviewed = ref(false)
  const selectedBundle = ref<TeamBundle>()
  const bundleReviewed = ref(false)
  const fileError = ref('')
  const trust = useMutation({
    mutationFn: () => trustPublisher(publisherName.value, publicKey.value),
    onSuccess: (value) => {
      queryClient.setQueryData<Array<TeamPublisher>>(['team-publishers'], (current = []) => [
        ...current.filter((item) => item.id !== value.id),
        value,
      ])
      publisherName.value = ''
      publicKey.value = ''
      publisherReviewed.value = false
    },
  })
  const install = useMutation({
    mutationFn: () => installBundle(selectedBundle.value as TeamBundle),
    onSuccess: async () => {
      selectedBundle.value = undefined
      bundleReviewed.value = false
      await Promise.all([bundles.refetch(), policy.refetch(), registry.refetch()])
    },
  })
  const error = computed(
    () =>
      publishers.error.value ||
      bundles.error.value ||
      policy.error.value ||
      registry.error.value ||
      trust.error.value ||
      install.error.value,
  )
  const loading = computed(() =>
    [publishers, bundles, policy, registry].some((query) => query.isPending.value),
  )
  const emptyConfiguration = computed(
    () =>
      !loading.value &&
      (publishers.data.value?.length ?? 0) === 0 &&
      (bundles.data.value?.length ?? 0) === 0 &&
      (policy.data.value?.sourceBundleIds.length ?? 0) === 0 &&
      (registry.data.value?.length ?? 0) === 0,
  )

  async function selectBundle(event: unknown) {
    fileError.value = ''
    selectedBundle.value = undefined
    const file = (
      event as {
        target?: { files?: ArrayLike<{ size: number; text(): Promise<string> }> }
      }
    ).target?.files?.[0]
    if (!file) return
    if (file.size > 2 * 1024 * 1024) {
      fileError.value = 'Bundle files must be 2 MiB or smaller.'
      return
    }
    try {
      selectedBundle.value = JSON.parse(await file.text()) as TeamBundle
    } catch {
      fileError.value = 'The selected file is not a JSON bundle.'
    }
  }
  return {
    publishers,
    bundles,
    policy,
    registry,
    publisherName,
    publicKey,
    publisherReviewed,
    selectedBundle,
    bundleReviewed,
    fileError,
    trust,
    install,
    error,
    loading,
    emptyConfiguration,
    selectBundle,
  }
}
