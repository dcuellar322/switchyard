import { afterEach, describe, expect, it } from "vitest";

import type { Operation } from "../../src/api/generated/types.gen";
import {
  mergeTrackedOperations,
  trackOperation,
  useOperationStore,
} from "../../src/domains/operations/store";

function operation(state: Operation["state"], updatedAt: string): Operation {
  return {
    id: "operation-1",
    projectId: "project-1",
    kind: "action.run",
    state,
    cancellationRequested: false,
    requestedAt: "2026-07-16T12:00:00Z",
    updatedAt,
  };
}

describe("operation notices", () => {
  afterEach(() => useOperationStore().dismissNotice());

  it("only notices locally tracked work and follows its durable state", () => {
    const store = useOperationStore();
    mergeTrackedOperations([operation("succeeded", "2026-07-16T12:00:01Z")]);
    expect(store.notice.value).toBeUndefined();

    trackOperation(operation("queued", "2026-07-16T12:01:00Z"));
    mergeTrackedOperations([
      operation("succeeded", "2026-07-16T12:01:02Z"),
    ]);
    expect(store.notice.value?.state).toBe("succeeded");

    store.dismissNotice("operation-1");
    expect(store.notice.value).toBeUndefined();
  });
});
