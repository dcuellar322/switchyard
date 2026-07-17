<script setup lang="ts">
import TelemetryPanel from '../../telemetry/components/TelemetryPanel.vue'
import SettingsAIProvidersPanel from '../components/SettingsAIProvidersPanel.vue'
import SettingsAppearancePanel from '../components/SettingsAppearancePanel.vue'
import SettingsProjectPanel from '../components/SettingsProjectPanel.vue'
import SettingsRetentionPanel from '../components/SettingsRetentionPanel.vue'
import SettingsRuntimePanel from '../components/SettingsRuntimePanel.vue'
import SettingsToolsPanel from '../components/SettingsToolsPanel.vue'
import { useDaemonSettings } from '../composables/useDaemonSettings'

const editor = useDaemonSettings()
</script>

<template>
  <section class="settings-view" aria-labelledby="settings-title">
    <header class="settings-heading">
      <div><p>Local control plane</p><h1 id="settings-title">Settings</h1><span>Durable preferences, safe roots, local integrations, privacy, and daemon identity.</span></div>
      <span v-if="editor.query.data.value" class="settings-revision">Revision {{ editor.query.data.value.settings.revision }}</span>
    </header>

    <div v-if="editor.query.isPending.value" class="settings-state" aria-live="polite"><span></span><span></span><span></span><strong>Loading durable settings…</strong></div>
    <div v-else-if="editor.query.isError.value" class="settings-state error" role="alert"><strong>Daemon settings are disconnected.</strong><p>{{ editor.query.error.value?.message }}</p><button type="button" @click="editor.query.refetch()">Retry</button></div>

    <form v-else-if="editor.draft.value" @submit.prevent="editor.save.mutate()">
      <p v-if="editor.query.data.value?.pendingRestart.length" class="settings-banner warning" role="status">Restart the daemon to apply: <strong>{{ editor.query.data.value.pendingRestart.join(', ') }}</strong>. Current collectors keep their previous safe bounds until then.</p>
      <p v-if="editor.save.isSuccess.value" class="settings-banner success" role="status">Settings revision {{ editor.query.data.value?.settings.revision }} saved.</p>
      <p v-if="editor.save.isError.value" class="settings-banner error" role="alert">{{ editor.save.error.value?.message }}</p>

      <div class="settings-grid">
        <SettingsRuntimePanel />
        <SettingsProjectPanel v-model="editor.draft.value" />
        <SettingsRetentionPanel v-model="editor.draft.value" />
        <SettingsToolsPanel v-model="editor.draft.value" />
        <SettingsAIProvidersPanel v-model="editor.draft.value" />
        <SettingsAppearancePanel v-model="editor.draft.value" />
        <article class="settings-panel safety"><div class="settings-panel__head"><div><p>Safety model</p><h2>Local by design</h2></div></div><p>Loopback browser sessions, owner-only IPC, server-resolved actions, durable operation audits, and generated contracts keep the browser away from raw command construction.</p></article>
        <TelemetryPanel />
      </div>

      <footer class="settings-actions">
        <span>{{ editor.dirty.value ? 'Unsaved changes' : `Saved ${new Date(editor.draft.value.updatedAt).toLocaleString()}` }}</span>
        <button class="settings-secondary" type="button" :disabled="!editor.dirty.value || editor.save.isPending.value" @click="editor.reset">Reset</button>
        <button class="settings-primary" type="submit" :disabled="!editor.dirty.value || editor.save.isPending.value">{{ editor.save.isPending.value ? 'Saving…' : 'Save settings' }}</button>
      </footer>
    </form>
  </section>
</template>

<style src="../settings.css"></style>
