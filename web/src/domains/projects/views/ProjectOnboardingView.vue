<script setup lang="ts">
import { RouterLink } from 'vue-router'

import AIFieldReview from '../components/AIFieldReview.vue'
import AIEvidenceConsent from '../components/AIEvidenceConsent.vue'
import { useProjectOnboarding } from '../composables/useProjectOnboarding'

const {
  repositoryPath,
  allowOutsideRoots,
  settings,
  projects,
  proposal,
  assisted,
  busy,
  displayedError,
  metadata,
  runtime,
  services,
  ports,
  actions,
  command,
  scan,
  validate,
  accept,
} = useProjectOnboarding()
</script>

<template>
  <section class="onboarding" aria-labelledby="onboarding-title">
    <header class="page-heading">
      <div>
        <p class="eyebrow">Project catalog</p>
        <h1 id="onboarding-title">Bring a repository into the yard.</h1>
        <p>
          Switchyard reads known project files, shows its evidence, and waits for your approval.
        </p>
      </div>
      <span class="safety-note">No repository code is executed</span>
    </header>

    <form class="scan-form" @submit.prevent="scan">
      <label for="repository-path">Repository path</label>
      <div class="scan-form__controls">
        <input
          id="repository-path"
          v-model="repositoryPath"
          required
          autocomplete="off"
          list="project-roots"
          placeholder="/Users/you/dev/project"
        />
        <datalist id="project-roots">
          <option
            v-for="root in settings.data.value?.settings.projectRoots ?? []"
            :key="root"
            :value="root"
          />
        </datalist>
        <button type="submit" :disabled="busy">{{ busy ? 'Scanning…' : 'Scan repository' }}</button>
      </div>
      <p v-if="settings.data.value" class="root-policy">
        Approved roots: <code>{{ settings.data.value.settings.projectRoots.join(', ') }}</code>
      </p>
      <label class="outside-root"
        ><input v-model="allowOutsideRoots" type="checkbox" /><span
          ><strong>Approve this one scan outside configured roots</strong
          ><small
            >The repository is still untrusted and no repository command is executed.</small
          ></span
        ></label
      >
      <p v-if="displayedError" class="message message--error" role="alert">{{ displayedError }}</p>
    </form>

    <div v-if="proposal" class="review-grid">
      <article class="review-card review-card--summary">
        <div class="review-card__heading">
          <div>
            <p class="eyebrow">Candidate manifest</p>
            <h2>{{ metadata.name ?? 'Unnamed project' }}</h2>
          </div>
          <span class="status" :class="{ 'status--ready': proposal.validation.valid }">
            {{
              proposal.status === 'accepted'
                ? 'Trusted'
                : proposal.validation.valid
                  ? 'Ready for review'
                  : 'Needs attention'
            }}
          </span>
        </div>
        <dl class="facts">
          <div>
            <dt>Runtime</dt>
            <dd>{{ runtime.driver ?? 'Unresolved' }}</dd>
          </div>
          <div>
            <dt>Services</dt>
            <dd>{{ services.length }}</dd>
          </div>
          <div>
            <dt>Ports</dt>
            <dd>{{ ports.length }}</dd>
          </div>
          <div>
            <dt>Evidence</dt>
            <dd>{{ proposal.evidence.length }}</dd>
          </div>
        </dl>
        <div v-if="proposal.unresolved.length" class="message">
          Unresolved: <code>{{ proposal.unresolved.join(', ') }}</code>
        </div>
        <div
          v-for="validationError in proposal.validation.errors"
          :key="validationError"
          class="message message--error"
        >
          {{ validationError }}
        </div>
        <div class="review-actions">
          <button class="button--secondary" type="button" :disabled="busy" @click="validate">
            Validate again
          </button>
          <button
            type="button"
            :disabled="
              busy ||
              !proposal.validation.valid ||
              proposal.unresolved.length > 0 ||
              proposal.status !== 'proposed'
            "
            @click="accept"
          >
            Approve and trust project
          </button>
        </div>
      </article>

      <article class="review-card">
        <p class="eyebrow">Detected runtime</p>
        <h2>Services and ports</h2>
        <ul class="detected-list">
          <li v-for="service in services" :key="String(service.id)">
            <strong>{{ service.displayName || service.id }}</strong
            ><span>Declared service</span>
          </li>
          <li v-for="port in ports" :key="String(port.id)">
            <strong>{{ port.host }} → {{ port.target }}</strong
            ><span>{{ port.service }} · {{ port.id }}</span>
          </li>
          <li v-if="services.length === 0 && ports.length === 0" class="empty">
            No runtime declarations were resolved.
          </li>
        </ul>
      </article>

      <article class="review-card review-card--wide">
        <p class="eyebrow">Provenance</p>
        <h2>Evidence from repository files</h2>
        <div class="evidence-table" role="table" aria-label="Discovery evidence">
          <div class="evidence-row evidence-row--header" role="row">
            <span>Finding</span><span>Source</span><span>Confidence</span>
          </div>
          <div v-for="item in proposal.evidence" :key="item.id" class="evidence-row" role="row">
            <span
              ><strong>{{ item.kind }}</strong
              ><small>{{ item.scanner }}</small></span
            >
            <code
              >{{ item.sourcePath }}:{{ item.location.startLine
              }}<template v-if="item.location.endLine !== item.location.startLine"
                >–{{ item.location.endLine }}</template
              ></code
            >
            <span>{{ Math.round(item.confidence * 100) }}%</span>
          </div>
        </div>
      </article>

      <AIEvidenceConsent
        v-if="proposal.status === 'proposed'"
        v-model:selected-provider="assisted.selectedProvider.value"
        v-model:consented="assisted.evidenceConsented.value"
        :providers="assisted.providers.value"
        :preview="assisted.evidencePreview.value"
        :pending="busy"
        @preview="assisted.previewEvidence"
        @start="assisted.enhance"
      />

      <AIFieldReview
        :operation="assisted.operation.value"
        :run="assisted.run.value"
        :cancelling="assisted.cancelling.value"
        @cancel="assisted.cancel"
      />

      <article v-if="actions.length" class="review-card review-card--wide">
        <p class="eyebrow">Executable review</p>
        <h2>Proposed commands</h2>
        <ul class="command-list">
          <li v-for="action in actions" :key="String(action.id)">
            <span>{{ action.name }}</span
            ><code>{{ command(action) }}</code>
          </li>
        </ul>
      </article>
    </div>

    <article v-if="projects.length" class="existing-projects">
      <p class="eyebrow">Already registered</p>
      <h2>Your local projects</h2>
      <ul>
        <li v-for="project in projects" :key="project.id">
          <RouterLink
            class="project-choice"
            :to="{ name: 'project', params: { projectId: project.id } }"
            ><strong>{{ project.displayName }}</strong
            ><span>{{ project.primaryLocation }}</span
            ><em>{{ project.trustState }}</em></RouterLink
          >
        </li>
      </ul>
    </article>
  </section>
</template>

<style scoped>
.onboarding {
  padding: 42px;
  max-width: 1240px;
  margin: 0 auto;
}
.page-heading {
  display: flex;
  justify-content: space-between;
  gap: 24px;
  align-items: flex-start;
  margin-bottom: 30px;
}
.page-heading h1 {
  margin: 4px 0 8px;
  font-size: clamp(28px, 4vw, 44px);
  letter-spacing: -0.035em;
}
.page-heading p {
  margin: 0;
  color: var(--muted);
  max-width: 680px;
}
.eyebrow {
  margin: 0;
  color: var(--accent);
  text-transform: uppercase;
  letter-spacing: 0.14em;
  font-size: 10px;
  font-weight: 800;
}
.safety-note {
  padding: 8px 11px;
  border: 1px solid rgba(84, 212, 154, 0.35);
  border-radius: 999px;
  color: var(--green);
  background: rgba(84, 212, 154, 0.08);
  font-size: 12px;
  white-space: nowrap;
}
.scan-form {
  padding: 20px;
  border: 1px solid var(--border);
  border-radius: 14px;
  background: rgba(17, 22, 30, 0.85);
  margin-bottom: 24px;
}
.scan-form label {
  display: block;
  margin-bottom: 9px;
  font-weight: 700;
}
.scan-form__controls {
  display: flex;
  gap: 10px;
}
input {
  min-width: 0;
  flex: 1;
  padding: 11px 13px;
  color: var(--text);
  border: 1px solid #344157;
  border-radius: 9px;
  background: #0b1017;
}
.root-policy {
  margin: 10px 0 0;
  color: var(--muted);
  font-size: 11px;
}
.root-policy code {
  overflow-wrap: anywhere;
}
.outside-root {
  display: flex;
  align-items: flex-start;
  gap: 9px;
  margin-top: 12px;
  color: var(--muted);
}
.outside-root input {
  flex: 0 0 auto;
  width: 17px;
  height: 17px;
}
.outside-root span {
  display: grid;
  gap: 3px;
}
.outside-root strong {
  color: var(--text);
  font-size: 12px;
}
.outside-root small {
  font-size: 10px;
}
button {
  padding: 10px 15px;
  border: 0;
  border-radius: 9px;
  color: #07111f;
  background: var(--accent);
  font-weight: 800;
  cursor: pointer;
}
button:disabled {
  opacity: 0.48;
  cursor: not-allowed;
}
.button--secondary {
  border: 1px solid #40506a;
  color: var(--text);
  background: transparent;
}
.review-grid {
  display: grid;
  grid-template-columns: 1.2fr 0.8fr;
  gap: 18px;
}
.review-card,
.existing-projects {
  padding: 22px;
  border: 1px solid var(--border);
  border-radius: 14px;
  background: linear-gradient(145deg, rgba(21, 27, 36, 0.96), rgba(14, 19, 26, 0.96));
}
.review-card--wide {
  grid-column: 1/-1;
}
.review-card__heading {
  display: flex;
  justify-content: space-between;
  gap: 20px;
}
.review-card h2,
.existing-projects h2 {
  margin: 5px 0 18px;
  font-size: 20px;
}
.status {
  height: min-content;
  padding: 5px 9px;
  border-radius: 999px;
  background: rgba(241, 199, 91, 0.1);
  color: var(--yellow);
  font-size: 11px;
}
.status--ready {
  background: rgba(84, 212, 154, 0.1);
  color: var(--green);
}
.facts {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 8px;
  margin: 12px 0 18px;
}
.facts div {
  padding: 12px;
  border: 1px solid var(--border);
  border-radius: 9px;
  background: #0d1219;
}
.facts dt {
  color: var(--soft);
  font-size: 11px;
}
.facts dd {
  margin: 5px 0 0;
  font-weight: 800;
}
.message {
  margin: 10px 0;
  padding: 10px 12px;
  border-radius: 8px;
  background: rgba(241, 199, 91, 0.08);
  color: var(--yellow);
}
.message--error {
  background: rgba(255, 115, 115, 0.08);
  color: var(--red);
}
.review-actions {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  margin-top: 18px;
}
.detected-list,
.command-list,
.existing-projects ul {
  list-style: none;
  padding: 0;
  margin: 0;
  display: grid;
  gap: 8px;
}
.detected-list li,
.command-list li,
.existing-projects li {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  border-radius: 8px;
  background: #0d1219;
}
.detected-list li,
.command-list li {
  padding: 10px 12px;
}
.project-choice {
  width: 100%;
  display: grid;
  grid-template-columns: 1fr 2fr auto;
  gap: 12px;
  padding: 10px 15px;
  text-align: left;
  color: var(--text);
  background: transparent;
  text-decoration: none;
}
.project-choice span {
  text-align: left;
  overflow: hidden;
  text-overflow: ellipsis;
}
.detected-list span,
.detected-list .empty,
.existing-projects span {
  color: var(--muted);
  font-size: 12px;
}
.evidence-table {
  display: grid;
  border: 1px solid var(--border);
  border-radius: 9px;
  overflow: hidden;
}
.evidence-row {
  display: grid;
  grid-template-columns: 1fr 1.4fr 100px;
  gap: 14px;
  align-items: center;
  padding: 11px 13px;
  border-top: 1px solid var(--border);
}
.evidence-row:first-child {
  border-top: 0;
}
.evidence-row--header {
  color: var(--soft);
  background: #0d1219;
  font-size: 11px;
  text-transform: uppercase;
  letter-spacing: 0.08em;
}
.evidence-row span:first-child {
  display: grid;
  gap: 2px;
}
.evidence-row small {
  color: var(--soft);
}
.command-list li code {
  color: var(--cyan);
}
.existing-projects em {
  color: var(--green);
  font-style: normal;
}
@media (max-width: 850px) {
  .onboarding {
    padding: 26px 18px;
  }
  .page-heading {
    display: grid;
  }
  .safety-note {
    width: max-content;
  }
  .review-grid {
    grid-template-columns: 1fr;
  }
  .review-card--wide {
    grid-column: auto;
  }
  .facts {
    grid-template-columns: 1fr 1fr;
  }
  .evidence-row {
    grid-template-columns: 1fr;
  }
  .evidence-row--header {
    display: none;
  }
  .scan-form__controls {
    display: grid;
  }
}
</style>
