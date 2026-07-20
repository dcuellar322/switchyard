import { useQuery, useQueryClient } from '@tanstack/vue-query'
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'

import type { ManifestProposal, Project } from '../../../api/generated/types.gen'
import { loadDaemonSettings } from '../../system/settingsApi'
import { approveProposal, loadProjects, revalidateProposal, scanRepository } from '../api'
import { useAssistedOnboarding } from './useAssistedOnboarding'

type JSONObject = Record<string, unknown>
const objectValue = (value: unknown): JSONObject =>
  typeof value === 'object' && value !== null && !Array.isArray(value) ? (value as JSONObject) : {}
const objectArray = (value: unknown): Array<JSONObject> =>
  Array.isArray(value) ? value.map(objectValue) : []

export function useProjectOnboarding() {
  const router = useRouter()
  const queryClient = useQueryClient()
  const repositoryPath = ref('')
  const allowOutsideRoots = ref(false)
  const settings = useQuery({ queryKey: ['daemon-settings'], queryFn: loadDaemonSettings })
  const projects = ref<Array<Project>>([])
  const proposal = ref<ManifestProposal>()
  const pending = ref(false)
  const error = ref('')
  const assisted = useAssistedOnboarding(proposal)
  const busy = computed(() => pending.value || assisted.pending.value)
  const displayedError = computed(() => error.value || assisted.error.value)
  const candidate = computed(() => objectValue(proposal.value?.candidate))
  const metadata = computed(() => objectValue(candidate.value.metadata))
  const runtime = computed(() => objectValue(candidate.value.runtime))
  const services = computed(() => objectArray(candidate.value.services))
  const ports = computed(() => objectArray(candidate.value.ports))
  const actions = computed(() => objectArray(candidate.value.actions))
  const command = (action: JSONObject): string =>
    Array.isArray(action.command) ? action.command.join(' ') : ''

  onMounted(async () => {
    const [loadedProjects] = await Promise.all([
      loadProjects().catch(() => []),
      assisted.initializeProviders(),
    ])
    projects.value = loadedProjects
  })
  async function scan() {
    pending.value = true
    error.value = ''
    try {
      proposal.value = await scanRepository(repositoryPath.value, allowOutsideRoots.value)
      assisted.reset()
    } catch (cause) {
      error.value = cause instanceof Error ? cause.message : 'Repository scan failed.'
    } finally {
      pending.value = false
    }
  }
  async function validate() {
    if (!proposal.value) return
    pending.value = true
    proposal.value = await revalidateProposal(proposal.value.id).finally(
      () => (pending.value = false),
    )
  }
  async function accept() {
    if (!proposal.value) return
    pending.value = true
    try {
      const accepted = await approveProposal(proposal.value.id)
      proposal.value = accepted.proposal
      projects.value = await loadProjects()
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['projects'] }),
        queryClient.invalidateQueries({ queryKey: ['ports'] }),
      ])
      await router.push({ name: 'project', params: { projectId: accepted.project.id } })
    } finally {
      pending.value = false
    }
  }
  return {
    repositoryPath,
    allowOutsideRoots,
    settings,
    projects,
    proposal,
    pending,
    assisted,
    busy,
    displayedError,
    metadata,
    runtime,
    services,
    ports,
    actions,
    command,
    scan,
    validate,
    accept,
  }
}
