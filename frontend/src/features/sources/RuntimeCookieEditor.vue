<script lang="ts">
import { defineComponent, type PropType } from "vue";
import {
  getSourceRuntimeCookies,
  replaceSourceRuntimeCookies,
  revealSourceRuntimeCookies,
  type RuntimeCookie,
  type RuntimeCookieMetadata,
} from "../../api/sources";
import AppButton from "../../ui/components/AppButton.vue";

export default defineComponent({
  name: "RuntimeCookieEditor",
  components: { AppButton },
  props: { sourceId: { type: String as PropType<string>, required: true } },
  data() {
    return {
      cookies: [] as RuntimeCookieMetadata[],
      revealedCookies: null as RuntimeCookie[] | null,
      currentPassword: "",
      loading: true,
      busy: "" as "" | "reveal" | "save",
      error: "",
      notice: "",
    };
  },
  async mounted() { await this.load(); },
  beforeUnmount() { this.clearSecrets(); },
  methods: {
    async load() {
      this.loading = true;
      this.error = "";
      try { this.cookies = (await getSourceRuntimeCookies(this.sourceId)).cookies; }
      catch { this.error = this.$t("sources.interaction.cookies.loadFailed"); }
      finally { this.loading = false; }
    },
    async reveal() {
      if (!this.currentPassword || this.busy) return;
      this.busy = "reveal";
      this.error = "";
      this.notice = "";
      try {
        this.revealedCookies = (await revealSourceRuntimeCookies(this.sourceId, this.currentPassword)).cookies;
      } catch {
        this.error = this.$t("sources.interaction.cookies.passwordFailed");
      } finally {
        this.currentPassword = "";
        this.busy = "";
      }
    },
    async save() {
      if (!this.currentPassword || !this.revealedCookies || this.busy) return;
      this.busy = "save";
      this.error = "";
      this.notice = "";
      try {
        this.cookies = (await replaceSourceRuntimeCookies(this.sourceId, this.currentPassword, this.revealedCookies)).cookies;
        this.revealedCookies = null;
        this.notice = this.$t("sources.interaction.cookies.saved");
      } catch {
        this.error = this.$t("sources.interaction.cookies.saveFailed");
      } finally {
        this.currentPassword = "";
        this.busy = "";
      }
    },
    clearSecrets() {
      this.currentPassword = "";
      this.revealedCookies = null;
    },
  },
});
</script>

<template>
  <section class="runtime-cookies">
    <h3>{{ $t("sources.interaction.cookies.title") }}</h3>
    <p>{{ $t("sources.interaction.cookies.description") }}</p>
    <p v-if="loading" aria-busy="true">{{ $t("sources.interaction.cookies.loading") }}</p>
    <p v-if="error" class="cookie-message error" role="alert">{{ error }}</p>
    <p v-if="notice" class="cookie-message" role="status">{{ notice }}</p>

    <div v-if="!loading && !revealedCookies" class="cookie-summary">
      <p v-if="!cookies.length" class="empty">{{ $t("sources.interaction.cookies.empty") }}</p>
      <article v-for="cookie in cookies" :key="cookie.scope">
        <strong>{{ cookie.scope }}</strong>
        <span>{{ cookie.names.join(", ") }}</span>
      </article>
    </div>

    <div v-if="revealedCookies" class="cookie-editor">
      <div v-for="(cookie, index) in revealedCookies" :key="index" class="cookie-scope">
        <label><span>{{ $t("sources.interaction.cookies.scope") }}</span><input v-model="cookie.scope" type="url" autocomplete="off" spellcheck="false"></label>
        <label><span>{{ $t("sources.interaction.cookies.header") }}</span><textarea v-model="cookie.header" rows="3" autocomplete="off" spellcheck="false" /></label>
        <button class="remove-scope" type="button" @click="revealedCookies.splice(index, 1)">{{ $t("sources.interaction.cookies.remove") }}</button>
      </div>
      <button class="add-scope" type="button" @click="revealedCookies.push({ scope: '', header: '' })">{{ $t("sources.interaction.cookies.add") }}</button>
    </div>

    <label class="password-field">
      <span>{{ $t("sources.interaction.cookies.currentPassword") }}</span>
      <input v-model="currentPassword" type="password" autocomplete="current-password">
    </label>
    <p class="security-note">{{ revealedCookies ? $t("sources.interaction.cookies.saveWarning") : $t("sources.interaction.cookies.revealWarning") }}</p>
    <div class="cookie-actions">
      <AppButton v-if="!revealedCookies" data-action="reveal" variant="secondary" :busy="busy === 'reveal'" :disabled="!currentPassword" @click="reveal">{{ $t("sources.interaction.cookies.reveal") }}</AppButton>
      <template v-else>
        <AppButton variant="quiet" :disabled="Boolean(busy)" @click="clearSecrets">{{ $t("sources.cancel") }}</AppButton>
        <AppButton data-action="save" :busy="busy === 'save'" :disabled="!currentPassword" @click="save">{{ $t("sources.interaction.cookies.save") }}</AppButton>
      </template>
    </div>
  </section>
</template>

<style scoped>
.runtime-cookies { display: grid; gap: .7rem; margin-top: .5rem; padding-top: 1.25rem; border-top: 1px solid var(--color-border); }
h3 { margin: 0; font-family: var(--font-literary); }.runtime-cookies > p { margin: 0; color: var(--color-ink-muted); line-height: 1.5; }
.cookie-summary, .cookie-editor { display: grid; gap: .65rem; }.cookie-summary article { min-width: 0; display: grid; gap: .3rem; padding: .7rem .8rem; border: 1px solid var(--color-border); border-radius: var(--radius-md); }.cookie-summary strong { overflow-wrap: anywhere; }.cookie-summary span, .empty, .security-note { color: var(--color-ink-muted); font-size: .82rem; overflow-wrap: anywhere; }
.cookie-scope, .cookie-scope label, .password-field { display: grid; gap: .35rem; }.cookie-scope { padding: .75rem; border: 1px solid var(--color-border); border-radius: var(--radius-md); }.cookie-scope label > span, .password-field > span { font-weight: 700; }.cookie-scope input, .cookie-scope textarea, .password-field input { width: 100%; min-width: 0; border: 1px solid var(--color-border); border-radius: var(--radius-md); padding: .65rem .7rem; background: white; color: var(--color-ink); font: inherit; }.cookie-scope input, .password-field input { min-height: 2.75rem; }.cookie-scope textarea { resize: vertical; overflow-wrap: anywhere; }
.add-scope, .remove-scope { min-height: 2.75rem; border: 1px solid var(--color-border); border-radius: var(--radius-md); padding: .6rem .75rem; background: transparent; color: var(--color-accent); font-weight: 700; cursor: pointer; }.remove-scope { color: var(--color-danger); text-align: start; }.cookie-actions { display: flex; justify-content: flex-end; gap: .5rem; }.cookie-message { padding: .7rem; border-radius: var(--radius-md); background: var(--color-paper-muted); }.cookie-message.error { background: #f8e4df; color: var(--color-danger); }
@media (max-width: 38rem) { .cookie-actions { display: grid; }.cookie-actions :deep(.app-button) { width: 100%; } }
</style>
