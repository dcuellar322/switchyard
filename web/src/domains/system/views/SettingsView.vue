<script setup lang="ts">
import { computed, ref, watch } from "vue";

import { formatBytes } from "../../../lib/format";
import { useHostObservation } from "../composables/useHostObservation";
import { useSystemInfo } from "../composables/useSystemInfo";

const system = useSystemInfo();
const host = useHostObservation();
function readPreference(key: string, fallback: boolean) {
  try {
    const value = localStorage.getItem(key);
    return value === null ? fallback : value === "true";
  } catch {
    return fallback;
  }
}

function savePreference(key: string, value: boolean) {
  try {
    localStorage.setItem(key, String(value));
  } catch {
    /* Preferences are best-effort in restricted browsers. */
  }
}

const compact = ref(readPreference("switchyard.compact", false));
const relativeTime = ref(readPreference("switchyard.relative-time", true));
const saved = ref(false);
const uptime = computed(() =>
  system.data.value ? Date.now() - Date.parse(system.data.value.startedAt) : 0,
);

watch(
  [compact, relativeTime],
  () => {
    savePreference("switchyard.compact", compact.value);
    savePreference("switchyard.relative-time", relativeTime.value);
    document.documentElement.dataset.compact = compact.value ? "true" : "false";
    saved.value = true;
    window.setTimeout(() => {
      saved.value = false;
    }, 1_200);
  },
  { immediate: true },
);
</script>

<template>
  <section class="settings-view" aria-labelledby="settings-title">
    <header>
      <p>Local control plane</p>
      <h1 id="settings-title">Settings</h1>
      <span
        >Daemon identity, host capabilities, and browser-only preferences.</span
      >
    </header>
    <p v-if="system.isError.value" class="error" role="alert">
      Daemon information is unavailable.
      <button type="button" @click="system.refetch()">Retry</button>
    </p>
    <div class="settings-grid">
      <article class="panel">
        <div class="panel-head">
          <div>
            <p>Runtime identity</p>
            <h2>Switchyard daemon</h2>
          </div>
          <span :class="{ ready: system.data.value }">{{
            system.data.value ? "ready" : "unavailable"
          }}</span>
        </div>
        <dl>
          <div>
            <dt>Version</dt>
            <dd>{{ system.data.value?.version ?? "—" }}</dd>
          </div>
          <div>
            <dt>Commit</dt>
            <dd>
              <code>{{ system.data.value?.commit ?? "—" }}</code>
            </dd>
          </div>
          <div>
            <dt>API</dt>
            <dd>{{ system.data.value?.apiVersion ?? "—" }}</dd>
          </div>
          <div>
            <dt>Database schema</dt>
            <dd>{{ system.data.value?.databaseSchemaVersion ?? "—" }}</dd>
          </div>
          <div>
            <dt>Started</dt>
            <dd>
              {{
                system.data.value
                  ? new Date(system.data.value.startedAt).toLocaleString()
                  : "—"
              }}
            </dd>
          </div>
          <div>
            <dt>Uptime snapshot</dt>
            <dd>
              {{
                uptime
                  ? `${Math.floor(uptime / 3_600_000)}h ${Math.floor((uptime % 3_600_000) / 60_000)}m`
                  : "—"
              }}
            </dd>
          </div>
        </dl>
      </article>
      <article class="panel">
        <div class="panel-head">
          <div>
            <p>Capabilities</p>
            <h2>Host observation</h2>
          </div>
          <button type="button" @click="host.refetch()">Refresh</button>
        </div>
        <dl>
          <div>
            <dt>CPU</dt>
            <dd>
              {{
                host.data.value
                  ? `${host.data.value.cpuPercent.toFixed(1)}%`
                  : "—"
              }}
            </dd>
          </div>
          <div>
            <dt>Memory</dt>
            <dd>
              {{
                host.data.value
                  ? `${formatBytes(host.data.value.memoryUsedBytes)} / ${formatBytes(host.data.value.memoryTotalBytes)}`
                  : "—"
              }}
            </dd>
          </div>
          <div>
            <dt>Docker</dt>
            <dd>
              {{
                host.data.value?.docker.connected ? "Connected" : "Unavailable"
              }}
            </dd>
          </div>
          <div>
            <dt>Storage attribution</dt>
            <dd>{{ host.data.value?.docker.attribution ?? "unknown" }}</dd>
          </div>
        </dl>
        <p v-if="host.data.value?.warnings.length" class="warning">
          {{ host.data.value.warnings.join(" ") }}
        </p>
      </article>
      <article class="panel preferences">
        <div class="panel-head">
          <div>
            <p>This browser</p>
            <h2>Display preferences</h2>
          </div>
          <span v-if="saved" class="ready" role="status">Saved locally</span>
        </div>
        <label
          ><span
            ><strong>Compact density</strong
            ><small>Reduce vertical spacing in data-heavy views.</small></span
          ><input v-model="compact" type="checkbox"
        /></label>
        <label
          ><span
            ><strong>Relative timestamps</strong
            ><small
              >Preference stored for views that support relative time.</small
            ></span
          ><input v-model="relativeTime" type="checkbox"
        /></label>
        <p>
          These settings stay in this browser. They do not change daemon or
          project configuration.
        </p>
      </article>
      <article class="panel about">
        <div class="panel-head">
          <div>
            <p>Safety model</p>
            <h2>Local by design</h2>
          </div>
        </div>
        <p>
          Switchyard's control plane binds to loopback, requires an
          authenticated browser session, and executes only server-resolved
          actions from trusted manifests. The browser never constructs a shell
          command.
        </p>
        <p>
          Generated clients, durable operations, idempotency, and manifest
          provenance keep the interface aligned with the daemon contract.
        </p>
      </article>
    </div>
  </section>
</template>

<style scoped>
.settings-view {
  width: min(100%, 1200px);
  margin: 0 auto;
  padding: 30px 28px;
}
.settings-view > header p {
  margin: 0;
  color: var(--accent);
  font-size: 10px;
  font-weight: 800;
  letter-spacing: 0.13em;
  text-transform: uppercase;
}
.settings-view > header h1 {
  margin: 6px 0;
  font-size: 28px;
}
.settings-view > header span {
  color: var(--muted);
}
.settings-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 14px;
  margin-top: 24px;
}
.panel {
  padding: 18px;
  border: 1px solid var(--border);
  border-radius: 13px;
  background: linear-gradient(145deg, var(--panel), #0d1219);
}
.panel-head {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 12px;
  margin-bottom: 14px;
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
  font-size: 17px;
}
.panel-head > span {
  padding: 4px 7px;
  border: 1px solid var(--border);
  border-radius: 99px;
  color: var(--muted);
  font-size: 9px;
}
.panel-head .ready {
  border-color: rgba(84, 212, 154, 0.28);
  color: var(--green);
}
button {
  padding: 7px 9px;
  border: 1px solid var(--border);
  border-radius: 7px;
  background: var(--panel-2);
  color: var(--text);
}
dl {
  display: grid;
  gap: 7px;
  margin: 0;
}
dl > div {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  padding: 8px;
  border-radius: 7px;
  background: #0b1017;
}
dt {
  color: var(--muted);
}
dd {
  margin: 0;
  text-align: right;
}
.warning,
.error {
  padding: 10px;
  border: 1px solid rgba(241, 199, 91, 0.26);
  border-radius: 8px;
  background: rgba(241, 199, 91, 0.07);
  color: var(--yellow);
}
.error {
  border-color: rgba(255, 115, 115, 0.26);
  color: var(--red);
}
.preferences label {
  display: flex;
  justify-content: space-between;
  gap: 15px;
  align-items: center;
  padding: 12px 0;
  border-top: 1px solid var(--border);
}
.preferences label span {
  display: grid;
  gap: 4px;
}
.preferences small,
.preferences > p,
.about > p {
  color: var(--muted);
  line-height: 1.5;
}
.preferences input {
  width: 18px;
  height: 18px;
  accent-color: var(--accent);
}
.about > p:first-of-type {
  margin-top: 0;
}
@media (max-width: 760px) {
  .settings-view {
    padding: 20px 18px;
  }
  .settings-grid {
    grid-template-columns: 1fr;
  }
}
</style>
