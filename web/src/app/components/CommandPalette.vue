<script setup lang="ts">
import { useMutation, useQuery } from "@tanstack/vue-query";
import {
  computed,
  nextTick,
  onBeforeUnmount,
  onMounted,
  ref,
  watch,
} from "vue";
import { useRouter } from "vue-router";

import {
  loadProjects,
  runProjectAction,
  runRuntimeAction,
} from "../../domains/projects/api";
import { trackOperation } from "../../domains/operations/store";

const props = defineProps<{ open: boolean }>();
const emit = defineEmits<{ close: [] }>();
const router = useRouter();
const projects = useQuery({ queryKey: ["projects"], queryFn: loadProjects });
const query = ref("");
const selected = ref(0);
const input = ref<HTMLInputElement>();
const error = ref("");
let previousFocus: HTMLElement | null = null;

type PaletteItem = {
  id: string;
  label: string;
  hint: string;
  run: () => Promise<void>;
};

const operation = useMutation({
  mutationFn: async (item: {
    projectId: string;
    action: "start" | "terminal";
  }) =>
    item.action === "start"
      ? runRuntimeAction(item.projectId, "start")
      : runProjectAction(item.projectId, "terminal"),
  onSuccess: trackOperation,
});

async function navigate(path: string) {
  await router.push(path);
}

const items = computed<Array<PaletteItem>>(() => [
  {
    id: "dashboard",
    label: "⌂ Open dashboard",
    hint: "navigate",
    run: () => navigate("/"),
  },
  {
    id: "ports",
    label: "⇄ Find next available port",
    hint: "ports:next",
    run: () => navigate("/ports"),
  },
  {
    id: "logs",
    label: "▤ Show all error logs",
    hint: "logs:error",
    run: () => navigate("/logs?level=error"),
  },
  {
    id: "discovery",
    label: "◇ Scan a repository",
    hint: "project:add",
    run: () => navigate("/discovery"),
  },
  ...(projects.data.value ?? []).flatMap((project) => [
    {
      id: `open-${project.id}`,
      label: `Open ${project.displayName}`,
      hint: "project:open",
      run: () => navigate(`/projects/${project.id}`),
    },
    {
      id: `start-${project.id}`,
      label: `▶ Start ${project.displayName}`,
      hint: "project:start",
      run: async () => {
        await operation.mutateAsync({ projectId: project.id, action: "start" });
      },
    },
    {
      id: `terminal-${project.id}`,
      label: `⌘ Open ${project.displayName} terminal`,
      hint: "terminal:open",
      run: async () => {
        await operation.mutateAsync({
          projectId: project.id,
          action: "terminal",
        });
      },
    },
  ]),
]);
const filtered = computed(() => {
  const value = query.value.trim().toLowerCase();
  return (
    value
      ? items.value.filter((item) =>
          `${item.label} ${item.hint}`.toLowerCase().includes(value),
        )
      : items.value
  ).slice(0, 12);
});
const activeOptionId = computed(() =>
  filtered.value[selected.value]
    ? `palette-option-${filtered.value[selected.value]!.id}`
    : undefined,
);

watch(
  () => props.open,
  async (open) => {
    if (!open) {
      previousFocus?.focus();
      previousFocus = null;
      return;
    }
    previousFocus =
      document.activeElement instanceof HTMLElement
        ? document.activeElement
        : null;
    query.value = "";
    selected.value = 0;
    error.value = "";
    await nextTick();
    input.value?.focus();
  },
);
watch(filtered, () => {
  selected.value = Math.min(
    selected.value,
    Math.max(0, filtered.value.length - 1),
  );
});

async function choose(item: PaletteItem | undefined) {
  if (!item) return;
  error.value = "";
  try {
    await item.run();
    emit("close");
  } catch (cause) {
    error.value =
      cause instanceof Error
        ? cause.message
        : "The command could not be completed.";
  }
}

function onKeydown(event: KeyboardEvent) {
  if (!props.open) return;
  if (event.key === "Escape") emit("close");
  if (event.key === "ArrowDown") {
    event.preventDefault();
    selected.value = (selected.value + 1) % Math.max(filtered.value.length, 1);
  }
  if (event.key === "ArrowUp") {
    event.preventDefault();
    selected.value =
      (selected.value - 1 + Math.max(filtered.value.length, 1)) %
      Math.max(filtered.value.length, 1);
  }
  if (event.key === "Enter") {
    event.preventDefault();
    void choose(filtered.value[selected.value]);
  }
  if (event.key === "Tab") {
    const focusable = [
      ...document.querySelectorAll<HTMLElement>(
        ".palette input, .palette button",
      ),
    ];
    const first = focusable[0];
    const last = focusable.at(-1);
    if (event.shiftKey && document.activeElement === first) {
      event.preventDefault();
      last?.focus();
    } else if (!event.shiftKey && document.activeElement === last) {
      event.preventDefault();
      first?.focus();
    }
  }
}

onMounted(() => document.addEventListener("keydown", onKeydown));
onBeforeUnmount(() => document.removeEventListener("keydown", onKeydown));
</script>

<template>
  <div
    v-if="open"
    class="palette-backdrop"
    role="presentation"
    @mousedown.self="$emit('close')"
  >
    <section
      class="palette"
      role="dialog"
      aria-modal="true"
      aria-labelledby="palette-title"
    >
      <h2 id="palette-title" class="sr-only">Command palette</h2>
      <div class="palette-input">
        <span aria-hidden="true">⌕</span
        ><input
          ref="input"
          v-model="query"
          aria-label="Type a command or project"
          :aria-activedescendant="activeOptionId"
          aria-controls="command-palette-options"
          placeholder="Type a command or project…"
        /><kbd>ESC</kbd>
      </div>
      <div
        id="command-palette-options"
        class="palette-list"
        role="listbox"
        aria-label="Available commands"
      >
        <button
          v-for="(item, index) in filtered"
          :id="`palette-option-${item.id}`"
          :key="item.id"
          type="button"
          role="option"
          :aria-selected="selected === index"
          :class="{ selected: selected === index }"
          @mouseenter="selected = index"
          @click="choose(item)"
        >
          <span>{{ item.label }}</span
          ><code>{{ item.hint }}</code>
        </button>
        <p v-if="!filtered.length" class="empty">No matching commands.</p>
      </div>
      <p v-if="error" class="palette-error" role="alert">{{ error }}</p>
    </section>
  </div>
</template>

<style scoped>
.palette-backdrop {
  position: fixed;
  inset: 0;
  z-index: 100;
  display: flex;
  align-items: flex-start;
  justify-content: center;
  padding-top: 12vh;
  background: rgba(3, 6, 10, 0.72);
  backdrop-filter: blur(9px);
}
.palette {
  width: min(660px, calc(100vw - 30px));
  overflow: hidden;
  border: 1px solid #344258;
  border-radius: 14px;
  background: #111720;
  box-shadow: 0 35px 100px rgba(0, 0, 0, 0.55);
}
.palette-input {
  height: 52px;
  display: flex;
  align-items: center;
  gap: 11px;
  padding: 0 15px;
  border-bottom: 1px solid var(--border);
}
.palette-input input {
  flex: 1;
  border: 0;
  outline: 0;
  background: transparent;
  color: var(--text);
  font-size: 14px;
}
.palette-input kbd {
  padding: 2px 6px;
  border: 1px solid #344157;
  border-radius: 5px;
  background: #19202b;
  color: var(--soft);
  font-size: 10px;
}
.palette-list {
  max-height: 430px;
  overflow: auto;
  padding: 8px;
}
.palette-list button {
  display: flex;
  align-items: center;
  gap: 11px;
  width: 100%;
  padding: 11px;
  border: 0;
  border-radius: 8px;
  background: transparent;
  color: var(--muted);
  text-align: left;
}
.palette-list button.selected {
  background: rgba(120, 166, 255, 0.1);
  color: var(--text);
}
.palette-list code {
  margin-left: auto;
  color: var(--soft);
  font-size: 10px;
}
.empty,
.palette-error {
  margin: 0;
  padding: 14px;
  color: var(--muted);
}
.palette-error {
  color: var(--red);
  background: rgba(255, 115, 115, 0.06);
}
.sr-only {
  position: absolute;
  width: 1px;
  height: 1px;
  overflow: hidden;
  clip: rect(0, 0, 0, 0);
}
</style>
