import { createRouter, createWebHistory } from "vue-router";

import DashboardView from "./domains/dashboard/views/DashboardView.vue";
import PortRegistryView from "./domains/ports/views/PortRegistryView.vue";
import ProjectDetailView from "./domains/projects/views/ProjectDetailView.vue";
import ProjectOnboardingView from "./domains/projects/views/ProjectOnboardingView.vue";
import LogExplorerView from "./domains/logs/views/LogExplorerView.vue";
import ResourcesView from "./domains/resources/views/ResourcesView.vue";
import WorkspacesView from "./domains/workspaces/views/WorkspacesView.vue";
import PluginsView from "./domains/plugins/views/PluginsView.vue";
import DiagnosticsView from "./domains/diagnostics/views/DiagnosticsView.vue";
import SettingsView from "./domains/system/views/SettingsView.vue";
import FleetView from "./domains/fleet/views/FleetView.vue";

export const router = createRouter({
  history: createWebHistory(),
  scrollBehavior: () => ({ top: 0 }),
  routes: [
    { path: "/", name: "dashboard", component: DashboardView },
    {
      path: "/projects",
      name: "projects",
      component: DashboardView,
      props: { catalogOnly: true },
    },
    {
      path: "/projects/:projectId",
      name: "project",
      component: ProjectDetailView,
      props: true,
    },
    { path: "/ports", name: "ports", component: PortRegistryView },
    { path: "/resources", name: "resources", component: ResourcesView },
    { path: "/logs", name: "logs", component: LogExplorerView },
    { path: "/discovery", name: "discovery", component: ProjectOnboardingView },
    { path: "/workspaces", name: "workspaces", component: WorkspacesView },
    { path: "/plugins", name: "plugins", component: PluginsView },
    { path: "/fleet", name: "fleet", component: FleetView },
    { path: "/companion", name: "companion", component: FleetView, props: { readOnly: true } },
    { path: "/agents", name: "agents", component: DiagnosticsView },
    { path: "/settings", name: "settings", component: SettingsView },
    { path: "/:pathMatch(.*)*", redirect: "/" },
  ],
});
