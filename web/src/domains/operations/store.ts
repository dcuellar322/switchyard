import { computed, ref } from 'vue'

import type { Operation } from '../../api/generated/types.gen'

const tracked = ref<Array<Operation>>([])
const drawerOpen = ref(false)
const notice = ref<Operation>()

export function trackOperation(operation: Operation): void {
  tracked.value = [operation, ...tracked.value.filter((item) => item.id !== operation.id)].slice(
    0,
    20,
  )
  notice.value = operation
}

export function mergeTrackedOperations(operations: Array<Operation>): void {
  const byId = new Map(tracked.value.map((operation) => [operation.id, operation]))
  for (const operation of operations) byId.set(operation.id, operation)
  tracked.value = [...byId.values()]
    .sort((left, right) => Date.parse(right.updatedAt) - Date.parse(left.updatedAt))
    .slice(0, 20)
  if (notice.value) notice.value = byId.get(notice.value.id) ?? notice.value
}

export function useOperationStore() {
  return {
    operations: computed(() => tracked.value),
    notice: computed(() => notice.value),
    drawerOpen,
    open: () => {
      drawerOpen.value = true
    },
    close: () => {
      drawerOpen.value = false
    },
    toggle: () => {
      drawerOpen.value = !drawerOpen.value
    },
    dismissNotice: (operationId?: string) => {
      if (!operationId || notice.value?.id === operationId) notice.value = undefined
    },
  }
}
