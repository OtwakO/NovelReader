<script lang="ts">
import { defineComponent, type PropType } from "vue";
import {
  getSourceInteraction,
  resetSourceInteraction,
  runSourceInteractionAction,
  type BookSourceIdentity,
  type SourceInteractionActionResult,
  type SourceInteractionEffect,
  type SourceInteractionResetScope,
  type SourceInteractionView,
} from "../../api/sources";
import { ApiError } from "../../api/transport";
import AppButton from "../../ui/components/AppButton.vue";
import SourceBrowserSession from "./SourceBrowserSession.vue";
import RuntimeCookieEditor from "./RuntimeCookieEditor.vue";

type PendingReset = SourceInteractionResetScope | null;

export default defineComponent({
  name: "SourceInteractionSheet",
  components: { AppButton, RuntimeCookieEditor, SourceBrowserSession },
  props: { source: { type: Object as PropType<BookSourceIdentity>, required: true } },
  emits: ["close", "refresh-explore"],
  data() {
    return {
      view: null as SourceInteractionView | null,
      values: {} as Record<string, string>,
      effects: [] as SourceInteractionEffect[],
      loading: true,
      busyAction: "",
      error: "",
      pendingReset: null as PendingReset,
      browserEffect: null as SourceInteractionEffect | null,
    };
  },
  async mounted() {
    document.addEventListener("keydown", this.onKeydown);
    document.body.style.overflow = "hidden";
    (this.$refs.closeButton as HTMLButtonElement)?.focus();
    await this.load();
  },
  beforeUnmount() {
    document.removeEventListener("keydown", this.onKeydown);
    document.body.style.overflow = "";
  },
  methods: {
    async load() {
      this.loading = true;
      this.error = "";
      try { this.applyView(await getSourceInteraction(this.source.sourceId!)); }
      catch (cause) { this.error = cause instanceof Error ? cause.message : this.$t("sources.interaction.loadFailed"); }
      finally { this.loading = false; }
    },
    applyView(view: SourceInteractionView) {
      this.view = view;
      const values: Record<string, string> = {};
      for (const control of view.controls) {
        if (["text", "password", "input", "toggle", "select"].includes(control.type)) values[control.label] = control.value || "";
      }
      this.values = values;
    },
    async act(actionId: string) {
      if (!this.view || this.busyAction) return;
      this.busyAction = actionId;
      this.error = "";
      this.effects = [];
      try {
        const result = await runSourceInteractionAction(this.source.sourceId!, this.view.revision, actionId, this.values);
        this.applyView(result.view);
        this.effects = result.effects;
        this.applyEffects(result.effects);
      } catch (cause) {
        this.error = cause instanceof Error ? cause.message : this.$t("sources.interaction.actionFailed");
        if (cause instanceof ApiError && cause.status === 409) await this.load();
      } finally { this.busyAction = ""; }
    },
    applyEffects(effects: SourceInteractionEffect[]) {
      for (const effect of effects) {
        if (effect.type === "refresh_explore") this.$emit("refresh-explore");
        if (effect.type === "open_external" && effect.url) window.open(effect.url, "_blank", "noopener,noreferrer");
        if (effect.type === "search" && effect.message) void this.$router.push({ name: "search", query: { q: effect.message } });
        if (effect.type === "browser_required" && effect.browserRequestId) this.browserEffect = effect;
      }
    },
    async confirmReset() {
      if (!this.pendingReset || this.busyAction) return;
      const scope = this.pendingReset;
      this.busyAction = `reset-${scope}`;
      this.error = "";
      try {
        this.applyView(await resetSourceInteraction(this.source.sourceId!, scope));
        this.effects = [{ type: "notice", message: this.$t(`sources.interaction.reset.${scope}.done`) }];
        this.pendingReset = null;
      } catch (cause) { this.error = cause instanceof Error ? cause.message : this.$t("sources.interaction.resetFailed"); }
      finally { this.busyAction = ""; }
    },
    onKeydown(event: KeyboardEvent) {
      if (event.key === "Escape" && !this.browserEffect) this.$emit("close");
    },
    changeControl(actionId?: string) {
      if (actionId) void this.act(actionId);
    },
    toggleValue(label: string, options: string[] | undefined, actionId?: string) {
      const off = options?.[0] ?? "0";
      const on = options?.[1] ?? "1";
      this.values[label] = this.values[label] === on ? off : on;
      if (actionId) void this.act(actionId);
    },
    toggleChecked(label: string, options: string[] | undefined) {
      return this.values[label] === (options?.[1] ?? "1");
    },
    async browserClosed(saved: boolean, resumed?: SourceInteractionActionResult) {
      this.browserEffect = null;
      if (!saved) return;
      if (resumed) {
        this.applyView(resumed.view);
        this.effects = resumed.effects;
        this.applyEffects(resumed.effects);
      } else {
        await this.load();
        this.effects = [{ type: "notice", message: this.$t("sources.interaction.effects.loginSaved") }];
      }
      if (!resumed?.effects.some((effect) => effect.type === "refresh_explore")) this.$emit("refresh-explore");
    },
    effectText(effect: SourceInteractionEffect) {
      if (effect.type === "browser_required") return this.$t("sources.interaction.effects.browserRequired");
      if (effect.type === "refresh_explore") return this.$t("sources.interaction.effects.exploreRefreshed");
      if (effect.type === "search") return this.$t("sources.interaction.effects.search", { query: effect.message || "" });
      return effect.message || effect.title || effect.url || "";
    },
  },
});
</script>

<template>
  <div class="sheet-overlay" @click.self="$emit('close')">
    <aside class="sheet" role="dialog" aria-modal="true" :aria-labelledby="`interaction-${source.sourceId}`">
      <header>
        <div><h2 :id="`interaction-${source.sourceId}`">{{ source.bookSourceName }}</h2><small>{{ source.bookSourceUrl }}</small></div>
        <button ref="closeButton" class="close" type="button" @click="$emit('close')">{{ $t("sources.close") }}</button>
      </header>
      <div class="sheet-body">
        <p class="intro">{{ $t("sources.interaction.description") }}</p>
        <p v-if="loading" aria-busy="true">{{ $t("sources.interaction.loading") }}</p>
        <p v-if="error" class="message error" role="alert">{{ error }}</p>
        <div v-if="effects.length" class="effects" role="status" aria-live="polite">
          <p v-for="(effect, index) in effects" :key="`${effect.type}-${index}`" :class="{ warning: effect.type === 'browser_required' }">{{ effectText(effect) }}</p>
        </div>
        <template v-if="view">
          <form class="controls" @submit.prevent>
            <template v-for="control in view.controls" :key="control.id">
              <p v-if="control.type === 'info'" class="info">{{ control.label }}</p>
              <label v-else-if="['text','password','input'].includes(control.type)" class="field">
                <span>{{ control.label }}</span><input v-model="values[control.label]" :type="control.type === 'password' ? 'password' : 'text'" :autocomplete="control.type === 'password' ? 'current-password' : 'off'">
              </label>
              <label v-else-if="control.type === 'select'" class="field">
                <span>{{ control.label }}</span><select v-model="values[control.label]" @change="changeControl(control.actionId)"><option v-for="option in control.options" :key="option" :value="option">{{ option }}</option></select>
              </label>
              <label v-else-if="control.type === 'toggle'" class="toggle">
                <input type="checkbox" :checked="toggleChecked(control.label, control.options)" @change="toggleValue(control.label, control.options, control.actionId)"><span>{{ control.label }}</span>
              </label>
              <AppButton v-else-if="control.type === 'button' && control.actionId" class="source-action" :busy="busyAction === control.actionId" :disabled="Boolean(busyAction)" @click="act(control.actionId)">{{ control.label }}</AppButton>
              <button v-else-if="control.type === 'unsupported'" class="unsupported" type="button" disabled>{{ control.label }} — {{ $t("sources.interaction.unsupported", { type: control.unsupported }) }}</button>
            </template>
          </form>
          <RuntimeCookieEditor :key="view.revision" :source-id="source.sourceId!" />
          <section class="maintenance">
            <h3>{{ $t("sources.interaction.maintenance.title") }}</h3>
            <p>{{ $t("sources.interaction.maintenance.description") }}</p>
            <button type="button" @click="pendingReset = 'login'"><strong>{{ $t("sources.interaction.reset.login.action") }}</strong><span>{{ $t("sources.interaction.reset.login.description") }}</span></button>
            <button type="button" @click="pendingReset = 'settings'"><strong>{{ $t("sources.interaction.reset.settings.action") }}</strong><span>{{ $t("sources.interaction.reset.settings.description") }}</span></button>
            <button class="full-reset" type="button" @click="pendingReset = 'all'"><strong>{{ $t("sources.interaction.reset.all.action") }}</strong><span>{{ $t("sources.interaction.reset.all.description") }}</span></button>
          </section>
        </template>
      </div>
    </aside>
    <SourceBrowserSession v-if="browserEffect?.browserRequestId" :source-id="source.sourceId!" :browser-request-id="browserEffect.browserRequestId" :title="browserEffect.title" @close="browserClosed" />
    <section v-if="pendingReset" class="reset-confirmation" role="alertdialog" :aria-label="$t(`sources.interaction.reset.${pendingReset}.title`)" @click.stop>
      <h2>{{ $t(`sources.interaction.reset.${pendingReset}.title`) }}</h2>
      <p>{{ $t(`sources.interaction.reset.${pendingReset}.confirm`) }}</p>
      <div><AppButton variant="secondary" :disabled="Boolean(busyAction)" @click="pendingReset = null">{{ $t("sources.cancel") }}</AppButton><AppButton variant="danger" :busy="busyAction === `reset-${pendingReset}`" @click="confirmReset">{{ $t(`sources.interaction.reset.${pendingReset}.action`) }}</AppButton></div>
    </section>
  </div>
</template>

<style scoped>
.sheet-overlay { position: fixed; z-index: 100; inset: 0; display: flex; justify-content: flex-end; background: rgb(38 34 29 / .38); }
.sheet { width: min(34rem, 100%); height: 100%; display: grid; grid-template-rows: auto minmax(0, 1fr); border-left: 1px solid var(--color-border); background: var(--color-paper-raised); box-shadow: -10px 0 28px rgb(38 34 29 / .12); }
.sheet > header { display: flex; align-items: flex-start; justify-content: space-between; gap: 1rem; padding: 1rem 1.15rem; border-bottom: 1px solid var(--color-border); }
h2, h3, p { margin: 0; }.sheet h2, .maintenance h3 { font-family: var(--font-literary); }.sheet h2 { overflow-wrap: anywhere; }.sheet header small { display: block; margin-top: .25rem; color: var(--color-ink-muted); overflow-wrap: anywhere; }.close { min-width: 2.75rem; min-height: 2.75rem; flex: 0 0 auto; border: 1px solid var(--color-border); border-radius: var(--radius-md); padding: .5rem .75rem; background: transparent; color: var(--color-ink); font-weight: 700; cursor: pointer; }
.sheet-body { overflow: auto; display: grid; align-content: start; gap: 1rem; padding: 1rem 1.15rem 2rem; }.intro { color: var(--color-ink-muted); line-height: 1.55; }.controls { display: grid; gap: .8rem; }.field { display: grid; gap: .35rem; }.field span, .toggle span { font-weight: 700; }.field input, .field select { width: 100%; min-width: 0; min-height: 2.75rem; border: 1px solid var(--color-border); border-radius: var(--radius-md); padding: .6rem .7rem; background: white; color: var(--color-ink); }.toggle { min-height: 2.75rem; display: flex; align-items: center; gap: .65rem; }.toggle input { width: 1.2rem; height: 1.2rem; }.info, .message, .effects p { padding: .75rem; border-radius: var(--radius-md); background: var(--color-paper-muted); line-height: 1.5; }.error { background: #f8e4df; color: var(--color-danger); }.effects { display: grid; gap: .5rem; }.effects .warning { background: #f5ead7; color: var(--color-ink); }.source-action { width: 100%; }.unsupported { min-height: 2.75rem; border: 1px dashed var(--color-border); border-radius: var(--radius-md); padding: .65rem; background: transparent; color: var(--color-ink-muted); text-align: left; }
.maintenance { display: grid; gap: .65rem; margin-top: .5rem; padding-top: 1.25rem; border-top: 1px solid var(--color-border); }.maintenance > p { color: var(--color-ink-muted); }.maintenance > button { min-height: 3.25rem; display: grid; gap: .2rem; border: 1px solid var(--color-border); border-radius: var(--radius-md); padding: .7rem .8rem; background: transparent; color: var(--color-ink); text-align: left; cursor: pointer; }.maintenance > button span { color: var(--color-ink-muted); font-size: .78rem; line-height: 1.35; }.maintenance .full-reset { border-color: color-mix(in srgb, var(--color-danger) 55%, var(--color-border)); color: var(--color-danger); }
.reset-confirmation { position: fixed; z-index: 110; inset: 50% auto auto 50%; width: min(30rem, calc(100% - 2rem)); transform: translate(-50%, -50%); display: grid; gap: .8rem; padding: 1rem; border: 1px solid var(--color-border); border-radius: var(--radius-lg); background: var(--color-paper-raised); box-shadow: 0 14px 36px rgb(38 34 29 / .2); }.reset-confirmation div { display: flex; justify-content: flex-end; gap: .5rem; }
@media (max-width: 38rem) { .sheet-overlay { align-items: flex-end; }.sheet { width: 100%; height: min(88dvh, 48rem); border: 1px solid var(--color-border); border-bottom: 0; border-radius: var(--radius-lg) var(--radius-lg) 0 0; box-shadow: 0 -10px 28px rgb(38 34 29 / .14); }.sheet > header { position: sticky; top: 0; background: var(--color-paper-raised); }.sheet-body { padding-bottom: calc(1.5rem + env(safe-area-inset-bottom)); }.reset-confirmation div { display: grid; }.reset-confirmation :deep(.app-button) { width: 100%; } }
</style>
