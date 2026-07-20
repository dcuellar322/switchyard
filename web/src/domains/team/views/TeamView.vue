<script setup lang="ts">
import { useTeamView } from '../composables/useTeamView'

const {
  publishers,
  bundles,
  policy,
  registry,
  publisherName,
  publicKey,
  publisherReviewed,
  selectedBundle,
  bundleReviewed,
  fileError,
  trust,
  install,
  error,
  loading,
  emptyConfiguration,
  selectBundle,
} = useTeamView()
</script>

<template>
  <section class="team-view" aria-labelledby="team-title">
    <header>
      <p>Local team configuration</p>
      <h1 id="team-title">Team</h1>
      <span
        >Share reviewed templates and policy without syncing source code, project paths, presence,
        or user accounts.</span
      >
    </header>
    <p v-if="error" class="error" role="alert">{{ error.message }}</p>
    <div v-if="loading" class="loading-state" aria-live="polite">
      Loading local team trust and policy…
    </div>
    <div v-else class="grid">
      <article class="panel orientation" :class="{ 'orientation--empty': emptyConfiguration }">
        <div>
          <p>{{ emptyConfiguration ? 'Nothing installed yet' : 'Configuration status' }}</p>
          <h2>
            {{
              emptyConfiguration
                ? 'No shared team configuration is installed'
                : 'Shared configuration is active'
            }}
          </h2>
        </div>
        <p v-if="emptyConfiguration">
          This page is for signed configuration exchange, not a live member directory. Start by
          trusting a publisher key below, then install a reviewed signed bundle or use
          <code>switchyard team sync</code>.
        </p>
        <p v-else>
          Trusted publishers and installed bundles apply only to this machine. Every imported policy
          remains visible and restrictive by default.
        </p>
      </article>
      <article class="panel">
        <div class="panel-head">
          <div>
            <p>Trust store</p>
            <h2>Publisher identities</h2>
          </div>
          <span>{{ publishers.data.value?.length ?? 0 }}</span>
        </div>
        <p class="description">
          Trust one exact public key only after confirming it through an independent channel.
        </p>
        <ul v-if="publishers.data.value?.length">
          <li v-for="publisher in publishers.data.value" :key="publisher.id">
            <strong>{{ publisher.name }}</strong
            ><code>{{ publisher.id }}</code>
          </li>
        </ul>
        <p v-else class="empty">No publisher keys are trusted.</p>
        <form @submit.prevent="trust.mutate()">
          <label>Name<input v-model.trim="publisherName" required maxlength="128" /></label
          ><label
            >Base64 Ed25519 public key<input
              v-model.trim="publicKey"
              required
              autocomplete="off" /></label
          ><label class="confirm"
            ><input v-model="publisherReviewed" type="checkbox" /><span
              >I verified this exact key out of band.</span
            ></label
          ><button
            :disabled="!publisherReviewed || !publisherName || !publicKey || trust.isPending.value"
          >
            Trust exact key
          </button>
        </form>
      </article>
      <article class="panel">
        <div class="panel-head">
          <div>
            <p>Verified configuration</p>
            <h2>Signed bundles</h2>
          </div>
          <span>{{ bundles.data.value?.length ?? 0 }}</span>
        </div>
        <p class="description">
          Project templates, policy packs, registries, and enterprise configuration are portable and
          contain no secrets or host paths.
        </p>
        <ul v-if="bundles.data.value?.length">
          <li v-for="bundle in bundles.data.value" :key="bundle.metadata.id">
            <strong>{{ bundle.metadata.name }}</strong
            ><span>{{ bundle.kind }} · {{ bundle.metadata.version }}</span
            ><code>{{ bundle.metadata.publisherId }}</code>
          </li>
        </ul>
        <p v-else class="empty">No signed bundles are installed.</p>
        <form @submit.prevent="install.mutate()">
          <label
            >Signed JSON bundle<input
              type="file"
              accept="application/json,.json"
              @change="selectBundle"
          /></label>
          <p v-if="selectedBundle" class="review">
            Review: <strong>{{ selectedBundle.metadata?.id }}</strong> · {{ selectedBundle.kind }} ·
            {{ selectedBundle.metadata?.publisherId }}
          </p>
          <p v-if="fileError" class="error">{{ fileError }}</p>
          <label class="confirm"
            ><input v-model="bundleReviewed" type="checkbox" /><span
              >I reviewed the kind, publisher ID, version, and payload.</span
            ></label
          ><button :disabled="!bundleReviewed || !selectedBundle || install.isPending.value">
            Verify signature and install
          </button>
        </form>
      </article>
      <article class="panel policy">
        <div class="panel-head">
          <div>
            <p>Restrictive intersection</p>
            <h2>Effective policy</h2>
          </div>
          <span>{{ policy.data.value?.sourceBundleIds.length ?? 0 }} sources</span>
        </div>
        <dl v-if="policy.data.value">
          <div>
            <dt>Remote capabilities</dt>
            <dd>{{ policy.data.value.allowedRemoteCapabilities.join(', ') || 'denied' }}</dd>
          </div>
          <div>
            <dt>Remote actions</dt>
            <dd>{{ policy.data.value.allowedRemoteActions.join(', ') || 'denied' }}</dd>
          </div>
          <div>
            <dt>Plugin publishers</dt>
            <dd>
              {{
                policy.data.value.allowedPluginPublishers.join(', ') ||
                (policy.data.value.sourceBundleIds.length ? 'denied' : 'no team policy')
              }}
            </dd>
          </div>
          <div>
            <dt>Anonymous metrics</dt>
            <dd>{{ policy.data.value.telemetryAllowed ? 'allowed with opt-in' : 'denied' }}</dd>
          </div>
        </dl>
      </article>
      <article class="panel registry">
        <div class="panel-head">
          <div>
            <p>Signed metadata only</p>
            <h2>Curated plugin registry</h2>
          </div>
          <span>{{ registry.data.value?.length ?? 0 }}</span>
        </div>
        <p class="description">
          Registry entries never auto-install or grant trust. Downloads still require SHA-256
          verification and local capability review.
        </p>
        <div v-if="registry.data.value?.length" class="registry-list">
          <article v-for="plugin in registry.data.value" :key="plugin.id">
            <div>
              <strong>{{ plugin.name }}</strong
              ><span>{{ plugin.version }} · {{ plugin.publisher }}</span>
            </div>
            <p>{{ plugin.summary }}</p>
            <code>{{ plugin.sha256 }}</code>
          </article>
        </div>
        <p v-else class="empty">No registry entries are allowed by installed policy.</p>
      </article>
      <article class="panel sync">
        <div class="panel-head">
          <div>
            <p>Configuration only</p>
            <h2>Encrypted sync</h2>
          </div>
          <span>age X25519</span>
        </div>
        <p>
          Use <code>switchyard team sync</code> to export, preview, and import encrypted files.
          Fleet credentials, projects, paths, logs, operations, and secrets are never included.
          Decryption stays in the CLI; every signature is verified before an explicit import.
        </p>
      </article>
    </div>
  </section>
</template>

<style scoped>
.team-view {
  max-width: 1300px;
  margin: 0 auto;
  padding: 28px;
}
.team-view > header p,
.panel-head p,
.orientation > div > p {
  margin: 0;
  color: var(--accent);
  font-size: 10px;
  font-weight: 800;
  letter-spacing: 0.13em;
  text-transform: uppercase;
}
.team-view > header h1 {
  margin: 6px 0;
  font-size: 30px;
}
.team-view > header span,
.description,
.empty,
.sync p,
.orientation > p {
  color: var(--muted);
}
.loading-state {
  margin-top: 22px;
  padding: 42px;
  border: 1px solid var(--border);
  border-radius: 13px;
  background: var(--panel);
  color: var(--muted);
  text-align: center;
}
.grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 14px;
  margin-top: 22px;
}
.panel {
  min-width: 0;
  padding: 18px;
  border: 1px solid var(--border);
  border-radius: 13px;
  background: linear-gradient(145deg, var(--panel), #0d1219);
}
.orientation {
  grid-column: 1/-1;
  display: grid;
  grid-template-columns: minmax(260px, 0.8fr) minmax(320px, 1.2fr);
  align-items: center;
  gap: 24px;
  border-color: rgba(120, 166, 255, 0.28);
  background: linear-gradient(135deg, rgba(120, 166, 255, 0.1), rgba(158, 123, 255, 0.05));
}
.orientation--empty {
  border-style: dashed;
}
.orientation h2 {
  margin: 5px 0 0;
  font-size: 19px;
}
.orientation > p {
  margin: 0;
  line-height: 1.6;
}
.panel-head {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 12px;
}
.panel-head h2 {
  margin: 4px 0 0;
  font-size: 17px;
}
.panel-head > span {
  padding: 4px 7px;
  border: 1px solid var(--border);
  border-radius: 99px;
  color: var(--muted);
  font-size: 9px;
}
.panel ul {
  display: grid;
  gap: 7px;
  margin: 13px 0;
  padding: 0;
  list-style: none;
}
.panel li {
  display: grid;
  gap: 3px;
  padding: 9px;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--panel-2);
}
.panel li span,
.panel code {
  color: var(--soft);
  overflow-wrap: anywhere;
}
.panel code {
  font-size: 10px;
}
form {
  display: grid;
  gap: 10px;
  margin-top: 15px;
  padding-top: 14px;
  border-top: 1px solid var(--border);
}
form label:not(.confirm) {
  display: grid;
  gap: 5px;
  color: var(--muted);
}
input:not([type='checkbox']) {
  width: 100%;
  padding: 9px;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: #0b1017;
  color: var(--text);
}
.confirm {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  color: var(--muted);
}
button {
  width: max-content;
  min-height: 36px;
  padding: 8px 11px;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--panel-2);
  color: var(--text);
}
button:disabled {
  opacity: 0.5;
}
.error {
  padding: 10px;
  border: 1px solid rgba(255, 115, 115, 0.3);
  border-radius: 8px;
  color: var(--red);
}
.review {
  color: var(--yellow);
}
.policy,
.registry,
.sync {
  grid-column: 1/-1;
}
.policy dl {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 8px;
}
.policy dl div {
  padding: 10px;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--panel-2);
}
.policy dt {
  color: var(--soft);
  font-size: 9px;
  text-transform: uppercase;
}
.policy dd {
  margin: 5px 0 0;
}
.registry-list {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(260px, 1fr));
  gap: 9px;
}
.registry-list article {
  padding: 11px;
  border: 1px solid var(--border);
  border-radius: 9px;
  background: var(--panel-2);
}
.registry-list article div {
  display: grid;
  gap: 3px;
}
.registry-list article span,
.registry-list p {
  color: var(--muted);
}
@media (max-width: 760px) {
  .team-view {
    padding: 20px 18px;
  }
  .grid {
    grid-template-columns: 1fr;
  }
  .orientation {
    grid-template-columns: 1fr;
  }
  .policy dl {
    grid-template-columns: 1fr;
  }
}
</style>
