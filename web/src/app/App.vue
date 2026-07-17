<script setup lang="ts">
import { useQuery } from "@tanstack/vue-query";
import { onBeforeUnmount, onMounted, ref, watch } from "vue";
import { RouterView } from "vue-router";

import { queryClient } from "../queryClient";
import { useEventConnection } from "../domains/system/composables/useEventConnection";
import { loadDaemonSettings } from "../domains/system/settingsApi";
import AppSidebar from "./components/AppSidebar.vue";
import AppTopbar from "./components/AppTopbar.vue";
import CommandPalette from "./components/CommandPalette.vue";
import OperationCenter from "./components/OperationCenter.vue";

const paletteOpen = ref(false);
const preferences = useQuery({
  queryKey: ["daemon-settings"],
  queryFn: loadDaemonSettings,
});
watch(
  () => preferences.data.value?.settings.appearance,
  (appearance) => {
    if (!appearance) return;
    document.documentElement.dataset.compact =
      appearance.density === "compact" ? "true" : "false";
    document.documentElement.dataset.timeDisplay = appearance.timeDisplay;
    document.documentElement.dataset.theme = appearance.theme;
  },
  { immediate: true },
);
const pendingProjects = new Set<string>();
let operationsInvalid = false;
let invalidationTimer: number | undefined;
const terminalOperationEvents = new Set([
  "operation.succeeded",
  "operation.failed",
  "operation.canceled",
]);

function flushInvalidations() {
  invalidationTimer = undefined;
  if (operationsInvalid)
    void queryClient.invalidateQueries({ queryKey: ["operations"] });
  operationsInvalid = false;
  if (pendingProjects.size)
    void queryClient.invalidateQueries({ queryKey: ["dashboard-snapshots"] });
  for (const projectId of pendingProjects) {
    void queryClient.invalidateQueries({
      queryKey: ["project-runtime", projectId],
    });
    void queryClient.invalidateQueries({
      queryKey: ["project-health", projectId],
    });
  }
  pendingProjects.clear();
}

const events = useEventConnection((event) => {
  if (event.operationId || event.type?.startsWith("operation."))
    operationsInvalid = true;
  if (
    event.projectId &&
    (event.type === "runtime.observed" ||
      terminalOperationEvents.has(event.type ?? ""))
  ) {
    pendingProjects.add(event.projectId);
  }
  if (invalidationTimer === undefined)
    invalidationTimer = window.setTimeout(flushInvalidations, 2_000);
});

function onGlobalKeydown(event: KeyboardEvent) {
  if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === "k") {
    event.preventDefault();
    paletteOpen.value = true;
  }
}

onMounted(() => document.addEventListener("keydown", onGlobalKeydown));
onBeforeUnmount(() => {
  document.removeEventListener("keydown", onGlobalKeydown);
  if (invalidationTimer !== undefined) window.clearTimeout(invalidationTimer);
});
</script>

<template>
  <a class="skip-link" href="#main-content">Skip to content</a>
  <div class="app-shell">
    <AppSidebar :connection="events" />
    <div class="app-main">
      <AppTopbar @palette="paletteOpen = true" />
      <main id="main-content"><RouterView /></main>
    </div>
    <CommandPalette :open="paletteOpen" @close="paletteOpen = false" />
    <OperationCenter />
  </div>
</template>

<style scoped>
.skip-link {
  position: fixed;
  top: 8px;
  left: 50%;
  z-index: 200;
  transform: translate(-50%, -180%);
  padding: 9px 13px;
  border-radius: 8px;
  background: var(--accent);
  color: #07111d;
  font-weight: 800;
}
.skip-link:focus {
  transform: translate(-50%, 0);
}
.app-shell {
  min-height: 100vh;
  display: grid;
  grid-template-columns: 230px minmax(0, 1fr);
}
.app-main {
  min-width: 0;
}
#main-content {
  min-height: calc(100vh - 72px);
}
@media (max-width: 760px) {
  .app-shell {
    grid-template-columns: 72px minmax(0, 1fr);
  }
}
</style>
