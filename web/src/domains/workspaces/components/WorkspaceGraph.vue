<script setup lang="ts">
import { computed } from 'vue'

import type { Workspace } from '../../../api/generated/types.gen'

const props = defineProps<{ workspace: Workspace; names: Record<string, string> }>()

const layers = computed(() => {
  const remaining = new Set(props.workspace.members.map((member) => member.projectId))
  const result: Array<Array<(typeof props.workspace.members)[number]>> = []
  while (remaining.size > 0) {
    const layer = props.workspace.members
      .filter((member) => remaining.has(member.projectId))
      .filter((member) =>
        props.workspace.dependencies
          .filter((edge) => edge.projectId === member.projectId)
          .every((edge) => !remaining.has(edge.dependsOnProjectId)),
      )
      .sort((left, right) => left.order - right.order)
    if (layer.length === 0) break
    result.push(layer)
    layer.forEach((member) => remaining.delete(member.projectId))
  }
  return result
})

function label(projectId: string): string {
  return props.names[projectId] ?? projectId
}
</script>

<template>
  <section class="workspace-graph" aria-labelledby="workspace-graph-title">
    <header>
      <div>
        <p>Dependency graph</p>
        <h2 id="workspace-graph-title">Start order</h2>
      </div>
      <span>{{ workspace.members.length }} projects · {{ layers.length }} stages</span>
    </header>
    <div class="layers" role="list" aria-label="Dependency-ordered workspace stages">
      <div v-for="(layer, index) in layers" :key="index" class="layer" role="listitem">
        <div class="stage">
          <span>{{ index + 1 }}</span
          >Stage {{ index + 1 }}
        </div>
        <div class="nodes">
          <article
            v-for="member in layer"
            :key="member.projectId"
            class="node"
            :class="`node--${member.status}`"
          >
            <div>
              <strong>{{ label(member.projectId) }}</strong>
              <span
                >{{ member.role }}<template v-if="member.healthGate"> · health gate</template></span
              >
            </div>
            <span class="state"><i aria-hidden="true"></i>{{ member.status }}</span>
            <small v-if="member.message">{{ member.message }}</small>
          </article>
        </div>
        <span v-if="index < layers.length - 1" class="connector" aria-hidden="true">↓</span>
      </div>
    </div>
  </section>
</template>

<style scoped>
.workspace-graph {
  border: 1px solid var(--border);
  border-radius: 15px;
  background: var(--panel);
  overflow: hidden;
}
header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 17px 19px;
  border-bottom: 1px solid var(--border);
}
header p {
  margin: 0 0 3px;
  color: var(--accent);
  font-size: 10px;
  font-weight: 800;
  letter-spacing: 0.12em;
  text-transform: uppercase;
}
h2 {
  margin: 0;
  font-size: 17px;
}
header > span {
  color: var(--soft);
  font-size: 11px;
}
.layers {
  display: grid;
  gap: 3px;
  padding: 18px;
}
.layer {
  position: relative;
  display: grid;
  grid-template-columns: 82px 1fr;
  gap: 13px;
  padding-bottom: 20px;
}
.layer:last-child {
  padding-bottom: 0;
}
.stage {
  padding-top: 14px;
  color: var(--soft);
  font-size: 10px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.08em;
}
.stage span {
  display: inline-grid;
  place-items: center;
  width: 20px;
  height: 20px;
  margin-right: 6px;
  border: 1px solid var(--border);
  border-radius: 6px;
  color: var(--accent);
}
.nodes {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(190px, 1fr));
  gap: 9px;
}
.node {
  display: grid;
  grid-template-columns: 1fr auto;
  gap: 8px;
  padding: 13px;
  border: 1px solid var(--border);
  border-radius: 11px;
  background: var(--panel-2);
}
.node strong,
.node span {
  display: block;
}
.node div > span {
  margin-top: 4px;
  color: var(--soft);
  font-size: 10px;
}
.state {
  align-self: start;
  color: var(--muted);
  font-size: 10px;
  text-transform: capitalize;
  white-space: nowrap;
}
.state i {
  display: inline-block;
  width: 7px;
  height: 7px;
  margin-right: 5px;
  border-radius: 50%;
  background: var(--soft);
}
.node--running .state i,
.node--stopped .state i {
  background: var(--green);
}
.node--starting .state i,
.node--checking_health .state i,
.node--stopping .state i {
  background: var(--yellow);
}
.node--start_failed .state i,
.node--stop_failed .state i,
.node--rollback_failed .state i {
  background: var(--red);
}
.node small {
  grid-column: 1 / -1;
  color: var(--muted);
  line-height: 1.4;
}
.connector {
  position: absolute;
  left: 37px;
  bottom: 1px;
  color: var(--border-strong);
}
@media (max-width: 720px) {
  header {
    align-items: flex-start;
    gap: 10px;
  }
  .layer {
    grid-template-columns: 1fr;
  }
  .connector {
    display: none;
  }
  .stage {
    padding-top: 0;
  }
}
</style>
