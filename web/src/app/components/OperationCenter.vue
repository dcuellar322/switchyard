<script setup lang="ts">
import { useMutation, useQuery } from "@tanstack/vue-query";
import { computed, onBeforeUnmount, watch } from "vue";

import { isTerminalOperation, stateLabel } from "../../lib/format";
import {
  loadOperations,
  requestOperationCancellation,
} from "../../domains/operations/api";
import {
  mergeTrackedOperations,
  trackOperation,
  useOperationStore,
} from "../../domains/operations/store";

const store = useOperationStore();
const query = useQuery({
  queryKey: ["operations"],
  queryFn: loadOperations,
  refetchInterval: 1_500,
});
watch(
  () => query.data.value,
  (operations) => operations && mergeTrackedOperations(operations),
  { immediate: true },
);
const cancellation = useMutation({
  mutationFn: requestOperationCancellation,
  onSuccess: trackOperation,
});
const recent = computed(() => store.operations.value.slice(0, 20));
const toast = computed(() => store.notice.value);
let dismissalTimer: number | undefined;
watch(
  () =>
    toast.value
      ? `${toast.value.id}:${toast.value.state}:${toast.value.updatedAt}`
      : "",
  () => {
    if (dismissalTimer !== undefined) window.clearTimeout(dismissalTimer);
    dismissalTimer = undefined;
    const current = toast.value;
    if (!current || !isTerminalOperation(current.state)) return;
    dismissalTimer = window.setTimeout(
      () => store.dismissNotice(current.id),
      5_000,
    );
  },
  { immediate: true },
);
onBeforeUnmount(() => {
  if (dismissalTimer !== undefined) window.clearTimeout(dismissalTimer);
});
</script>

<template>
  <div
    v-if="toast && !store.drawerOpen.value"
    class="operation-toast"
    role="status"
    aria-live="polite"
    @click="store.open()"
  >
    <span
      class="operation-spinner"
      :class="{ done: isTerminalOperation(toast.state) }"
      aria-hidden="true"
    ></span>
    <span
      ><strong>{{ toast.kind.replace("runtime.", "") }}</strong
      ><small>{{ stateLabel(toast.state) }}</small></span
    >
    <button type="button" aria-label="Open operation details">→</button>
  </div>
  <div
    v-if="store.drawerOpen.value"
    class="drawer-backdrop"
    @mousedown.self="store.close()"
  >
    <aside class="operation-drawer" aria-labelledby="operation-drawer-title">
      <header>
        <div>
          <p>Durable work</p>
          <h2 id="operation-drawer-title">Operations</h2>
        </div>
        <button
          type="button"
          aria-label="Close operations"
          @click="store.close()"
        >
          ×
        </button>
      </header>
      <div v-if="query.isError.value" class="drawer-error" role="alert">
        Operations are unavailable.
        <button type="button" @click="query.refetch()">Retry</button>
      </div>
      <ol v-else-if="recent.length" class="operation-list">
        <li v-for="operation in recent" :key="operation.id">
          <span
            class="operation-spinner"
            :class="{
              done: isTerminalOperation(operation.state),
              failed: operation.state === 'failed',
            }"
            aria-hidden="true"
          ></span>
          <div>
            <strong>{{ operation.kind }}</strong
            ><code>{{ operation.id }}</code
            ><small v-if="operation.errorMessage">{{
              operation.errorMessage
            }}</small>
          </div>
          <div class="operation-state">
            <span>{{ stateLabel(operation.state) }}</span
            ><button
              v-if="!isTerminalOperation(operation.state)"
              type="button"
              :disabled="cancellation.isPending.value"
              @click="cancellation.mutate(operation.id)"
            >
              Cancel
            </button>
          </div>
        </li>
      </ol>
      <p v-else class="drawer-empty">No durable operations yet.</p>
    </aside>
  </div>
</template>

<style scoped>
.operation-toast {
  position: fixed;
  left: 252px;
  bottom: 22px;
  z-index: 70;
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 260px;
  padding: 12px 14px;
  border: 1px solid #344258;
  border-radius: 12px;
  background: #111720;
  box-shadow: 0 18px 60px rgba(0, 0, 0, 0.45);
  cursor: pointer;
}
@media (max-width: 760px) {
  .operation-toast {
    left: 94px;
  }
}
@media (max-width: 420px) {
  .operation-toast {
    right: 16px;
    left: 16px;
    min-width: 0;
  }
}
.operation-toast span:nth-child(2) {
  display: grid;
  gap: 2px;
}
.operation-toast small {
  color: var(--muted);
}
.operation-toast button {
  margin-left: auto;
  border: 0;
  background: transparent;
  color: var(--accent);
}
.operation-spinner {
  width: 10px;
  height: 10px;
  border: 2px solid var(--soft);
  border-top-color: var(--accent);
  border-radius: 50%;
  animation: spin 0.9s linear infinite;
}
.operation-spinner.done {
  border-color: var(--green);
  animation: none;
}
.operation-spinner.failed {
  border-color: var(--red);
}
.drawer-backdrop {
  position: fixed;
  inset: 0;
  z-index: 80;
  background: rgba(3, 6, 10, 0.52);
}
.operation-drawer {
  position: absolute;
  top: 0;
  right: 0;
  width: min(440px, 100vw);
  height: 100%;
  padding: 18px;
  border-left: 1px solid var(--border);
  background: #0d1219;
  box-shadow: -24px 0 80px rgba(0, 0, 0, 0.38);
}
.operation-drawer header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  padding-bottom: 15px;
  border-bottom: 1px solid var(--border);
}
.operation-drawer header p {
  margin: 0;
  color: var(--accent);
  font-size: 10px;
  text-transform: uppercase;
  letter-spacing: 0.13em;
}
.operation-drawer h2 {
  margin: 4px 0 0;
}
.operation-drawer header button {
  width: 34px;
  height: 34px;
  border: 1px solid var(--border);
  border-radius: 9px;
  background: var(--panel);
  color: var(--text);
}
.operation-list {
  display: grid;
  gap: 8px;
  margin: 15px 0;
  padding: 0;
  list-style: none;
}
.operation-list li {
  display: grid;
  grid-template-columns: 12px 1fr auto;
  gap: 10px;
  align-items: start;
  padding: 12px;
  border: 1px solid var(--border);
  border-radius: 10px;
  background: var(--panel);
}
.operation-list div:nth-child(2) {
  display: grid;
  gap: 4px;
  min-width: 0;
}
.operation-list code {
  overflow: hidden;
  text-overflow: ellipsis;
  color: var(--soft);
  font-size: 9px;
}
.operation-list small {
  color: var(--red);
}
.operation-state {
  display: grid;
  justify-items: end;
  gap: 7px;
  color: var(--muted);
  font-size: 10px;
}
.operation-state button,
.drawer-error button {
  padding: 5px 7px;
  border: 1px solid var(--border);
  border-radius: 7px;
  background: var(--panel-2);
  color: var(--text);
}
.drawer-empty,
.drawer-error {
  padding: 28px;
  color: var(--muted);
  text-align: center;
}
@keyframes spin {
  to {
    transform: rotate(360deg);
  }
}
</style>
