<script setup lang="ts">
import type { DaemonSettings } from '../../../api/generated/types.gen'

const settings = defineModel<DaemonSettings>({ required: true })
</script>

<template>
  <article class="settings-panel">
    <div class="settings-panel__head">
      <div>
        <p>Local integrations</p>
        <h2>Tools and permissions</h2>
      </div>
      <span>Live</span>
    </div>
    <div class="settings-fields settings-fields--single">
      <label
        >Preferred terminal<select v-model="settings.tools.terminal">
          <option value="integrated">Integrated terminal</option>
          <option value="system">System terminal (Terminal.app on macOS)</option>
          <option value="iterm">iTerm2 (macOS only)</option>
        </select></label
      >
      <label
        >Preferred editor<select v-model="settings.tools.editor">
          <option value="vscode">Visual Studio Code</option>
          <option value="none">No external editor</option>
        </select></label
      >
      <label
        >Default agent profile<select v-model="settings.permissions.defaultAgentProfile">
          <option value="observe">Observe — bounded reads</option>
          <option value="develop">Develop — ordinary mutations</option>
          <option value="maintain">Maintain — rebuild and discovery</option>
          <option value="admin">Admin — destructive and trust decisions</option>
        </select></label
      >
    </div>
    <p class="settings-help">
      iTerm2 must already be installed. The agent profile default applies to new MCP sessions; an
      explicit <code>--profile</code> flag always wins.
    </p>
  </article>
</template>
