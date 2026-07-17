/* eslint-disable vue/one-component-per-file */
import { QueryClient, VueQueryPlugin } from "@tanstack/vue-query";
import { render, screen } from "@testing-library/vue";
import { defineComponent } from "vue";
import { createMemoryHistory, createRouter } from "vue-router";
import { expect, test, vi } from "vitest";

vi.mock("../../src/domains/system/api", () => ({
  loadHostObservation: vi.fn().mockResolvedValue({
    cpuPercent: 12,
    memoryUsedBytes: 1_073_741_824,
    memoryTotalBytes: 8_589_934_592,
    docker: {
      connected: true,
      storageBytes: 2_147_483_648,
      reclaimableBytes: 0,
      attribution: "shared",
    },
    observedAt: "2026-07-15T12:00:00Z",
    warnings: [],
  }),
}));

vi.mock("../../src/domains/system/settingsApi", () => ({
  loadDaemonSettings: vi.fn().mockResolvedValue({
    settings: { appearance: { density: "comfortable", timeDisplay: "relative", theme: "dark" } },
    pendingRestart: [],
  }),
}));

vi.mock("../../src/domains/projects/api", () => ({
  loadProjects: vi
    .fn()
    .mockResolvedValue([{ id: "project-1", displayName: "Alpha", tags: [] }]),
  runProjectAction: vi.fn(),
  runRuntimeAction: vi.fn(),
}));

vi.mock("../../src/domains/ports/api", () => ({
  loadPortRegistry: vi
    .fn()
    .mockResolvedValue({
      facts: [],
      conflicts: [],
      warnings: [],
      observedAt: "2026-07-15T12:00:00Z",
    }),
}));

vi.mock("../../src/domains/operations/api", () => ({
  loadOperations: vi.fn().mockResolvedValue([]),
  requestOperationCancellation: vi.fn(),
}));

import App from "../../src/app/App.vue";

test("renders the routed application shell with live host and catalog summaries", async () => {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      {
        path: "/",
        component: defineComponent({ template: "<h1>Dashboard content</h1>" }),
      },
      {
        path: "/:pathMatch(.*)*",
        component: defineComponent({ template: "<div />" }),
      },
    ],
  });
  await router.push("/");
  await router.isReady();

  render(App, {
    global: {
      plugins: [
        router,
        [
          VueQueryPlugin,
          {
            queryClient: new QueryClient({
              defaultOptions: { queries: { retry: false } },
            }),
          },
        ],
      ],
    },
  });

  expect(
    await screen.findByRole("heading", { name: "Dashboard content" }),
  ).toBeInTheDocument();
  expect(await screen.findByText("12%")).toBeInTheDocument();
  expect(
    screen.getByRole("complementary", { name: "Primary navigation" }),
  ).toBeInTheDocument();
  expect(
    screen.getByRole("button", { name: /Projects, commands, ports/ }),
  ).toBeInTheDocument();
  expect(screen.getByText(/1 project indexed/)).toBeInTheDocument();
});
