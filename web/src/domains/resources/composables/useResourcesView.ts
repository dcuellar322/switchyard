import { useQuery } from '@tanstack/vue-query'
import { computed, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import {
  loadCleanupPreview,
  loadMetricHistory,
  loadResourceOverview,
  loadStorageInventory,
} from '../api'

export function useResourcesView() {
  const overview = useQuery({
    queryKey: ['resource-overview'],
    queryFn: loadResourceOverview,
    refetchInterval: 10_000,
  })
  const storage = useQuery({
    queryKey: ['storage-inventory'],
    queryFn: loadStorageInventory,
    refetchInterval: 120_000,
  })
  const route = useRoute()
  const router = useRouter()
  const routeProject = computed(() =>
    typeof route.query.project === 'string' ? route.query.project : '',
  )
  const selectedProject = ref(routeProject.value)
  const selectedService = ref('')
  const range = ref<'1h' | '24h' | '7d'>('1h')
  const cleanup = ref<Awaited<ReturnType<typeof loadCleanupPreview>>>()
  const cleanupPending = ref(false)
  const cleanupError = ref('')

  function selectConsumer(projectId: string, serviceId: string) {
    if (selectedProject.value !== projectId) {
      cleanup.value = undefined
      cleanupError.value = ''
    }
    selectedProject.value = projectId
    selectedService.value = serviceId
    if (routeProject.value !== projectId) {
      void router.replace({ query: { ...route.query, project: projectId || undefined } })
    }
  }
  watch(
    () => overview.data.value?.projects,
    (projects) => {
      if (!projects) return
      const firstProject = projects[0]
      if (!selectedProject.value && firstProject) selectedProject.value = firstProject.projectId
      if (
        selectedProject.value &&
        !projects.some((item) => item.projectId === selectedProject.value)
      ) {
        selectConsumer(firstProject?.projectId ?? '', '')
      }
    },
    { immediate: true },
  )
  watch(routeProject, (projectId) => {
    if (!projectId || projectId === selectedProject.value) return
    selectedProject.value = projectId
    selectedService.value = ''
  })
  const history = useQuery({
    queryKey: computed(() => [
      'resource-history',
      selectedProject.value,
      selectedService.value,
      range.value,
    ]),
    queryFn: () => loadMetricHistory(selectedProject.value, selectedService.value, range.value),
    enabled: computed(() => Boolean(selectedProject.value)),
    refetchInterval: 30_000,
  })
  const projects = computed(() => overview.data.value?.projects ?? [])
  const totals = computed(() =>
    projects.value.reduce(
      (result, project) => ({
        cpu: result.cpu + (project.metric.cpuAvailable ? project.metric.cpuPercent : 0),
        memory: result.memory + (project.metric.memoryAvailable ? project.metric.memoryBytes : 0),
        processes: result.processes + project.metric.processCount,
        unavailable:
          result.unavailable +
          (!project.metric.cpuAvailable || !project.metric.memoryAvailable ? 1 : 0),
      }),
      { cpu: 0, memory: 0, processes: 0, unavailable: 0 },
    ),
  )
  const stale = computed(() =>
    overview.data.value
      ? Date.now() - new Date(overview.data.value.observedAt).getTime() > 30_000
      : false,
  )
  const footprintBytes = computed(() => {
    const value = overview.data.value?.footprint
    return value
      ? value.databaseBytes + value.databaseWalBytes + value.databaseShmBytes + value.logBytes
      : 0
  })
  async function previewCleanup(projectId: string) {
    cleanupPending.value = true
    cleanupError.value = ''
    try {
      cleanup.value = await loadCleanupPreview(projectId)
    } catch (cause) {
      cleanupError.value = cause instanceof Error ? cause.message : 'Cleanup preview failed.'
    } finally {
      cleanupPending.value = false
    }
  }
  async function refreshResources() {
    await Promise.all([overview.refetch(), storage.refetch()])
  }
  return {
    overview,
    storage,
    selectedProject,
    selectedService,
    range,
    cleanup,
    cleanupPending,
    cleanupError,
    history,
    projects,
    totals,
    stale,
    footprintBytes,
    selectConsumer,
    previewCleanup,
    refreshResources,
  }
}
