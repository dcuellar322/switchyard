import { onUnmounted, ref, watch, type Ref } from 'vue'

import type {
  AiEvidencePreview,
  AiGenerationLimits,
  AiManifestEnhancement,
  AiProviderDescriptor,
  ManifestProposal,
  Operation,
} from '../../../api/generated/types.gen'
import {
  loadAIEnhancement,
  loadAIProviders,
  loadManifestProposal,
  loadOperation,
  previewAIEvidence,
  startAIEnhancement,
  stopOperation,
} from '../api'
import { loadDaemonSettings } from '../../system/settingsApi'

const limits: AiGenerationLimits = {
  evidenceBytes: 65_536,
  outputBytes: 262_144,
  timeoutSeconds: 90,
  maxTurns: 1,
  maxOutputTokens: 4_096,
  maxBudgetUsd: 0,
}

export function useAssistedOnboarding(proposal: Ref<ManifestProposal | undefined>) {
  const providers = ref<Array<AiProviderDescriptor>>([])
  const selectedProvider = ref('')
  const evidencePreview = ref<AiEvidencePreview>()
  const evidenceConsented = ref(false)
  const operation = ref<Operation>()
  const run = ref<AiManifestEnhancement>()
  const pending = ref(false)
  const cancelling = ref(false)
  const error = ref('')
  let pollGeneration = 0

  watch(selectedProvider, () => {
    evidencePreview.value = undefined
    evidenceConsented.value = false
  })
  onUnmounted(() => { pollGeneration += 1 })

  async function initializeProviders() {
    const [loadedProviders, settings] = await Promise.all([
      loadAIProviders().catch(() => []),
      loadDaemonSettings().catch(() => undefined),
    ])
    providers.value = loadedProviders
    const preferred = settings?.settings.ai.defaultProvider
    if (preferred === 'none') {
      selectedProvider.value = ''
      return
    }
    selectedProvider.value = providers.value.find((provider) => provider.id === preferred && provider.available)?.id
      ?? providers.value.find((provider) => provider.available)?.id
      ?? ''
  }

  function reset() {
    pollGeneration += 1
    evidencePreview.value = undefined
    evidenceConsented.value = false
    operation.value = undefined
    run.value = undefined
    error.value = ''
  }

  async function previewEvidence() {
    if (!proposal.value || !selectedProvider.value) return
    pending.value = true
    error.value = ''
    evidenceConsented.value = false
    try {
      evidencePreview.value = await previewAIEvidence(proposal.value.id, limits)
    } catch (cause) {
      error.value = cause instanceof Error ? cause.message : 'Evidence preview failed.'
    } finally {
      pending.value = false
    }
  }

  async function enhance() {
    if (!proposal.value || !selectedProvider.value || !evidenceConsented.value) return
    pending.value = true
    error.value = ''
    run.value = undefined
    const sourceProposalId = proposal.value.id
    try {
      operation.value = await startAIEnhancement(sourceProposalId, selectedProvider.value, limits)
      const generation = ++pollGeneration
      pending.value = false
      while (generation === pollGeneration && operation.value && ['queued', 'running'].includes(operation.value.state)) {
        await new Promise((resolve) => window.setTimeout(resolve, 650))
        operation.value = await loadOperation(operation.value.id)
      }
      if (generation !== pollGeneration || !operation.value) return
      run.value = await loadAIEnhancement(sourceProposalId, operation.value.id)
      if (run.value.state === 'succeeded' && run.value.resultProposalId) {
        proposal.value = await loadManifestProposal(run.value.resultProposalId)
        evidencePreview.value = undefined
        evidenceConsented.value = false
      }
    } catch (cause) {
      error.value = cause instanceof Error ? cause.message : 'Assisted onboarding became disconnected.'
    } finally {
      pending.value = false
    }
  }

  async function cancel() {
    if (!operation.value) return
    cancelling.value = true
    try {
      operation.value = await stopOperation(operation.value.id)
    } catch (cause) {
      error.value = cause instanceof Error ? cause.message : 'Cancellation failed.'
    } finally {
      cancelling.value = false
    }
  }

  return {
    providers,
    selectedProvider,
    evidencePreview,
    evidenceConsented,
    operation,
    run,
    pending,
    cancelling,
    error,
    initializeProviders,
    reset,
    previewEvidence,
    enhance,
    cancel,
  }
}
