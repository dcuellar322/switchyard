<script setup lang="ts">
import type { AiProviderPreferences, DaemonSettings } from '../../../api/generated/types.gen'

const settings = defineModel<DaemonSettings>({ required: true })

function providerName(provider: AiProviderPreferences): string {
  return provider.id === 'openai-compatible' ? 'OpenAI-compatible HTTP' : provider.id === 'claude' ? 'Claude Code' : 'Codex'
}

function toggle(provider: AiProviderPreferences) {
  provider.enabled = !provider.enabled
  if (!provider.enabled && settings.value.ai.defaultProvider === provider.id) {
    const fallback = settings.value.ai.providers.find((candidate) => candidate.enabled)
    settings.value.ai.defaultProvider = fallback?.id ?? 'none'
  }
}
</script>

<template>
  <article class="settings-panel settings-panel--wide">
    <div class="settings-panel__head"><div><p>Provider-neutral assistance</p><h2>AI providers</h2></div><span>Restart</span></div>
    <p class="settings-help">Only adapter metadata and credential references are stored. Paste no token here; <code>env:NAME</code> resolves from the daemon environment after restart.</p>
    <label class="settings-default">Default provider<select v-model="settings.ai.defaultProvider"><option value="none">None — deterministic onboarding only</option><option v-for="provider in settings.ai.providers" :key="provider.id" :value="provider.id" :disabled="!provider.enabled">{{ providerName(provider) }}</option></select></label>
    <div class="provider-grid">
      <section v-for="provider in settings.ai.providers" :key="provider.id" class="provider-card">
        <header><div><strong>{{ providerName(provider) }}</strong><small>{{ provider.id }}</small></div><button type="button" :class="{ enabled: provider.enabled }" :aria-pressed="provider.enabled" @click="toggle(provider)">{{ provider.enabled ? 'Enabled' : 'Disabled' }}</button></header>
        <label v-if="provider.id !== 'openai-compatible'">Executable<input v-model.trim="provider.executable" :required="provider.enabled" autocomplete="off" /></label>
        <label v-else>HTTPS endpoint<input v-model.trim="provider.endpoint" :required="provider.enabled" type="url" autocomplete="off" placeholder="https://api.openai.com/v1" /></label>
        <label>Model <small>optional</small><input v-model.trim="provider.model" autocomplete="off" /></label>
        <label v-if="provider.id === 'openai-compatible'">Credential reference<input v-model.trim="provider.credentialReference" :required="provider.enabled" autocomplete="off" placeholder="env:OPENAI_API_KEY" /></label>
      </section>
    </div>
  </article>
</template>
