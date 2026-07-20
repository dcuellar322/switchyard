import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { computed, ref, watch } from 'vue'

import { loadProjects } from '../../projects/api'
import { loadDaemonSettings } from '../../system/settingsApi'
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

export function useDiagnosticsView() {
  const queryClient = useQueryClient()
  const selectedProject = ref('')
  const selectedProvider = ref('')
  const recipeName = ref('Respond to repeated crashes')
  const recipeTrigger = ref<
    'REPEATED_CRASH' | 'PORT_CONFLICT' | 'RESOURCE_PRESSURE' | 'UNHEALTHY_DEPENDENCY'
  >('REPEATED_CRASH')
  const recipeAction = ref('')
  const notice = ref('')
  const projects = useQuery({ queryKey: ['projects'], queryFn: loadProjects })
  const providers = useQuery({ queryKey: ['ai-providers'], queryFn: loadProviders })
  const settings = useQuery({ queryKey: ['daemon-settings'], queryFn: loadDaemonSettings })
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
  watch(
    () => projects.data.value,
    (values) => {
      if (!selectedProject.value && values?.length) selectedProject.value = values[0]!.id
    },
    { immediate: true },
  )
  const availableProviders = computed(
    () => providers.data.value?.filter((item) => item.available) ?? [],
  )
  const defaultProviderApplied = ref(false)
  watch(
    [availableProviders, () => settings.data.value],
    ([values, status]) => {
      if (defaultProviderApplied.value || !providers.data.value || !status) return
      const preferred = status.settings.ai.defaultProvider
      if (preferred !== 'none' && values.some((provider) => provider.id === preferred)) {
        selectedProvider.value = preferred
      }
      defaultProviderApplied.value = true
    },
    { immediate: true },
  )
  const suggestedActions = computed(() => {
    const values = diagnosis.data.value?.hypotheses.flatMap((item) => item.suggestedActions) ?? []
    return [...new Map(values.map((item) => [item.actionId, item])).values()]
  })
  watch(
    suggestedActions,
    (values) => {
      if (!recipeAction.value && values.length) recipeAction.value = values[0]!.actionId
    },
    { immediate: true },
  )
  const run = useMutation({
    mutationFn: () => diagnoseProject(selectedProject.value, selectedProvider.value || undefined),
    onSuccess: (value) => {
      queryClient.setQueryData(['diagnosis', selectedProject.value], value)
      void queryClient.invalidateQueries({
        queryKey: ['diagnostic-notifications', selectedProject.value],
      })
      notice.value = 'Fresh bounded evidence collected. Deterministic rules ran before optional AI.'
    },
  })
  const feedback = useMutation({
    mutationFn: (input: { hypothesisId: string; verdict: 'accurate' | 'false_positive' }) =>
      reviewHypothesis(diagnosis.data.value!.id, input.hypothesisId, input.verdict),
    onSuccess: () => {
      notice.value = 'Feedback stored locally. Nothing was sent as telemetry.'
    },
  })
  const action = useMutation({
    mutationFn: (actionId: string) => runSuggestedAction(diagnosis.data.value!.id, actionId),
    onSuccess: (value) => {
      notice.value = `Approved action queued as ${value.id}.`
    },
  })
  const createRecipe = useMutation({
    mutationFn: () =>
      saveRecipe({
        projectId: selectedProject.value,
        name: recipeName.value,
        triggerCode: recipeTrigger.value,
        actionId: recipeAction.value,
        cooldownSeconds: 3600,
        maxRunsPerDay: 3,
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({
        queryKey: ['automation-recipes', selectedProject.value],
      })
      notice.value = 'Recipe saved disabled for separate review.'
    },
  })
  const toggleRecipe = useMutation({
    mutationFn: (input: { id: string; enabled: boolean }) =>
      setRecipeEnabled(input.id, input.enabled),
    onSuccess: () => {
      void queryClient.invalidateQueries({
        queryKey: ['automation-recipes', selectedProject.value],
      })
    },
  })
  const evaluate = useMutation({
    mutationFn: () => evaluateRecipes(selectedProject.value),
    onSuccess: (ids) => {
      notice.value = ids.length
        ? `${ids.length} safe operation(s) dispatched.`
        : 'No enabled recipe was due.'
    },
  })
  const acknowledge = useMutation({
    mutationFn: (id: string) => acknowledgeNotification(id),
    onSuccess: () => {
      void queryClient.invalidateQueries({
        queryKey: ['diagnostic-notifications', selectedProject.value],
      })
      notice.value = 'Diagnostic notification acknowledged locally.'
    },
  })
  const mutationFailed = computed(() =>
    [feedback, action, createRecipe, toggleRecipe, evaluate, acknowledge].some(
      (mutation) => mutation.isError.value,
    ),
  )
  const percent = (value: number) => `${Math.round(value * 100)}%`
  return {
    selectedProject,
    selectedProvider,
    recipeName,
    recipeTrigger,
    recipeAction,
    notice,
    projects,
    providers,
    diagnosis,
    recipes,
    notifications,
    availableProviders,
    suggestedActions,
    run,
    feedback,
    action,
    createRecipe,
    toggleRecipe,
    evaluate,
    acknowledge,
    mutationFailed,
    percent,
  }
}
