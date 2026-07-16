<script setup lang="ts">
import { useQuery } from "@tanstack/vue-query";
import { RouterLink } from "vue-router";

import { loadPortRegistry } from "../../domains/ports/api";
import { loadProjects } from "../../domains/projects/api";
import { useHostObservation } from "../../domains/system/composables/useHostObservation";

defineProps<{ connection: "connecting" | "connected" | "disconnected" }>();
const projects = useQuery({
  queryKey: ["projects"],
  queryFn: loadProjects,
  refetchInterval: 15_000,
});
const ports = useQuery({
  queryKey: ["ports"],
  queryFn: loadPortRegistry,
  refetchInterval: 10_000,
});
const host = useHostObservation();

const navigation = [
  { to: "/", label: "Dashboard", icon: "⌂" },
  { to: "/projects", label: "Projects", icon: "▦", count: "projects" },
  { to: "/ports", label: "Ports", icon: "⇄", count: "ports" },
  { to: "/resources", label: "Resources", icon: "◒" },
  { to: "/logs", label: "Logs", icon: "▤" },
  { to: "/discovery", label: "Discovery", icon: "◇" },
];
const tools = [
  { to: "/workspaces", label: "Workspaces", icon: "▥" },
  { to: "/agents", label: "Agents", icon: "◆" },
  { to: "/settings", label: "Settings", icon: "⚙" },
];
</script>

<template>
  <aside class="sidebar" aria-label="Primary navigation">
    <RouterLink class="brand" to="/" aria-label="Switchyard dashboard">
      <span class="brand-mark" aria-hidden="true">S</span>
      <span class="brand-copy"
        ><strong>Switchyard</strong
        ><small>Local development control</small></span
      >
    </RouterLink>
    <p class="nav-label">Command center</p>
    <nav>
      <RouterLink
        v-for="item in navigation"
        :key="item.to"
        class="nav-item"
        :to="item.to"
        :aria-label="item.label"
      >
        <span class="nav-icon" aria-hidden="true">{{ item.icon }}</span
        ><span class="nav-copy">{{ item.label }}</span>
        <span v-if="item.count === 'projects'" class="count">{{
          projects.data.value?.length ?? "—"
        }}</span>
        <span v-else-if="item.count === 'ports'" class="count">{{
          ports.data.value?.conflicts.length ?? "—"
        }}</span>
      </RouterLink>
    </nav>
    <p class="nav-label nav-label--tools">Tools</p>
    <nav>
      <RouterLink
        v-for="item in tools"
        :key="item.to"
        class="nav-item"
        :to="item.to"
        :aria-label="item.label"
      >
        <span class="nav-icon" aria-hidden="true">{{ item.icon }}</span
        ><span class="nav-copy">{{ item.label }}</span>
      </RouterLink>
    </nav>
    <div class="sidebar-footer">
      <div class="agent-card">
        <div>
          <span
            class="connection-dot"
            :class="`connection-dot--${connection}`"
            aria-hidden="true"
          ></span
          ><strong>Daemon {{ connection }}</strong
          ><span>v0.1</span>
        </div>
        <p>
          Docker
          {{
            host.data.value?.docker.connected ? "connected" : "unavailable"
          }}
          · {{ projects.data.value?.length ?? 0 }}
          {{
            projects.data.value?.length === 1 ? "project" : "projects"
          }}
          indexed
        </p>
      </div>
    </div>
  </aside>
</template>

<style scoped>
.sidebar {
  position: sticky;
  top: 0;
  display: flex;
  flex-direction: column;
  width: 230px;
  height: 100vh;
  padding: 22px 16px;
  border-right: 1px solid var(--border);
  background: rgba(13, 17, 24, 0.86);
  backdrop-filter: blur(18px);
  z-index: 30;
}
.brand {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 0 9px 22px;
  color: var(--text);
  text-decoration: none;
}
.brand-mark {
  width: 34px;
  height: 34px;
  border-radius: 10px;
  display: grid;
  place-items: center;
  background: linear-gradient(135deg, var(--accent), var(--accent-2));
  box-shadow: 0 10px 30px rgba(120, 166, 255, 0.22);
  color: #07111f;
  font-weight: 900;
  font-size: 17px;
}
.brand-copy {
  display: grid;
  gap: 2px;
}
.brand-copy strong {
  letter-spacing: -0.02em;
}
.brand-copy small {
  color: var(--muted);
  font-size: 11px;
}
.nav-label {
  margin: 0;
  padding: 16px 11px 8px;
  color: var(--soft);
  font-size: 10px;
  font-weight: 700;
  letter-spacing: 0.13em;
  text-transform: uppercase;
}
.nav-label--tools {
  padding-top: 22px;
}
.nav-item {
  display: flex;
  align-items: center;
  gap: 10px;
  width: 100%;
  margin: 2px 0;
  padding: 10px 11px;
  border-radius: 9px;
  color: var(--muted);
  text-decoration: none;
  transition: 0.18s ease;
}
.nav-item:hover {
  color: var(--text);
  background: rgba(255, 255, 255, 0.04);
}
.nav-item.router-link-exact-active {
  color: var(--text);
  background: linear-gradient(
    90deg,
    rgba(120, 166, 255, 0.18),
    rgba(120, 166, 255, 0.055)
  );
  box-shadow: inset 2px 0 var(--accent);
}
.nav-icon {
  width: 20px;
  text-align: center;
  color: var(--soft);
}
.router-link-exact-active .nav-icon {
  color: var(--accent);
}
.count {
  margin-left: auto;
  padding: 2px 6px;
  border: 1px solid var(--border);
  border-radius: 99px;
  color: var(--soft);
  font-size: 10px;
}
.sidebar-footer {
  margin-top: auto;
}
.agent-card {
  padding: 12px;
  border: 1px solid var(--border);
  border-radius: 12px;
  background: var(--panel);
}
.agent-card > div {
  display: flex;
  align-items: center;
  font-size: 11px;
}
.agent-card > div > span:last-child {
  margin-left: auto;
  color: var(--soft);
}
.agent-card p {
  margin: 9px 0 0;
  color: var(--soft);
  font-size: 10px;
  line-height: 1.35;
}
.connection-dot {
  display: inline-block;
  width: 8px;
  height: 8px;
  margin-right: 7px;
  border-radius: 50%;
  background: var(--yellow);
}
.connection-dot--connected {
  background: var(--green);
  box-shadow: 0 0 12px rgba(84, 212, 154, 0.6);
}
.connection-dot--disconnected {
  background: var(--red);
}
@media (max-width: 760px) {
  .sidebar {
    width: 72px;
    padding: 18px 10px;
  }
  .brand {
    justify-content: center;
    padding-inline: 0;
  }
  .brand-copy,
  .nav-copy,
  .nav-label,
  .count,
  .agent-card {
    display: none;
  }
  .nav-item {
    justify-content: center;
  }
  .nav-icon {
    width: auto;
  }
}
</style>
