<script setup lang="ts">
import { useQuery } from '@tanstack/vue-query'
import { computed, ref } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'

import AgentSessionsPanel from '../../terminal/components/AgentSessionsPanel.vue'
import TerminalPanel from '../../terminal/components/TerminalPanel.vue'
import { loadPortRegistry } from '../../ports/api'
import ProjectConfigTab from '../components/ProjectConfigTab.vue'
import ProjectGitTab from '../components/ProjectGitTab.vue'
import ProjectHeader from '../components/ProjectHeader.vue'
import ProjectLogsTab from '../components/ProjectLogsTab.vue'
import ProjectOverviewTab from '../components/ProjectOverviewTab.vue'
import ProjectPortsTab from '../components/ProjectPortsTab.vue'
import ProjectStorageTab from '../components/ProjectStorageTab.vue'
import ProjectTabList from '../components/ProjectTabList.vue'
import { useProjectDetail } from '../composables/useProjectDetail'
import { projectTabs, type ProjectTab } from '../projectTabs'

const props = defineProps<{ projectId: string }>()
const route = useRoute()
const router = useRouter()
const projectId = computed(() => props.projectId)
const activeTab = ref<ProjectTab>(
  projectTabs.includes(route.query.tab as ProjectTab)
    ? (route.query.tab as ProjectTab)
    : 'overview',
)
const {
  project,
  runtime,
  health,
  metrics,
  git,
  actions,
  manifest,
  environments,
  operationError,
  liveLogs,
  logConnection,
  lifecycle,
  customAction,
  registerWorktrees,
  state,
  active,
  stateTone,
  memory,
  cpu,
  cpuAvailable,
  memoryAvailable,
  memoryLimit,
  changes,
  recentLogs,
  terminalAction,
  browserAction,
  endpoints,
  quickActions,
  requiredHealth,
  isPartial,
  runLifecycle,
  runAction,
} = useProjectDetail(projectId)
const portRegistry = useQuery({
  queryKey: ['ports'],
  queryFn: loadPortRegistry,
  enabled: computed(() => activeTab.value === 'ports'),
  refetchInterval: computed(() => (activeTab.value === 'ports' ? 10_000 : false)),
})
const projectPorts = computed(
  () => portRegistry.data.value?.facts.filter((fact) => fact.projectId === projectId.value) ?? [],
)

async function selectTab(tab: ProjectTab) {
  activeTab.value = tab
  await router.replace({ query: tab === 'overview' ? {} : { tab } })
}
</script>

<template>
  <section class="project-view" aria-labelledby="project-title">
    <div v-if="project.isPending.value" class="loading" aria-live="polite">
      <span></span><span></span><span></span>
    </div>
    <div v-else-if="project.isError.value" class="state-panel state-panel--error" role="alert">
      <strong>Project unavailable</strong>
      <p>This project may have been removed or the daemon could not read it.</p>
      <RouterLink to="/projects">Return to projects</RouterLink>
    </div>
    <template v-else-if="project.data.value">
      <ProjectHeader
        :project="project.data.value"
        :state="state"
        :state-tone="stateTone"
        :active="active"
        :browser-action="browserAction"
        :action-pending="customAction.isPending.value"
        :lifecycle-pending="lifecycle.isPending.value"
        :operation-error="operationError"
        :partial="isPartial"
        :docker-unavailable="
          runtime.data.value?.driver === 'compose' && runtime.data.value.engine?.connected === false
        "
        :available-profiles="runtime.data.value?.availableProfiles ?? []"
        @action="runAction"
        @lifecycle="runLifecycle"
        @terminal="selectTab('terminal')"
      />
      <ProjectTabList :active="activeTab" @select="selectTab" />

      <div
        v-if="activeTab === 'overview'"
        id="panel-overview"
        role="tabpanel"
        tabindex="0"
        aria-labelledby="tab-overview"
      >
        <ProjectOverviewTab
          :project-id="projectId"
          :runtime="runtime.data.value"
          :runtime-pending="runtime.isPending.value"
          :metrics="metrics.data.value ?? []"
          :recent-logs="recentLogs"
          :log-connection="logConnection.state.value"
          :cpu="cpu"
          :cpu-available="cpuAvailable"
          :memory="memory"
          :memory-available="memoryAvailable"
          :memory-limit="memoryLimit"
          :git="git.data.value"
          :changes="changes"
          :health-state="health.data.value?.observerState ?? 'unavailable'"
          :required-health="requiredHealth"
          :endpoints="endpoints"
          :quick-actions="quickActions"
          :action-pending="customAction.isPending.value"
          @select-tab="selectTab"
          @run-action="runAction"
        />
      </div>
      <div
        v-else-if="activeTab === 'logs'"
        id="panel-logs"
        class="single-panel"
        role="tabpanel"
        tabindex="0"
        aria-labelledby="tab-logs"
      >
        <ProjectLogsTab :entries="liveLogs" :connection="logConnection.state.value" />
      </div>
      <div
        v-else-if="activeTab === 'terminal'"
        id="panel-terminal"
        class="single-panel"
        role="tabpanel"
        tabindex="0"
        aria-labelledby="tab-terminal"
      >
        <TerminalPanel
          :project-id="projectId"
          :services="runtime.data.value?.services.map((service) => service.id) ?? []"
          :environments="environments.data.value ?? []"
          :actions="actions.data.value?.actions ?? []"
          :external-available="Boolean(terminalAction)"
          @external="runAction(terminalAction)"
        />
      </div>
      <div
        v-else-if="activeTab === 'git'"
        id="panel-git"
        class="single-panel"
        role="tabpanel"
        tabindex="0"
        aria-labelledby="tab-git"
      >
        <ProjectGitTab
          :git="git.data.value"
          :environments="environments.data.value ?? []"
          :environments-pending="environments.isPending.value"
          :environments-error="environments.isError.value"
          :registration-pending="registerWorktrees.isPending.value"
          :registration-error="registerWorktrees.error.value?.message"
          @register="registerWorktrees.mutate()"
        />
      </div>
      <div
        v-else-if="activeTab === 'ports'"
        id="panel-ports"
        class="single-panel"
        role="tabpanel"
        tabindex="0"
        aria-labelledby="tab-ports"
      >
        <ProjectPortsTab :ports="projectPorts" />
      </div>
      <div
        v-else-if="activeTab === 'storage'"
        id="panel-storage"
        class="single-panel"
        role="tabpanel"
        tabindex="0"
        aria-labelledby="tab-storage"
      >
        <ProjectStorageTab :project-id="projectId" />
      </div>
      <div
        v-else-if="activeTab === 'agents'"
        id="panel-agents"
        class="single-panel"
        role="tabpanel"
        tabindex="0"
        aria-labelledby="tab-agents"
      >
        <AgentSessionsPanel
          :project-id="projectId"
          :environments="environments.data.value ?? []"
          @terminal="selectTab('terminal')"
        />
      </div>
      <div
        v-else
        id="panel-config"
        class="single-panel"
        role="tabpanel"
        tabindex="0"
        aria-labelledby="tab-config"
      >
        <ProjectConfigTab
          :manifest="manifest.data.value"
          :pending="manifest.isPending.value"
          :error="manifest.isError.value"
        />
      </div>
    </template>
  </section>
</template>

<style src="../project-detail.css"></style>
