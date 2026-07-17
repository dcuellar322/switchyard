/* eslint-disable vue/one-component-per-file */
import { QueryClient, VueQueryPlugin } from "@tanstack/vue-query";
import { fireEvent, render, screen } from "@testing-library/vue";
import { defineComponent } from "vue";
import { createMemoryHistory, createRouter } from "vue-router";
import { expect, test, vi } from "vitest";

vi.mock("../../src/domains/projects/api", () => ({
  loadProjects: vi.fn().mockResolvedValue([
    {
      id: "alpha",
      displayName: "Alpha",
      slug: "alpha",
      primaryLocation: "/dev/alpha",
      tags: [],
      trustState: "trusted",
      manifestRevision: 1,
      createdAt: "2026-07-17T12:00:00Z",
      updatedAt: "2026-07-17T12:00:00Z",
    },
  ]),
  loadProjectLogs: vi.fn().mockResolvedValue([]),
}));

import LogExplorerView from "../../src/domains/logs/views/LogExplorerView.vue";

test("allows fleet log auto refresh to be disabled and configured", async () => {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: "/logs", component: LogExplorerView },
      { path: "/projects/:projectId", name: "project", component: defineComponent({ template: "<div />" }) },
      { path: "/discovery", component: defineComponent({ template: "<div />" }) },
    ],
  });
  await router.push("/logs");
  await router.isReady();

  render(LogExplorerView, {
    global: {
      plugins: [
        router,
        [VueQueryPlugin, { queryClient: new QueryClient({ defaultOptions: { queries: { retry: false } } }) }],
      ],
    },
  });

  await screen.findByText("No matching entries");
  const toggle = screen.getByRole("checkbox", { name: "Auto refresh" });
  expect(screen.getByRole("combobox", { name: "Refresh interval" })).toHaveValue("5000");
  await fireEvent.click(toggle);
  expect(screen.queryByRole("combobox", { name: "Refresh interval" })).not.toBeInTheDocument();
  expect(screen.getByText("Auto refresh off")).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "Refresh now" })).toBeEnabled();
});
