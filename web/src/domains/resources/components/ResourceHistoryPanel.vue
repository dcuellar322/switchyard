<script setup lang="ts">
import { computed, ref } from 'vue'

import type { MetricHistory, ResourceProjectSnapshot } from '../../../api/generated/types.gen'
import { formatBytes } from '../../../lib/format'

const props = defineProps<{
  projects: Array<ResourceProjectSnapshot>
  projectId: string
  serviceId: string
  range: '1h' | '24h' | '7d'
  history?: MetricHistory
  pending: boolean
  error: boolean
}>()
const emit = defineEmits<{
  selectProject: [value: string]
  selectService: [value: string]
  selectRange: [value: '1h' | '24h' | '7d']
  retry: []
}>()
const metric = ref<'cpu' | 'memory'>('cpu')
const project = computed(() => props.projects.find((item) => item.projectId === props.projectId))
const available = (point: MetricHistory['points'][number]) =>
  metric.value === 'cpu' ? point.cpuAvailable : point.memoryAvailable
const metricValue = (point: MetricHistory['points'][number]) =>
  metric.value === 'cpu' ? point.cpuPercent : point.memoryBytes
const values = computed(() => (props.history?.points ?? []).filter(available).map(metricValue))
const peak = computed(() => Math.max(...values.value, 1))
const unavailableCount = computed(
  () => (props.history?.points ?? []).filter((point) => !available(point)).length,
)
const polylines = computed(() => {
  const points = props.history?.points ?? []
  const segments: Array<string> = []
  let current: Array<string> = []
  points.forEach((point, index) => {
    if (!available(point)) {
      if (current.length) segments.push(current.join(' '))
      current = []
      return
    }
    const x = points.length < 2 ? 0 : (index / (points.length - 1)) * 100
    current.push(`${x},${38 - (metricValue(point) / peak.value) * 34}`)
  })
  if (current.length) segments.push(current.join(' '))
  return segments
})
</script>

<template>
  <article class="panel history">
    <header class="panel-head">
      <div>
        <p class="eyebrow">Retained history</p>
        <h2>Time-series evidence</h2>
      </div>
      <span v-if="history"
        >{{ history.points.length }} points · {{ history.resolutionSeconds || 'exact' }}s tier</span
      >
    </header>
    <div class="controls">
      <label
        >Project<select
          :value="projectId"
          @change="emit('selectProject', ($event.target as HTMLSelectElement).value)"
        >
          <option v-for="item in projects" :key="item.projectId" :value="item.projectId">
            {{ item.name }}
          </option>
        </select></label
      >
      <label
        >Service<select
          :value="serviceId"
          @change="emit('selectService', ($event.target as HTMLSelectElement).value)"
        >
          <option value="">Whole project</option>
          <option
            v-for="service in project?.services ?? []"
            :key="service.serviceId"
            :value="service.serviceId"
          >
            {{ service.serviceId }}
          </option>
        </select></label
      >
      <label
        >Metric<select v-model="metric">
          <option value="cpu">CPU</option>
          <option value="memory">Memory</option>
        </select></label
      >
      <label
        >Range<select
          :value="range"
          @change="
            emit('selectRange', ($event.target as HTMLSelectElement).value as '1h' | '24h' | '7d')
          "
        >
          <option value="1h">1 hour</option>
          <option value="24h">24 hours</option>
          <option value="7d">7 days</option>
        </select></label
      >
    </div>
    <div v-if="pending" class="chart-state" aria-live="polite">Loading retained samples…</div>
    <div v-else-if="error" class="chart-state" role="alert">
      History could not be read. <button type="button" @click="emit('retry')">Retry</button>
    </div>
    <div v-else-if="!history?.points.length" class="chart-state">
      No retained samples exist in this range yet.
    </div>
    <template v-else>
      <svg
        class="chart"
        viewBox="0 0 100 40"
        preserveAspectRatio="none"
        role="img"
        :aria-label="`${metric === 'cpu' ? 'CPU' : 'Memory'} history for ${serviceId || project?.name}`"
      >
        <path d="M0 38 H100" />
        <polyline v-for="(line, index) in polylines" :key="index" :points="line" />
      </svg>
      <p class="chart-summary">
        Peak {{ metric === 'cpu' ? `${peak.toFixed(1)}%` : formatBytes(peak) }}.
        {{ unavailableCount }} unavailable
        {{ unavailableCount === 1 ? 'sample remains a gap' : 'samples remain gaps' }}; unavailable
        values are never presented as zero.
      </p>
      <details>
        <summary>Accessible data table</summary>
        <div class="history-table">
          <table>
            <thead>
              <tr>
                <th>Observed</th>
                <th>Average</th>
                <th>Peak</th>
                <th>Samples</th>
                <th>Quality</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="point in history.points" :key="point.timestamp">
                <td>{{ new Date(point.timestamp).toLocaleString() }}</td>
                <td>
                  {{
                    !available(point)
                      ? '—'
                      : metric === 'cpu'
                        ? `${point.cpuPercent.toFixed(1)}%`
                        : formatBytes(point.memoryBytes)
                  }}
                </td>
                <td>
                  {{
                    !available(point)
                      ? '—'
                      : metric === 'cpu'
                        ? `${point.cpuMaxPercent.toFixed(1)}%`
                        : formatBytes(point.memoryMaxBytes)
                  }}
                </td>
                <td>{{ point.sampleCount }}</td>
                <td>
                  {{ !available(point) ? 'Unavailable' : point.partial ? 'Partial' : 'Complete' }}
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </details>
    </template>
  </article>
</template>

<style scoped>
.panel {
  padding: 16px;
  border: 1px solid var(--border);
  border-radius: 13px;
  background: linear-gradient(145deg, var(--panel), #0e131a);
}
.panel-head {
  display: flex;
  justify-content: space-between;
  gap: 16px;
  align-items: end;
}
.panel-head h2 {
  margin: 4px 0 0;
}
.panel-head span,
.chart-summary {
  color: var(--muted);
  font-size: 10px;
}
.eyebrow {
  margin: 0;
  color: var(--accent);
  font-size: 10px;
  font-weight: 800;
  letter-spacing: 0.13em;
  text-transform: uppercase;
}
.controls {
  display: grid;
  grid-template-columns: 1.2fr 1fr 0.7fr 0.7fr;
  gap: 10px;
  margin: 16px 0;
}
.controls label {
  display: grid;
  gap: 5px;
  color: var(--soft);
  font-size: 9px;
  text-transform: uppercase;
}
.controls select {
  padding: 8px;
  border: 1px solid var(--border);
  border-radius: 7px;
  background: var(--panel-2);
  color: var(--text);
}
.chart {
  width: 100%;
  height: 190px;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: linear-gradient(180deg, rgba(120, 166, 255, 0.05), transparent);
}
.chart path {
  stroke: var(--border);
  stroke-width: 0.3;
}
.chart polyline {
  fill: none;
  stroke: var(--accent);
  stroke-width: 1.2;
  vector-effect: non-scaling-stroke;
}
.chart-state {
  display: grid;
  place-items: center;
  min-height: 190px;
  border: 1px dashed var(--border);
  border-radius: 8px;
  color: var(--muted);
}
.chart-state button {
  color: var(--accent);
}
details {
  margin-top: 10px;
  color: var(--muted);
}
summary {
  cursor: pointer;
}
.history-table {
  overflow: auto;
}
table {
  width: 100%;
  margin-top: 8px;
  border-collapse: collapse;
}
th,
td {
  padding: 7px;
  border-top: 1px solid var(--border);
  font-size: 10px;
  text-align: left;
}
@media (max-width: 700px) {
  .panel-head {
    display: grid;
  }
  .controls {
    grid-template-columns: 1fr 1fr;
  }
  .chart {
    height: 150px;
  }
}
</style>
