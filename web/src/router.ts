import { createRouter, createWebHistory } from "vue-router";

import DashboardView from "./domains/dashboard/views/DashboardView.vue";
import PortRegistryView from "./domains/ports/views/PortRegistryView.vue";
import ProjectDetailView from "./domains/projects/views/ProjectDetailView.vue";
import ProjectOnboardingView from "./domains/projects/views/ProjectOnboardingView.vue";
import LogExplorerView from "./domains/logs/views/LogExplorerView.vue";
import ResourcesView from "./domains/resources/views/ResourcesView.vue";
import WorkspacesView from "./domains/workspaces/views/WorkspacesView.vue";
import SettingsView from "./domains/system/views/SettingsView.vue";
import FeatureShellView from "./domains/shell/views/FeatureShellView.vue";

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
    {
      path: "/agents",
      name: "agents",
      component: FeatureShellView,
      props: {
        eyebrow: "Roadmap preview",
        title: "Agents",
        description:
          "Policy-governed automation with durable, inspectable work.",
        phase: "Planned for Phase 14",
      },
    },
    { path: "/settings", name: "settings", component: SettingsView },
    { path: "/:pathMatch(.*)*", redirect: "/" },
  ],
});
