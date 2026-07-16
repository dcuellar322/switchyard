import { QueryClient, VueQueryPlugin } from "@tanstack/vue-query";
import { fireEvent, render, screen } from "@testing-library/vue";
import { ref } from "vue";
import { createMemoryHistory, createRouter } from "vue-router";
import { expect, test, vi } from "vitest";

const { runRuntimeAction } = vi.hoisted(() => ({ runRuntimeAction: vi.fn() }));
runRuntimeAction.mockResolvedValue({
  id: "operation-1",
  projectId: "alpha",
  kind: "runtime.start",
  state: "queued",
  cancellationRequested: false,
  requestedAt: "2026-07-15T12:00:00Z",
  updatedAt: "2026-07-15T12:00:00Z",
});

vi.mock("../../src/domains/projects/api", () => ({
  loadProject: vi
    .fn()
    .mockResolvedValue({
      id: "alpha",
      slug: "alpha",
      displayName: "Alpha App",
      trustState: "trusted",
      primaryLocation: "/dev/alpha",
      tags: ["api"],
      manifestRevision: 1,
      createdAt: "2026-07-15T12:00:00Z",
      updatedAt: "2026-07-15T12:00:00Z",
    }),
  loadProjectRuntime: vi
    .fn()
    .mockResolvedValue({
      projectId: "alpha",
      driver: "compose",
      projectIdentity: "alpha",
      state: "stopped",
      origin: "switchyard",
      engine: { connected: false, errorCode: "unavailable" },
      services: [],
      observedAt: "2026-07-15T12:00:00Z",
    }),
  loadProjectHealth: vi
    .fn()
    .mockResolvedValue({
      projectId: "alpha",
      status: "unknown",
      observerState: "disconnected",
      results: [],
      observedAt: "2026-07-15T12:00:00Z",
    }),
  loadProjectLogs: vi
    .fn()
    .mockResolvedValue([
      {
        sequence: 1,
        timestamp: "2026-07-15T12:00:00Z",
        projectId: "alpha",
        serviceId: "api",
        runId: "run",
        source: "docker",
        stream: "stderr",
        level: "error",
        message: "database unavailable",
        redacted: false,
        attributes: {},
      },
    ]),
  loadProjectMetrics: vi.fn().mockResolvedValue([]),
  loadProjectGit: vi
    .fn()
    .mockResolvedValue({
      projectId: "alpha",
      repository: true,
      branch: "main",
      detached: false,
      ahead: 0,
      behind: 0,
      changes: { staged: 0, modified: 0, untracked: 0, conflicted: 0 },
      stashes: 0,
      remotes: [],
      worktrees: [],
      observedAt: "2026-07-15T12:00:00Z",
    }),
  loadProjectActions: vi
    .fn()
    .mockResolvedValue({
      projectId: "alpha",
      projectName: "Alpha App",
      actions: [
        {
          id: "terminal",
          name: "Open terminal",
          type: "terminal.open",
          command: [],
          workingDirectory: ".",
          shell: false,
          captureOutput: false,
          risk: "interactive",
          timeoutSeconds: 0,
        },
      ],
    }),
  loadEffectiveManifest: vi
    .fn()
    .mockResolvedValue({
      manifest: { schemaVersion: "switchyard.dev/v1alpha1" },
      provenance: {},
      sources: [{ name: "accepted" }],
    }),
  runProjectAction: vi.fn(),
  runRuntimeAction,
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
vi.mock("../../src/domains/projects/composables/useProjectLogStream", () => ({
  useProjectLogStream: () => ({
    state: ref("disconnected"),
    lastSequence: ref(1),
  }),
}));

import ProjectDetailView from "../../src/domains/projects/views/ProjectDetailView.vue";

test("keeps project controls usable and honest when Docker is unavailable", async () => {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      {
        path: "/projects/:projectId",
        name: "project",
        component: ProjectDetailView,
        props: true,
      },
      {
        path: "/projects",
        name: "projects",
        component: { template: "<div />" },
      },
      { path: "/ports", name: "ports", component: { template: "<div />" } },
      {
        path: "/resources",
        name: "resources",
        component: { template: "<div />" },
      },
    ],
  });
  await router.push("/projects/alpha");
  await router.isReady();
  render(ProjectDetailView, {
    props: { projectId: "alpha" },
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
    await screen.findByRole("heading", { name: "Alpha App" }),
  ).toBeInTheDocument();
  expect(screen.getByText(/Docker is unavailable/)).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "▶ Start" })).toBeEnabled();
  expect(screen.getByRole("button", { name: "⌘ Terminal" })).toBeEnabled();
  await fireEvent.click(screen.getByRole("button", { name: "▶ Start" }));
  expect(runRuntimeAction).toHaveBeenCalledWith("alpha", "start");

  const overview = screen.getByRole("tab", { name: "overview" });
  overview.focus();
  await fireEvent.keyDown(overview, { key: "ArrowRight" });
  expect(screen.getByRole("tab", { name: "logs" })).toHaveAttribute(
    "aria-selected",
    "true",
  );
  expect(await screen.findByText("database unavailable")).toBeInTheDocument();
});
