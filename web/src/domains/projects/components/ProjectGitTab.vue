<script setup lang="ts">
import type {
  GitState,
  ProjectEnvironment,
} from "../../../api/generated/types.gen";

defineProps<{
  git?: GitState;
  environments: Array<ProjectEnvironment>;
  environmentsPending: boolean;
  environmentsError: boolean;
  registrationPending: boolean;
  registrationError?: string;
}>();
defineEmits<{ register: [] }>();
</script>

<template>
  <article class="panel">
    <header class="panel-head">
      <div>
        <p>Read-only snapshot</p>
        <h2>Repository state</h2>
      </div>
      <span>{{
        git?.observedAt ? new Date(git.observedAt).toLocaleTimeString() : "—"
      }}</span>
    </header>
    <dl class="fact-grid">
      <div>
        <dt>Branch</dt>
        <dd>{{ git?.branch ?? "detached" }}</dd>
      </div>
      <div>
        <dt>HEAD</dt>
        <dd>
          <code>{{ git?.head?.slice(0, 12) ?? "—" }}</code>
        </dd>
      </div>
      <div>
        <dt>Ahead / behind</dt>
        <dd>{{ git?.ahead ?? 0 }} / {{ git?.behind ?? 0 }}</dd>
      </div>
      <div>
        <dt>Stashes</dt>
        <dd>{{ git?.stashes ?? 0 }}</dd>
      </div>
      <div>
        <dt>Modified</dt>
        <dd>{{ git?.changes.modified ?? 0 }}</dd>
      </div>
      <div>
        <dt>Untracked</dt>
        <dd>{{ git?.changes.untracked ?? 0 }}</dd>
      </div>
    </dl>
    <p v-if="git?.lastCommit" class="commit">
      <code>{{ git.lastCommit.shortHash }}</code> {{ git.lastCommit.subject }}
      <span>by {{ git.lastCommit.author }}</span>
    </p>
  </article>
  <article class="panel">
    <header class="panel-head">
      <div>
        <p>Parallel feature environments</p>
        <h2>Registered worktrees</h2>
      </div>
      <button
        type="button"
        :disabled="registrationPending"
        @click="$emit('register')"
      >
        {{ registrationPending ? "Registering…" : "↻ Reconcile worktrees" }}
      </button>
    </header>
    <p v-if="registrationError" class="panel-state message--error" role="alert">
      {{ registrationError }}
    </p>
    <p v-else-if="environmentsPending" class="panel-state">
      Reading durable environment registrations…
    </p>
    <p
      v-else-if="environmentsError"
      class="panel-state message--error"
      role="alert"
    >
      Worktree environments are unavailable.
    </p>
    <div v-else-if="environments.length" class="environment-list">
      <article v-for="environment in environments" :key="environment.id">
        <div>
          <strong>{{ environment.name }}</strong
          ><span>{{
            environment.primary
              ? "primary checkout"
              : environment.branch || "detached worktree"
          }}</span>
        </div>
        <code>{{ environment.hostname }}</code>
        <span
          :class="`environment-state environment-state--${environment.state}`"
          >{{ environment.state }}</span
        >
        <small
          >{{ environment.allocation.composeProjectName }} ·
          {{ environment.allocation.portLeases.length }} exact ports</small
        >
      </article>
    </div>
    <p v-else class="panel-state">
      No worktrees are registered. Reconcile after the project is trusted.
    </p>
  </article>
</template>
