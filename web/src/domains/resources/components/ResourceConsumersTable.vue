<script setup lang="ts">
import type { ResourceProjectSnapshot } from '../../../api/generated/types.gen'
import { formatBytes } from '../../../lib/format'

defineProps<{ projects: Array<ResourceProjectSnapshot>; selectedProject: string; selectedService: string }>()
const emit = defineEmits<{ select: [projectId: string, serviceId: string] }>()
</script>

<template>
  <article class="panel consumers">
    <header class="panel-head">
      <div><p class="eyebrow">Managed runtimes</p><h2>Project and service consumption</h2></div>
      <span>CPU is measured as percent of one logical core and may exceed 100%.</span>
    </header>
    <div class="table-wrap">
      <table>
        <caption class="sr-only">Current project and service resource samples</caption>
        <thead><tr><th scope="col">Consumer</th><th scope="col">CPU</th><th scope="col">Memory</th><th scope="col">Network</th><th scope="col">Disk I/O</th><th scope="col">Processes</th><th scope="col">Status</th></tr></thead>
        <tbody v-for="project in projects" :key="project.projectId">
          <tr class="project-row" :class="{ selected: project.projectId === selectedProject && !selectedService }" @click="emit('select', project.projectId, '')">
            <th scope="row"><button type="button" @click.stop="emit('select', project.projectId, '')"><strong>{{ project.name }}</strong><small>{{ project.driver }} · {{ project.state }}</small></button></th>
			<td>{{ project.metric.cpuAvailable ? `${project.metric.cpuPercent.toFixed(1)}%` : '—' }}</td>
			<td>{{ project.metric.memoryAvailable ? formatBytes(project.metric.memoryBytes) : '—' }}</td>
            <td>{{ project.metric.networkAvailable ? `↓ ${formatBytes(project.metric.networkRxBytes)} · ↑ ${formatBytes(project.metric.networkTxBytes)}` : '—' }}</td>
            <td>{{ project.metric.diskAvailable ? `R ${formatBytes(project.metric.diskReadBytes)} · W ${formatBytes(project.metric.diskWriteBytes)}` : '—' }}</td>
            <td>{{ project.metric.processCount }}</td>
            <td><span v-if="project.warnings.length" class="status warn">{{ project.warnings.length }} budget warning</span><span v-else-if="project.metric.partial" class="status partial">Partial</span><span v-else class="status">Observed</span></td>
          </tr>
          <tr v-for="service in project.services" :key="`${project.projectId}:${service.serviceId}`" class="service-row" :class="{ selected: project.projectId === selectedProject && service.serviceId === selectedService }" @click="emit('select', project.projectId, service.serviceId)">
            <th scope="row"><button type="button" @click.stop="emit('select', project.projectId, service.serviceId)"><span aria-hidden="true">↳</span> {{ service.serviceId }}<small>Service</small></button></th>
			<td>{{ service.metric.cpuAvailable ? `${service.metric.cpuPercent.toFixed(1)}%` : '—' }}</td><td>{{ service.metric.memoryAvailable ? formatBytes(service.metric.memoryBytes) : '—' }}</td>
            <td>{{ service.metric.networkAvailable ? `↓ ${formatBytes(service.metric.networkRxBytes)} · ↑ ${formatBytes(service.metric.networkTxBytes)}` : '—' }}</td>
            <td>{{ service.metric.diskAvailable ? `R ${formatBytes(service.metric.diskReadBytes)} · W ${formatBytes(service.metric.diskWriteBytes)}` : '—' }}</td>
            <td>{{ service.metric.processCount }}</td><td><span class="status" :class="{ partial: service.metric.partial }">{{ service.metric.partial ? 'Partial' : 'Observed' }}</span></td>
          </tr>
        </tbody>
      </table>
    </div>
  </article>
</template>

<style scoped>
.panel{padding:16px;border:1px solid var(--border);border-radius:13px;background:linear-gradient(145deg,var(--panel),#0e131a)}.panel-head{display:flex;justify-content:space-between;gap:20px;align-items:end;margin-bottom:14px}.panel-head h2{margin:4px 0 0}.panel-head>span{max-width:430px;color:var(--muted);font-size:10px}.eyebrow{margin:0;color:var(--accent);font-size:10px;font-weight:800;letter-spacing:.13em;text-transform:uppercase}.table-wrap{overflow:auto;border:1px solid var(--border);border-radius:9px}table{width:100%;border-collapse:collapse;min-width:920px}th,td{padding:10px 12px;border-top:1px solid var(--border);color:var(--muted);font-size:11px;text-align:left}thead th{border-top:0;background:#0b1017;color:var(--soft);font-size:9px;text-transform:uppercase}tbody:first-of-type tr:first-child>*{border-top:0}th button{display:grid;gap:2px;padding:0;border:0;background:none;color:var(--text);text-align:left;cursor:pointer}th small{color:var(--muted);font-size:9px}.service-row th{padding-left:26px}.project-row{background:rgba(255,255,255,.015)}tr:not(:first-child):hover,tr.selected{background:rgba(120,166,255,.08)}.status{display:inline-flex;padding:3px 7px;border-radius:999px;background:rgba(92,207,159,.1);color:var(--green);font-size:9px;white-space:nowrap}.status.warn{background:rgba(241,199,91,.12);color:var(--yellow)}.status.partial{background:rgba(120,166,255,.1);color:var(--accent)}.sr-only{position:absolute;width:1px;height:1px;overflow:hidden;clip:rect(0,0,0,0)}@media(max-width:700px){.panel-head{display:grid}.table-wrap{margin:0 -8px}}
</style>
