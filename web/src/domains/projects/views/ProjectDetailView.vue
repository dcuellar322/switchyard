<script setup lang="ts">
import { useMutation, useQuery } from "@tanstack/vue-query";
import { computed, ref, watch } from "vue";
import { RouterLink, useRoute, useRouter } from "vue-router";

import type {
  ActionDefinition,
  RuntimeAction,
  RuntimeLogEntry,
} from "../../../api/generated/types.gen";
import {
  formatBytes,
  isActiveState,
  projectInitials,
  stateLabel,
} from "../../../lib/format";
import { trackOperation } from "../../operations/store";
import { loadPortRegistry } from "../../ports/api";
import AgentSessionsPanel from "../../terminal/components/AgentSessionsPanel.vue";
import TerminalPanel from "../../terminal/components/TerminalPanel.vue";
import {
  loadProjectEnvironments,
  registerEnvironments,
} from "../../environments/api";
import {
  loadEffectiveManifest,
  loadProject,
  loadProjectActions,
  loadProjectGit,
  loadProjectHealth,
  loadProjectLogs,
  loadProjectMetrics,
  loadProjectRuntime,
  runProjectAction,
  runRuntimeAction,
} from "../api";
import { markProjectAccess } from "../recent";
import { useProjectLogStream } from "../composables/useProjectLogStream";

const props = defineProps<{ projectId: string }>();
const route = useRoute();
const router = useRouter();
const projectId = computed(() => props.projectId);
const tabs = [
  "overview",
  "logs",
  "terminal",
  "git",
  "ports",
  "storage",
  "agents",
  "config",
] as const;
type Tab = (typeof tabs)[number];
const activeTab = ref<Tab>(
  tabs.includes(route.query.tab as Tab) ? (route.query.tab as Tab) : "overview",
);
const operationError = ref("");
const liveLogs = ref<Array<RuntimeLogEntry>>([]);

const project = useQuery({
  queryKey: computed(() => ["project", projectId.value]),
  queryFn: () => loadProject(projectId.value),
});
const runtime = useQuery({
  queryKey: computed(() => ["project-runtime", projectId.value]),
  queryFn: () => loadProjectRuntime(projectId.value),
  refetchInterval: 5_000,
});
const health = useQuery({
  queryKey: computed(() => ["project-health", projectId.value]),
  queryFn: () => loadProjectHealth(projectId.value),
  refetchInterval: 5_000,
});
const logs = useQuery({
  queryKey: computed(() => ["project-logs", projectId.value]),
  queryFn: () => loadProjectLogs(projectId.value),
  refetchInterval: 10_000,
});
const metrics = useQuery({
  queryKey: computed(() => ["project-metrics", projectId.value]),
  queryFn: () => loadProjectMetrics(projectId.value),
  refetchInterval: 5_000,
});
const git = useQuery({
  queryKey: computed(() => ["project-git", projectId.value]),
  queryFn: () => loadProjectGit(projectId.value),
  refetchInterval: 10_000,
});
const actions = useQuery({
  queryKey: computed(() => ["project-actions", projectId.value]),
  queryFn: () => loadProjectActions(projectId.value),
});
const manifest = useQuery({
  queryKey: computed(() => ["project-manifest", projectId.value]),
  queryFn: () => loadEffectiveManifest(projectId.value),
});
const ports = useQuery({
  queryKey: ["ports"],
  queryFn: loadPortRegistry,
  refetchInterval: 10_000,
});
const environments = useQuery({
  queryKey: computed(() => ["project-environments", projectId.value]),
  queryFn: () => loadProjectEnvironments(projectId.value),
  refetchInterval: 10_000,
});

watch(() => props.projectId, markProjectAccess, { immediate: true });
watch(
  () => logs.data.value,
  (entries) => {
    if (entries) liveLogs.value = entries.slice(-300);
  },
  { immediate: true },
);
const logConnection = useProjectLogStream(projectId, (entry) => {
  if (liveLogs.value.some((item) => item.sequence === entry.sequence)) return;
  liveLogs.value = [...liveLogs.value, entry].slice(-300);
});

const lifecycle = useMutation({
  mutationFn: (action: RuntimeAction) =>
    runRuntimeAction(projectId.value, action),
  onSuccess: trackOperation,
});
const customAction = useMutation({
  mutationFn: (action: ActionDefinition) =>
    runProjectAction(projectId.value, action.id),
  onSuccess: trackOperation,
});
const registerWorktrees = useMutation({
  mutationFn: () => registerEnvironments(projectId.value),
  onSuccess: async () => {
    await Promise.all([environments.refetch(), git.refetch()]);
  },
});

const state = computed(() => runtime.data.value?.state ?? "unknown");
const active = computed(() => isActiveState(state.value));
const stateTone = computed(() =>
  ["degraded", "failed", "partially_running"].includes(state.value)
    ? "warning"
    : active.value
      ? "ready"
      : "neutral",
);
const memory = computed(
  () =>
    metrics.data.value?.reduce(
      (sum, item) => sum + (item.memoryAvailable ? item.memoryBytes : 0),
      0,
    ) ?? 0,
);
const cpu = computed(
  () =>
    metrics.data.value?.reduce(
      (sum, item) => sum + (item.cpuAvailable ? item.cpuPercent : 0),
      0,
    ) ?? 0,
);
const cpuAvailable = computed(() =>
  Boolean(metrics.data.value?.some((item) => item.cpuAvailable)),
);
const memoryAvailable = computed(() =>
  Boolean(metrics.data.value?.some((item) => item.memoryAvailable)),
);
const memoryLimit = computed(() =>
  Math.max(
    0,
    ...(metrics.data.value
      ?.filter((item) => item.memoryAvailable)
      .map((item) => item.memoryLimit) ?? []),
  ),
);
const changes = computed(() => {
  const value = git.data.value?.changes;
  return value
    ? value.staged + value.modified + value.untracked + value.conflicted
    : 0;
});
const projectPorts = computed(
  () =>
    ports.data.value?.facts.filter(
      (fact) => fact.projectId === projectId.value,
    ) ?? [],
);
const recentLogs = computed(() => liveLogs.value.slice(-80));
const terminalAction = computed(() =>
  actions.data.value?.actions.find(
    (action) => action.id === "terminal" || action.type === "terminal.open",
  ),
);
const browserAction = computed(() =>
  actions.data.value?.actions.find((action) => action.type === "browser.open"),
);
const quickActions = computed(
  () =>
    actions.data.value?.actions
      .filter(
        (action) =>
          ![terminalAction.value?.id, browserAction.value?.id].includes(
            action.id,
          ),
      )
      .slice(0, 5) ?? [],
);
const requiredHealth = computed(
  () => health.data.value?.results.filter((item) => item.required) ?? [],
);
const isPartial = computed(() =>
  [runtime, health, metrics, git, actions].some((query) => query.isError.value),
);

async function runLifecycle(action: RuntimeAction) {
  operationError.value = "";
  try {
    await lifecycle.mutateAsync(action);
  } catch (cause) {
    operationError.value =
      cause instanceof Error
        ? cause.message
        : "The lifecycle operation could not be queued.";
  }
}

async function runAction(action: ActionDefinition | undefined) {
  if (!action) return;
  operationError.value = "";
  try {
    await customAction.mutateAsync(action);
  } catch (cause) {
    operationError.value =
      cause instanceof Error
        ? cause.message
        : "The trusted action could not be queued.";
  }
}

async function selectTab(tab: Tab) {
  activeTab.value = tab;
  await router.replace({ query: tab === "overview" ? {} : { tab } });
}

function onTabKeydown(event: KeyboardEvent, index: number) {
  if (!["ArrowLeft", "ArrowRight", "Home", "End"].includes(event.key)) return;
  event.preventDefault();
  let next = index;
  if (event.key === "ArrowRight") next = (index + 1) % tabs.length;
  if (event.key === "ArrowLeft") next = (index - 1 + tabs.length) % tabs.length;
  if (event.key === "Home") next = 0;
  if (event.key === "End") next = tabs.length - 1;
  const tab = tabs[next];
  if (tab) void selectTab(tab);
  requestAnimationFrame(() => document.getElementById(`tab-${tab}`)?.focus());
}
</script>

<template>
  <section class="project-view" aria-labelledby="project-title">
    <div v-if="project.isPending.value" class="loading" aria-live="polite">
      <span></span><span></span><span></span>
    </div>
    <div
      v-else-if="project.isError.value"
      class="state-panel state-panel--error"
      role="alert"
    >
      <strong>Project unavailable</strong>
      <p>This project may have been removed or the daemon could not read it.</p>
      <RouterLink to="/projects">Return to projects</RouterLink>
    </div>
    <template v-else-if="project.data.value">
      <header class="project-hero">
        <div class="hero-identity">
          <RouterLink class="back" to="/projects" aria-label="Back to projects"
            >←</RouterLink
          >
          <div class="project-avatar" aria-hidden="true">
            {{ projectInitials(project.data.value.displayName) }}
          </div>
          <div>
            <div class="title-line">
              <h1 id="project-title">{{ project.data.value.displayName }}</h1>
              <span class="status" :class="`status--${stateTone}`"
                ><i></i>{{ stateLabel(state) }}</span
              >
            </div>
            <p>{{ project.data.value.primaryLocation }}</p>
          </div>
        </div>
        <div class="hero-actions">
          <button
            v-if="browserAction"
            type="button"
            :disabled="customAction.isPending.value"
            @click="runAction(browserAction)"
          >
            ↗ Open app
          </button>
          <button
            type="button"
            :disabled="customAction.isPending.value || !terminalAction"
            @click="runAction(terminalAction)"
          >
            ⌘ Terminal
          </button>
          <button
            v-if="active"
            type="button"
            :disabled="lifecycle.isPending.value"
            @click="runLifecycle('restart')"
          >
            ↻ Restart
          </button>
          <button
            class="primary"
            type="button"
            :disabled="lifecycle.isPending.value"
            @click="runLifecycle(active ? 'stop' : 'start')"
          >
            {{ active ? "■ Stop" : "▶ Start" }}
          </button>
        </div>
      </header>

      <p v-if="operationError" class="message message--error" role="alert">
        {{ operationError }}
      </p>
      <p v-if="isPartial" class="message" role="status">
        Some observations are unavailable. Available project controls and
        evidence remain usable.
      </p>
      <p
        v-if="
          runtime.data.value?.driver === 'compose' &&
          runtime.data.value.engine?.connected === false
        "
        class="message"
        role="status"
      >
        Docker is unavailable. Catalog, Git, manifest, and persisted logs remain
        available.
      </p>

      <nav class="tabs" role="tablist" aria-label="Project sections">
        <button
          v-for="(tab, index) in tabs"
          :id="`tab-${tab}`"
          :key="tab"
          type="button"
          role="tab"
          :aria-selected="activeTab === tab"
          :aria-controls="`panel-${tab}`"
          :tabindex="activeTab === tab ? 0 : -1"
          @click="selectTab(tab)"
          @keydown="onTabKeydown($event, index)"
        >
          {{ tab }}
        </button>
      </nav>

      <div
        v-if="activeTab === 'overview'"
        id="panel-overview"
        class="overview-grid"
        role="tabpanel"
        tabindex="0"
        aria-labelledby="tab-overview"
      >
        <div class="overview-main">
          <article class="panel">
            <header class="panel-head">
              <div>
                <p>Runtime</p>
                <h2>Services</h2>
              </div>
              <span
                >{{ runtime.data.value?.driver ?? "—" }} ·
                {{ runtime.data.value?.origin ?? "unobserved" }}</span
              >
            </header>
            <div v-if="runtime.isPending.value" class="panel-state">
              Observing runtime…
            </div>
            <div
              v-else-if="runtime.data.value?.services.length"
              class="service-table"
            >
              <div class="service-row service-row--head">
                <span>Service</span><span>Status</span><span>Health</span
				><span>Port</span><span>CPU</span><span>Memory</span>
              </div>
              <div
                v-for="service in runtime.data.value.services"
                :key="service.id"
                class="service-row"
              >
                <strong>{{ service.id }}</strong
                ><span
                  ><i
                    class="dot"
                    :class="{ online: service.state === 'running' }"
                  ></i
                  >{{ stateLabel(service.state) }}</span
                ><span>{{ service.health }}</span
                ><span>{{
                  service.ports
                    .map((port) => port.hostPort)
                    .filter(Boolean)
                    .join(", ") || "—"
                }}</span
				><span>{{
					metrics.data.value?.find((item) => item.serviceId === service.id)?.cpuAvailable
						? `${metrics.data.value.find((item) => item.serviceId === service.id)!.cpuPercent.toFixed(1)}%`
						: "—"
				}}</span><span>{{
					metrics.data.value?.find((item) => item.serviceId === service.id)?.memoryAvailable
						? formatBytes(metrics.data.value.find((item) => item.serviceId === service.id)!.memoryBytes)
						: "—"
				}}</span>
              </div>
            </div>
            <p v-else class="panel-state">
              No runtime services are currently observed.
            </p>
          </article>

          <article class="panel logs-panel">
            <header class="panel-head">
              <div>
                <p>Streaming output</p>
                <h2>Live logs</h2>
              </div>
              <div class="stream-state">
                <i
                  :class="{ online: logConnection.state.value === 'connected' }"
                ></i
                >{{ logConnection.state.value
                }}<button type="button" @click="selectTab('logs')">
                  View all →
                </button>
              </div>
            </header>
            <div
              v-if="recentLogs.length"
              class="log-lines"
              aria-label="Recent project logs"
            >
              <div v-for="entry in recentLogs.slice(-14)" :key="entry.sequence">
                <time>{{ new Date(entry.timestamp).toLocaleTimeString() }}</time
                ><span>{{ entry.serviceId }}</span
                ><code :class="{ stderr: entry.stream === 'stderr' }">{{
                  entry.message
                }}</code>
              </div>
            </div>
            <p v-else class="panel-state">
              No persisted or live log entries yet.
            </p>
          </article>
        </div>

        <aside class="overview-side">
          <article class="panel resource-card">
            <header class="panel-head">
              <div>
                <p>Current sample</p>
                <h2>Resources</h2>
              </div>
            </header>
            <dl>
              <div>
                <dt>CPU</dt>
				<dd>{{ cpuAvailable ? `${cpu.toFixed(1)}%` : "—" }}</dd>
                <span
					><i :style="{ width: `${cpuAvailable ? Math.min(cpu, 100) : 0}%` }"></i
                ></span>
              </div>
              <div>
                <dt>Memory</dt>
				<dd>{{ memoryAvailable ? formatBytes(memory) : "—" }}</dd>
                <span
                  ><i
                    :style="{
						width: `${memoryAvailable && memoryLimit > 0 ? Math.min((memory / memoryLimit) * 100, 100) : 0}%`,
                    }"
                  ></i
                ></span>
              </div>
            </dl>
            <RouterLink :to="{ name: 'resources', query: { project: projectId } }"
              >Open resource view →</RouterLink
            >
          </article>
          <article class="panel">
            <header class="panel-head">
              <div>
                <p>Repository</p>
                <h2>Git</h2>
              </div>
            </header>
            <dl class="compact-facts">
              <div>
                <dt>Branch</dt>
                <dd>
                  {{
                    git.data.value?.branch ??
                    (git.data.value?.detached ? "detached" : "—")
                  }}
                </dd>
              </div>
              <div>
                <dt>Working tree</dt>
                <dd :class="{ warn: changes }">
                  {{ changes ? `${changes} changes` : "Clean" }}
                </dd>
              </div>
              <div>
                <dt>Upstream</dt>
                <dd>
                  {{
                    git.data.value
                      ? `${git.data.value.ahead} ahead · ${git.data.value.behind} behind`
                      : "—"
                  }}
                </dd>
              </div>
            </dl>
            <button type="button" @click="selectTab('git')">
              Inspect repository →
            </button>
          </article>
          <article class="panel">
            <header class="panel-head">
              <div>
                <p>Checks</p>
                <h2>Health</h2>
              </div>
              <span>{{
                health.data.value?.observerState ?? "unavailable"
              }}</span>
            </header>
            <ul class="health-list">
              <li v-for="check in requiredHealth" :key="check.checkId">
                <i :class="{ online: check.status === 'healthy' }"></i
                ><span
                  ><strong>{{ check.serviceId }}</strong
                  ><small>{{ check.message }}</small></span
                >
              </li>
              <li v-if="!requiredHealth.length">
                No required health checks declared.
              </li>
            </ul>
          </article>
          <article v-if="quickActions.length" class="panel">
            <header class="panel-head">
              <div>
                <p>Trusted manifest</p>
                <h2>Quick actions</h2>
              </div>
            </header>
            <div class="quick-actions">
              <button
                v-for="action in quickActions"
                :key="action.id"
                type="button"
                :disabled="customAction.isPending.value"
                :title="`${action.risk} action`"
                @click="runAction(action)"
              >
                {{ action.name }}
              </button>
            </div>
          </article>
        </aside>
      </div>

      <div
        v-else-if="activeTab === 'logs'"
        id="panel-logs"
        class="single-panel"
        role="tabpanel"
        tabindex="0"
        aria-labelledby="tab-logs"
      >
        <article class="panel">
          <header class="panel-head">
            <div>
              <p>{{ liveLogs.length }} bounded entries</p>
              <h2>Project logs</h2>
            </div>
            <span class="stream-state"
              ><i
                :class="{ online: logConnection.state.value === 'connected' }"
              ></i
              >{{ logConnection.state.value }}</span
            >
          </header>
          <div v-if="liveLogs.length" class="log-lines log-lines--full">
            <div v-for="entry in liveLogs" :key="entry.sequence">
              <time>{{ new Date(entry.timestamp).toLocaleTimeString() }}</time
              ><span>{{ entry.serviceId }}</span
              ><code :class="{ stderr: entry.stream === 'stderr' }">{{
                entry.message
              }}</code>
            </div>
          </div>
          <p v-else class="panel-state">
            No log entries match this project yet.
          </p>
        </article>
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
        <article class="panel">
          <header class="panel-head">
            <div>
              <p>Read-only snapshot</p>
              <h2>Repository state</h2>
            </div>
            <span>{{
              git.data.value?.observedAt
                ? new Date(git.data.value.observedAt).toLocaleTimeString()
                : "—"
            }}</span>
          </header>
          <dl class="fact-grid">
            <div>
              <dt>Branch</dt>
              <dd>{{ git.data.value?.branch ?? "detached" }}</dd>
            </div>
            <div>
              <dt>HEAD</dt>
              <dd>
                <code>{{ git.data.value?.head?.slice(0, 12) ?? "—" }}</code>
              </dd>
            </div>
            <div>
              <dt>Ahead / behind</dt>
              <dd>
                {{ git.data.value?.ahead ?? 0 }} /
                {{ git.data.value?.behind ?? 0 }}
              </dd>
            </div>
            <div>
              <dt>Stashes</dt>
              <dd>{{ git.data.value?.stashes ?? 0 }}</dd>
            </div>
            <div>
              <dt>Modified</dt>
              <dd>{{ git.data.value?.changes.modified ?? 0 }}</dd>
            </div>
            <div>
              <dt>Untracked</dt>
              <dd>{{ git.data.value?.changes.untracked ?? 0 }}</dd>
            </div>
          </dl>
          <p v-if="git.data.value?.lastCommit" class="commit">
            <code>{{ git.data.value.lastCommit.shortHash }}</code>
            {{ git.data.value.lastCommit.subject }}
            <span>by {{ git.data.value.lastCommit.author }}</span>
          </p>
        </article>
        <article class="panel">
          <header class="panel-head">
            <div>
              <p>Parallel feature environments</p>
              <h2>Registered worktrees</h2>
            </div>
            <button type="button" :disabled="registerWorktrees.isPending.value" @click="registerWorktrees.mutate()">
              {{ registerWorktrees.isPending.value ? "Registering…" : "↻ Reconcile worktrees" }}
            </button>
          </header>
          <p v-if="registerWorktrees.isError.value" class="panel-state message--error" role="alert">
            {{ registerWorktrees.error.value?.message }}
          </p>
          <p v-else-if="environments.isPending.value" class="panel-state">Reading durable environment registrations…</p>
          <p v-else-if="environments.isError.value" class="panel-state message--error" role="alert">Worktree environments are unavailable.</p>
          <div v-else-if="environments.data.value?.length" class="environment-list">
            <article v-for="environment in environments.data.value" :key="environment.id">
              <div><strong>{{ environment.name }}</strong><span>{{ environment.primary ? "primary checkout" : environment.branch || "detached worktree" }}</span></div>
              <code>{{ environment.hostname }}</code>
              <span :class="`environment-state environment-state--${environment.state}`">{{ environment.state }}</span>
              <small>{{ environment.allocation.composeProjectName }} · {{ environment.allocation.portLeases.length }} exact ports</small>
            </article>
          </div>
          <p v-else class="panel-state">No worktrees are registered. Reconcile after the project is trusted.</p>
        </article>
      </div>
      <div
        v-else-if="activeTab === 'ports'"
        id="panel-ports"
        class="single-panel"
        role="tabpanel"
        tabindex="0"
        aria-labelledby="tab-ports"
      >
        <article class="panel">
          <header class="panel-head">
            <div>
              <p>Registry evidence</p>
              <h2>Project ports</h2>
            </div>
            <RouterLink to="/ports">Open global registry →</RouterLink>
          </header>
          <div class="fact-grid">
            <div v-for="fact in projectPorts" :key="fact.id">
              <dt>{{ fact.port }}/{{ fact.protocol }}</dt>
              <dd>{{ fact.serviceId ?? fact.kind }} · {{ fact.source }}</dd>
            </div>
            <p v-if="!projectPorts.length" class="panel-state">
              No declared, reserved, or observed ports.
            </p>
          </div>
        </article>
      </div>
      <div
        v-else-if="activeTab === 'storage'"
        id="panel-storage"
        class="single-panel"
        role="tabpanel"
        tabindex="0"
        aria-labelledby="tab-storage"
      >
        <article class="panel future-panel">
          <strong>Project storage intelligence</strong>
          <p>
            Inspect containers, images, volumes, and build cache attributed to
            this project. Shared, estimated, and unknown values stay labeled,
            and cleanup remains a non-executable preview.
          </p>
          <RouterLink :to="{ name: 'resources', query: { project: projectId } }"
            >Inspect project storage →</RouterLink
          >
        </article>
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
        <article class="panel">
          <header class="panel-head">
            <div>
              <p>{{ manifest.data.value?.sources.length ?? 0 }} sources</p>
              <h2>Effective manifest</h2>
            </div>
            <span>Read-only provenance</span>
          </header>
          <pre
            v-if="manifest.data.value"
          ><code>{{ JSON.stringify(manifest.data.value.manifest, null, 2) }}</code></pre>
          <p v-else class="panel-state">Effective manifest is unavailable.</p>
        </article>
      </div>
    </template>
  </section>
</template>

<style scoped>
.project-view {
  width: min(100%, 1600px);
  margin: 0 auto;
  padding: 25px 28px 42px;
}
.project-hero,
.hero-identity,
.title-line,
.hero-actions,
.panel-head,
.stream-state {
  display: flex;
  align-items: center;
}
.project-hero {
  justify-content: space-between;
  gap: 18px;
  margin-bottom: 20px;
}
.hero-identity {
  gap: 12px;
  min-width: 0;
}
.back {
  display: grid;
  place-items: center;
  width: 32px;
  height: 32px;
  border: 1px solid var(--border);
  border-radius: 9px;
  color: var(--muted);
  text-decoration: none;
}
.project-avatar {
  display: grid;
  place-items: center;
  width: 44px;
  height: 44px;
  border: 1px solid var(--border);
  border-radius: 12px;
  background: var(--panel-3);
  color: var(--accent);
  font-weight: 900;
}
.title-line {
  gap: 10px;
}
.title-line h1 {
  margin: 0;
  font-size: 22px;
  letter-spacing: -0.03em;
}
.hero-identity p {
  max-width: 620px;
  margin: 4px 0 0;
  overflow: hidden;
  text-overflow: ellipsis;
  color: var(--muted);
  font-size: 11px;
  white-space: nowrap;
}
.status {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 4px 8px;
  border: 1px solid var(--border);
  border-radius: 99px;
  color: var(--muted);
  font-size: 10px;
}
.status i,
.dot,
.stream-state i,
.health-list i {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: currentColor;
}
.status--ready {
  color: var(--green);
  border-color: rgba(84, 212, 154, 0.28);
  background: rgba(84, 212, 154, 0.07);
}
.status--warning {
  color: var(--yellow);
  border-color: rgba(241, 199, 91, 0.3);
  background: rgba(241, 199, 91, 0.07);
}
.hero-actions {
  gap: 7px;
}
.hero-actions button,
.panel button,
.panel a {
  padding: 8px 10px;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--panel-2);
  color: var(--text);
  text-decoration: none;
}
.hero-actions .primary {
  border-color: rgba(120, 166, 255, 0.5);
  background: var(--accent);
  color: #07111d;
  font-weight: 800;
}
.message {
  padding: 10px 12px;
  border: 1px solid rgba(241, 199, 91, 0.25);
  border-radius: 9px;
  background: rgba(241, 199, 91, 0.07);
  color: var(--yellow);
}
.message--error {
  border-color: rgba(255, 115, 115, 0.28);
  background: rgba(255, 115, 115, 0.07);
  color: var(--red);
}
.tabs {
  display: flex;
  gap: 4px;
  margin: 0 -28px 20px;
  padding: 0 28px;
  border-bottom: 1px solid var(--border);
  overflow: auto;
}
.tabs button {
  padding: 11px 13px;
  border: 0;
  border-bottom: 2px solid transparent;
  background: transparent;
  color: var(--muted);
  text-transform: capitalize;
}
.tabs button[aria-selected="true"] {
  border-bottom-color: var(--accent);
  color: var(--text);
}
.overview-grid {
  display: grid;
  grid-template-columns: minmax(0, 2fr) minmax(260px, 0.78fr);
  gap: 14px;
}
.overview-main,
.overview-side {
  display: grid;
  align-content: start;
  gap: 14px;
}
.panel {
  min-width: 0;
  padding: 16px;
  border: 1px solid var(--border);
  border-radius: 13px;
  background: linear-gradient(
    145deg,
    rgba(19, 25, 34, 0.97),
    rgba(13, 18, 25, 0.97)
  );
}
.panel-head {
  justify-content: space-between;
  gap: 14px;
  margin-bottom: 13px;
}
.panel-head p {
  margin: 0;
  color: var(--accent);
  font-size: 9px;
  font-weight: 800;
  letter-spacing: 0.12em;
  text-transform: uppercase;
}
.panel-head h2 {
  margin: 4px 0 0;
  font-size: 15px;
}
.panel-head > span,
.panel-head > div:last-child {
  color: var(--muted);
  font-size: 10px;
}
.service-table {
  border: 1px solid var(--border);
  border-radius: 9px;
  overflow: hidden;
}
.service-row {
  display: grid;
	grid-template-columns: 1.1fr 0.85fr 0.8fr 0.7fr 0.6fr 0.7fr;
  gap: 10px;
  padding: 9px 11px;
  border-top: 1px solid var(--border);
  align-items: center;
  color: var(--muted);
  font-size: 10px;
}
.service-row:first-child {
  border-top: 0;
}
.service-row--head {
  background: #0b1017;
  color: var(--soft);
  font-size: 9px;
  text-transform: uppercase;
}
.service-row strong {
  color: var(--text);
}
.dot {
  display: inline-block;
  margin-right: 6px;
  color: var(--soft);
}
.dot.online,
.stream-state i.online,
.health-list i.online {
  color: var(--green);
  background: var(--green);
}
.panel-state {
  padding: 20px;
  color: var(--muted);
  text-align: center;
}
.stream-state {
  gap: 6px;
}
.stream-state button {
  margin-left: 7px;
  padding: 0;
  border: 0;
  background: transparent;
  color: var(--accent);
}
.log-lines {
  height: 236px;
  overflow: auto;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: #080c11;
  font:
    10px/1.55 ui-monospace,
    SFMono-Regular,
    Menlo,
    monospace;
}
.log-lines > div {
  display: grid;
  grid-template-columns: 74px 76px minmax(0, 1fr);
  gap: 9px;
  padding: 3px 9px;
}
.log-lines time {
  color: var(--soft);
}
.log-lines span {
  overflow: hidden;
  text-overflow: ellipsis;
  color: var(--accent);
}
.log-lines code {
  overflow-wrap: anywhere;
  color: #b9c7da;
}
.log-lines code.stderr {
  color: var(--yellow);
}
.log-lines--full {
  height: min(650px, calc(100vh - 260px));
}
.resource-card dl {
  display: grid;
  gap: 12px;
}
.resource-card dl > div {
  display: grid;
  grid-template-columns: 1fr auto;
  gap: 5px;
}
.resource-card dt {
  color: var(--muted);
}
.resource-card dd {
  margin: 0;
  font-weight: 800;
}
.resource-card dl span {
  grid-column: 1/-1;
  height: 5px;
  border-radius: 3px;
  background: #202b39;
}
.resource-card dl i {
  display: block;
  height: 100%;
  max-width: 100%;
  border-radius: 3px;
  background: linear-gradient(90deg, var(--accent), var(--accent-2));
}
.resource-card > a {
  display: block;
  margin-top: 12px;
  text-align: center;
}
.compact-facts,
.fact-grid {
  display: grid;
  gap: 8px;
}
.compact-facts > div,
.fact-grid > div {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  padding: 8px;
  border-radius: 7px;
  background: #0c1118;
}
.compact-facts dt,
.fact-grid dt {
  color: var(--muted);
}
.compact-facts dd,
.fact-grid dd {
  margin: 0;
  text-align: right;
}
.warn {
  color: var(--yellow);
}
.health-list {
  display: grid;
  gap: 8px;
  margin: 0;
  padding: 0;
  list-style: none;
}
.health-list li {
  display: flex;
  gap: 8px;
  align-items: flex-start;
  color: var(--muted);
}
.health-list i {
  margin-top: 4px;
  background: var(--yellow);
}
.health-list span {
  display: grid;
}
.health-list small {
  margin-top: 2px;
  color: var(--soft);
}
.quick-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 7px;
}
.single-panel {
  display: grid;
  gap: 14px;
  max-width: 1220px;
}
.environment-list {
  display: grid;
  gap: 7px;
}
.environment-list > article {
  display: grid;
  grid-template-columns: minmax(150px, 1fr) minmax(180px, auto) auto;
  align-items: center;
  gap: 10px;
  padding: 11px 13px;
  border: 1px solid var(--border);
  border-radius: 9px;
  background: var(--panel-2);
}
.environment-list div,
.environment-list div span {
  display: grid;
  gap: 3px;
}
.environment-list div span,
.environment-list small {
  color: var(--soft);
  font-size: 10px;
}
.environment-list small {
  grid-column: 1 / -1;
}
.environment-state {
  padding: 3px 7px;
  border-radius: 99px;
  background: rgba(148, 163, 184, .1);
  color: var(--muted);
  font-size: 9px;
  text-transform: uppercase;
}
.environment-state--active { background: rgba(77, 208, 137, .12); color: var(--green); }
.future-panel {
  padding: 34px;
}
.future-panel strong {
  font-size: 18px;
}
.future-panel p {
  max-width: 680px;
  color: var(--muted);
  line-height: 1.6;
}
.fact-grid {
  grid-template-columns: repeat(3, 1fr);
}
.commit {
  padding: 12px;
  border-radius: 8px;
  background: #0c1118;
}
.commit span {
  color: var(--muted);
}
pre {
  max-height: 650px;
  margin: 0;
  overflow: auto;
  padding: 16px;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: #080c11;
  color: #c8d4e5;
  font-size: 11px;
}
.loading {
  display: grid;
  gap: 14px;
}
.loading span {
  height: 100px;
  border-radius: 13px;
  background: var(--panel);
  animation: pulse 1.2s infinite;
}
.loading span:nth-child(2) {
  height: 45px;
}
.loading span:nth-child(3) {
  height: 420px;
}
.state-panel {
  padding: 42px;
  border: 1px solid var(--border);
  border-radius: 14px;
  background: var(--panel);
  text-align: center;
}
.state-panel p {
  color: var(--muted);
}
.state-panel a {
  color: var(--accent);
}
@keyframes pulse {
  50% {
    opacity: 0.55;
  }
}
@media (max-width: 1050px) {
  .overview-grid {
    grid-template-columns: 1fr;
  }
  .overview-side {
    grid-template-columns: repeat(2, 1fr);
  }
}
@media (max-width: 800px) {
  .project-view {
    padding: 18px;
  }
  .project-hero {
    align-items: flex-start;
    display: grid;
  }
  .hero-actions {
    flex-wrap: wrap;
  }
  .tabs {
    margin-inline: -18px;
    padding-inline: 18px;
  }
  .overview-side {
    grid-template-columns: 1fr;
  }
  .service-row {
    grid-template-columns: 1fr 1fr;
  }
  .service-row--head {
    display: none;
  }
  .fact-grid {
    grid-template-columns: 1fr;
  }
  .log-lines > div {
    grid-template-columns: 65px 60px minmax(0, 1fr);
  }
}
</style>
