<script setup lang="ts">
import { ref } from 'vue'

import ProjectOnboardingView from '../domains/projects/views/ProjectOnboardingView.vue'
import SystemStatusView from '../domains/system/views/SystemStatusView.vue'

const activeView = ref<'projects' | 'system'>('system')
</script>

<template>
  <div class="app-shell">
    <aside class="sidebar" aria-label="Primary navigation">
      <div class="brand">
        <span class="brand__mark" aria-hidden="true">S</span>
        <span class="brand__copy">
          <strong>Switchyard</strong>
          <small>Local development control</small>
        </span>
      </div>
      <p class="nav-label">Command center</p>
      <nav>
        <button class="nav-item" :class="{ 'nav-item--active': activeView === 'projects' }" type="button" :aria-current="activeView === 'projects' ? 'page' : undefined" @click="activeView = 'projects'">
          <span aria-hidden="true">◇</span>
          <span>Projects</span>
        </button>
        <button class="nav-item" :class="{ 'nav-item--active': activeView === 'system' }" type="button" :aria-current="activeView === 'system' ? 'page' : undefined" @click="activeView = 'system'">
          <span aria-hidden="true">⌂</span>
          <span>System</span>
        </button>
      </nav>
      <p class="phase-note">Local-first · diagnostics ready</p>
    </aside>
    <main>
      <header class="topbar">
        <div class="command-placeholder" aria-label="Command palette coming in dashboard phase">
          <span aria-hidden="true">⌕</span>
          <span>Projects, commands, ports…</span>
          <kbd>⌘ K</kbd>
        </div>
      </header>
      <ProjectOnboardingView v-if="activeView === 'projects'" />
      <SystemStatusView v-else />
    </main>
  </div>
</template>

<style scoped>
.app-shell {
  min-height: 100vh;
  display: grid;
  grid-template-columns: 230px minmax(0, 1fr);
}

.sidebar {
  min-height: 100vh;
  padding: 22px 16px;
  border-right: 1px solid var(--border);
  background: rgba(13, 17, 24, 0.88);
}

.brand {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 0 9px 28px;
}

.brand__mark {
  width: 34px;
  height: 34px;
  display: grid;
  place-items: center;
  border-radius: 10px;
  background: linear-gradient(135deg, var(--accent), var(--accent-2));
  color: #07111f;
  font-weight: 900;
}

.brand__copy {
  display: grid;
  gap: 3px;
}

.brand__copy small,
.nav-label,
.phase-note {
  color: var(--soft);
  font-size: 11px;
}

.nav-label {
  padding: 0 11px;
  text-transform: uppercase;
  letter-spacing: 0.13em;
  font-size: 10px;
  font-weight: 700;
}

.nav-item {
	width: 100%;
	border: 0;
	background: transparent;
	font: inherit;
	cursor: pointer;
  display: flex;
  gap: 10px;
  align-items: center;
  padding: 10px 11px;
  border-radius: 9px;
  color: var(--text);
  text-decoration: none;
}

.nav-item--active {
  box-shadow: inset 2px 0 var(--accent);
  background: linear-gradient(90deg, rgba(120, 166, 255, 0.18), rgba(120, 166, 255, 0.055));
}

.phase-note {
  position: absolute;
  bottom: 20px;
  padding: 0 11px;
}

.topbar {
  height: 72px;
  display: flex;
  align-items: center;
  padding: 0 28px;
  border-bottom: 1px solid var(--border);
  background: rgba(10, 13, 18, 0.74);
}

.command-placeholder {
  width: min(480px, 46vw);
  height: 38px;
  display: flex;
  align-items: center;
  gap: 9px;
  padding: 0 12px;
  border: 1px solid var(--border);
  border-radius: 10px;
  background: var(--panel);
  color: var(--soft);
}

kbd {
  margin-left: auto;
  padding: 2px 6px;
  border: 1px solid #344157;
  border-radius: 5px;
  background: #19202b;
  color: var(--soft);
  font-size: 10px;
}

@media (max-width: 760px) {
  .app-shell {
    grid-template-columns: 72px minmax(0, 1fr);
  }

  .sidebar {
    padding: 18px 10px;
  }

  .brand__copy,
  .nav-item span:last-child,
  .nav-label,
  .phase-note {
    display: none;
  }

  .brand,
  .nav-item {
    justify-content: center;
  }

  .topbar {
    padding: 0 16px;
  }
}
</style>
