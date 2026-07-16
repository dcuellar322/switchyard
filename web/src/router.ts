import { createRouter, createWebHistory } from "vue-router";

const DashboardView = () => import("./domains/dashboard/views/DashboardView.vue");
const PortRegistryView = () => import("./domains/ports/views/PortRegistryView.vue");
const ProjectDetailView = () => import("./domains/projects/views/ProjectDetailView.vue");
const ProjectOnboardingView = () => import("./domains/projects/views/ProjectOnboardingView.vue");
const LogExplorerView = () => import("./domains/logs/views/LogExplorerView.vue");
const ResourcesView = () => import("./domains/resources/views/ResourcesView.vue");
const WorkspacesView = () => import("./domains/workspaces/views/WorkspacesView.vue");
const PluginsView = () => import("./domains/plugins/views/PluginsView.vue");
const DiagnosticsView = () => import("./domains/diagnostics/views/DiagnosticsView.vue");
const SettingsView = () => import("./domains/system/views/SettingsView.vue");
const FleetView = () => import("./domains/fleet/views/FleetView.vue");
const TeamView = () => import("./domains/team/views/TeamView.vue");

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
    { path: "/team", name: "team", component: TeamView },
    { path: "/companion", name: "companion", component: FleetView, props: { readOnly: true } },
    { path: "/agents", name: "agents", component: DiagnosticsView },
    { path: "/settings", name: "settings", component: SettingsView },
    { path: "/:pathMatch(.*)*", redirect: "/" },
  ],
});
