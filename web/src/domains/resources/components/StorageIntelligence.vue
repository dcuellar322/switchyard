<script setup lang="ts">
import { computed, ref, watch } from 'vue'

import type {
  CleanupPreview,
  ResourceProjectSnapshot,
  StorageInventory,
} from '../../../api/generated/types.gen'
import { formatBytes } from '../../../lib/format'

const props = defineProps<{
  inventory?: StorageInventory
  projects: Array<ResourceProjectSnapshot>
  projectId: string
  preview?: CleanupPreview
  loading: boolean
  pending: boolean
  inventoryError: boolean
  previewError: string
}>()
const emit = defineEmits<{
  selectProject: [projectId: string]
  preview: [projectId: string]
  retry: []
}>()
const projectFilter = ref(props.projectId)
const kindFilter = ref('')
const resources = computed(() =>
  (props.inventory?.resources ?? []).filter(
    (item) =>
      (!projectFilter.value || item.projectIds.includes(projectFilter.value)) &&
      (!kindFilter.value || item.kind === kindFilter.value),
  ),
)

watch(
  () => props.projectId,
  (projectId) => {
    projectFilter.value = projectId
  },
)

function selectProject(projectId: string) {
  projectFilter.value = projectId
  emit('selectProject', projectId)
}
</script>

<template>
  <article class="panel storage">
    <header class="panel-head">
      <div>
        <p class="eyebrow">Docker storage</p>
        <h2>Attribution and cleanup preview</h2>
      </div>
      <span class="boundary">Inspection only · no delete capability</span>
    </header>
    <p v-if="inventory && !inventory.connected" class="warning" role="status">
      Docker is disconnected. Persisted process metrics and history remain available.
    </p>
    <div class="filters">
      <label
        >Project<select
          :value="projectFilter"
          @change="selectProject(($event.target as HTMLSelectElement).value)"
        >
          <option value="">All and unowned</option>
          <option v-for="project in projects" :key="project.projectId" :value="project.projectId">
            {{ project.name }}
          </option>
        </select></label
      ><label
        >Kind<select v-model="kindFilter">
          <option value="">All kinds</option>
          <option value="container">Containers</option>
          <option value="image">Images</option>
          <option value="volume">Volumes</option>
          <option value="build_cache">Build cache</option>
        </select></label
      ><button
        type="button"
        :disabled="pending || !inventory?.connected"
        @click="emit('preview', projectFilter)"
      >
        {{ pending ? 'Inspecting…' : 'Preview reclaimable' }}
      </button>
    </div>
    <div v-if="inventoryError" class="state" role="alert">
      Storage inventory could not be read.
      <button type="button" @click="emit('retry')">Retry</button>
    </div>
    <div v-else-if="loading" class="state" aria-live="polite">Inspecting Docker storage…</div>
    <div v-else-if="!resources.length" class="state">No storage resources match these filters.</div>
    <div v-else class="inventory">
      <div v-for="item in resources" :key="`${item.kind}:${item.id}`" class="resource">
        <div>
          <strong>{{ item.name || item.id }}</strong
          ><code>{{ item.kind }} · {{ item.id }}</code>
        </div>
        <span>{{ item.bytes === undefined ? 'Unknown size' : formatBytes(item.bytes) }}</span
        ><span class="badge" :class="item.classification">{{ item.classification }}</span
        ><span>{{ item.reclaimable ? 'Candidate' : 'In use' }}</span>
        <p>{{ item.reason }}</p>
      </div>
    </div>
    <p v-if="previewError" class="preview-error" role="alert">{{ previewError }}</p>
    <section v-if="preview" class="preview" aria-live="polite">
      <header>
        <div>
          <p class="eyebrow">Exact preview</p>
          <h3>{{ preview.resources.length }} reclaimable candidates</h3>
        </div>
        <strong>≈ {{ formatBytes(preview.estimatedBytes) }}</strong>
      </header>
      <p>{{ preview.warnings.join(' ') }}</p>
      <ul>
        <li v-for="item in preview.resources" :key="`${item.kind}:${item.id}`">
          <code>{{ item.kind }}/{{ item.id }}</code
          ><span
            >{{ item.bytes === undefined ? 'unknown size' : formatBytes(item.bytes) }} ·
            {{ item.classification }}</span
          >
        </li>
      </ul>
      <p v-if="preview.unknownSizes">
        {{ preview.unknownSizes }} candidate sizes are unknown and excluded from the estimate.
      </p>
      <strong v-if="!preview.executable">This preview cannot execute cleanup.</strong>
    </section>
  </article>
</template>

<style scoped>
.panel {
  padding: 16px;
  border: 1px solid var(--border);
  border-radius: 13px;
  background: linear-gradient(145deg, var(--panel), #0e131a);
}
.panel-head,
.preview header {
  display: flex;
  justify-content: space-between;
  gap: 16px;
  align-items: end;
}
.panel-head h2,
.preview h3 {
  margin: 4px 0 0;
}
.eyebrow {
  margin: 0;
  color: var(--accent);
  font-size: 10px;
  font-weight: 800;
  letter-spacing: 0.13em;
  text-transform: uppercase;
}
.boundary {
  padding: 5px 8px;
  border: 1px solid rgba(241, 199, 91, 0.28);
  border-radius: 999px;
  color: var(--yellow);
  font-size: 9px;
}
.filters {
  display: grid;
  grid-template-columns: 1fr 0.8fr auto;
  gap: 10px;
  align-items: end;
  margin: 16px 0;
}
.filters label {
  display: grid;
  gap: 5px;
  color: var(--soft);
  font-size: 9px;
  text-transform: uppercase;
}
.filters select,
.filters button,
.state button {
  padding: 8px;
  border: 1px solid var(--border);
  border-radius: 7px;
  background: var(--panel-2);
  color: var(--text);
}
.inventory {
  border: 1px solid var(--border);
  border-radius: 8px;
  overflow: hidden;
}
.resource {
  display: grid;
  grid-template-columns: 1.5fr 0.7fr 0.6fr 0.5fr;
  gap: 12px;
  align-items: center;
  padding: 10px 12px;
  border-top: 1px solid var(--border);
  color: var(--muted);
  font-size: 10px;
}
.resource:first-child {
  border-top: 0;
}
.resource > div {
  display: grid;
  gap: 2px;
}
.resource strong {
  color: var(--text);
}
.resource code {
  overflow: hidden;
  text-overflow: ellipsis;
}
.resource p {
  grid-column: 1/-1;
  margin: 0;
  color: var(--soft);
}
.badge {
  justify-self: start;
  padding: 3px 6px;
  border-radius: 999px;
  text-transform: capitalize;
}
.badge.exclusive {
  background: rgba(92, 207, 159, 0.1);
  color: var(--green);
}
.badge.shared,
.badge.estimated {
  background: rgba(241, 199, 91, 0.1);
  color: var(--yellow);
}
.badge.unknown {
  background: rgba(255, 255, 255, 0.06);
  color: var(--muted);
}
.preview {
  margin-top: 14px;
  padding: 14px;
  border: 1px solid rgba(241, 199, 91, 0.3);
  border-radius: 9px;
  background: rgba(241, 199, 91, 0.04);
}
.preview p {
  color: var(--muted);
  font-size: 10px;
}
.preview ul {
  display: grid;
  gap: 6px;
  padding: 0;
  list-style: none;
}
.preview li {
  display: flex;
  justify-content: space-between;
  gap: 12px;
}
.state,
.warning {
  padding: 20px;
  border: 1px dashed var(--border);
  border-radius: 8px;
  color: var(--muted);
  text-align: center;
}
.warning {
  border-color: rgba(241, 199, 91, 0.3);
  color: var(--yellow);
}
.preview-error {
  padding: 9px;
  border: 1px solid rgba(255, 115, 115, 0.3);
  border-radius: 7px;
  color: var(--red);
}
@media (max-width: 800px) {
  .resource {
    grid-template-columns: 1fr 1fr;
  }
  .filters {
    grid-template-columns: 1fr 1fr;
  }
  .filters button {
    grid-column: 1/-1;
  }
  .panel-head {
    display: grid;
  }
}
@media (max-width: 520px) {
  .filters,
  .resource {
    grid-template-columns: 1fr;
  }
  .resource p {
    grid-column: auto;
  }
  .preview li {
    display: grid;
  }
}
</style>
