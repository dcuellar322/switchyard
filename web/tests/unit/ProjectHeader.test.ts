import { cleanup, fireEvent, render } from "@testing-library/vue";
import { afterEach, expect, test } from "vitest";

import type {
  ActionDefinition,
  Project,
} from "../../src/api/generated/types.gen";
import ProjectHeader from "../../src/domains/projects/components/ProjectHeader.vue";

afterEach(cleanup);

const project: Project = {
  id: "alpha",
  slug: "alpha",
  displayName: "Alpha App",
  trustState: "trusted",
  primaryLocation: "/dev/alpha",
  tags: [],
  manifestRevision: 1,
  createdAt: "2026-07-15T12:00:00Z",
  updatedAt: "2026-07-15T12:00:00Z",
};

const terminalAction: ActionDefinition = {
  id: "terminal",
  name: "Open terminal",
  type: "terminal.open",
  command: [],
  workingDirectory: ".",
  shell: false,
  captureOutput: false,
  risk: "interactive",
  timeoutSeconds: 0,
};

function renderHeader(action?: ActionDefinition) {
  return render(ProjectHeader, {
    props: {
      project,
      state: "stopped",
      stateTone: "neutral",
      active: false,
      terminalAction: action,
      actionPending: false,
      lifecyclePending: false,
      operationError: "",
      partial: false,
      dockerUnavailable: false,
      availableProfiles: [],
    },
    global: { stubs: { RouterLink: { template: "<a><slot /></a>" } } },
  });
}

test("opens the integrated terminal when no external terminal action is resolved", async () => {
  const view = renderHeader();

  await fireEvent.click(view.getByRole("button", { name: "⌘ Terminal" }));

  expect(view.emitted("terminal")).toHaveLength(1);
  expect(view.emitted("action")).toBeUndefined();
});

test("runs the resolved system terminal action when one is available", async () => {
  const view = renderHeader(terminalAction);

  await fireEvent.click(view.getByRole("button", { name: "⌘ Terminal" }));

  expect(view.emitted("action")).toEqual([[terminalAction]]);
  expect(view.emitted("terminal")).toBeUndefined();
});

test("offers trusted Compose profiles when starting a stopped project", async () => {
  const view = render(ProjectHeader, {
    props: {
      project,
      state: "stopped",
      stateTone: "neutral",
      active: false,
      actionPending: false,
      lifecyclePending: false,
      operationError: "",
      partial: false,
      dockerUnavailable: false,
      availableProfiles: ["marketing"],
    },
    global: { stubs: { RouterLink: { template: "<a><slot /></a>" } } },
  });

  await fireEvent.click(view.getByRole("button", { name: "▶ Start" }));
  expect(view.getByRole("dialog", { name: "Start Alpha App services" })).toBeInTheDocument();
  await fireEvent.click(view.getByRole("checkbox", { name: "Marketing" }));
  await fireEvent.click(view.getByRole("button", { name: "▶ Start services" }));

  expect(view.emitted("lifecycle")).toEqual([["start", ["marketing"]]]);
});
