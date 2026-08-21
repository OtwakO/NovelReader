<script lang="ts">
import { defineComponent } from 'vue';
import { getWebViewStatus, type WebViewStatusKind } from '../../api/system';
import AppButton from '../../ui/components/AppButton.vue';

export default defineComponent({
  name: 'WebViewStatusCard',
  components: { AppButton },
  data() {
    return { status: '' as WebViewStatusKind | '', checking: false, checkedAt: 0, requestFailed: false };
  },
  computed: {
    tone(): string { return this.requestFailed || this.status === 'unavailable' ? 'danger' : this.status === 'ready' ? 'success' : 'neutral'; },
    statusKey(): string { return this.requestFailed ? 'requestFailed' : this.status || 'checking'; },
    checkedLabel(): string { return this.checkedAt ? new Intl.DateTimeFormat(this.$i18n.locale, { hour: 'numeric', minute: '2-digit', second: '2-digit' }).format(new Date(this.checkedAt)) : ''; },
  },
  async mounted() { await this.check(); },
  methods: {
    async check() {
      if (this.checking) return;
      this.checking = true; this.requestFailed = false;
      try { const result = await getWebViewStatus(); this.status = result.status; this.checkedAt = result.checkedAt; }
      catch { this.requestFailed = true; this.checkedAt = Date.now(); }
      finally { this.checking = false; }
    },
  },
});
</script>

<template>
  <section class="panel webview-card" :class="`tone-${tone}`" aria-live="polite">
    <div class="status-copy">
      <p class="eyebrow">{{ $t('settings.webview.eyebrow') }}</p>
      <div class="title-row"><span class="indicator" aria-hidden="true" /><h2>{{ $t('settings.webview.title') }}</h2></div>
      <strong class="summary">{{ $t(`settings.webview.states.${statusKey}.title`) }}</strong>
      <p>{{ $t(`settings.webview.states.${statusKey}.description`) }}</p>
      <small v-if="checkedLabel">{{ $t('settings.webview.checkedAt', { time: checkedLabel }) }}</small>
    </div>
    <AppButton variant="quiet" :busy="checking" @click="check">{{ checking ? $t('settings.webview.checking') : $t('settings.webview.retry') }}</AppButton>
  </section>
</template>

<style scoped>
.panel{padding:1rem;border:1px solid var(--color-border);border-radius:var(--radius-lg);background:var(--color-paper-raised)}.webview-card{display:flex;justify-content:space-between;align-items:center;gap:1.5rem}.status-copy{min-width:0}.eyebrow{margin:0;color:var(--color-warm);font-size:.7rem;font-weight:800;letter-spacing:.1em;text-transform:uppercase}.title-row{display:flex;align-items:center;gap:.65rem}.title-row h2{margin:.15rem 0;font:700 1.25rem var(--font-literary)}.indicator{width:.72rem;height:.72rem;flex:none;border-radius:999px;background:var(--color-ink-muted);box-shadow:0 0 0 .22rem color-mix(in srgb,var(--color-ink-muted) 14%,transparent)}.summary{display:block;margin-top:.45rem}.webview-card p:not(.eyebrow){margin:.25rem 0 0;color:var(--color-ink-muted);line-height:1.55}.webview-card small{display:block;margin-top:.5rem;color:var(--color-ink-muted)}.tone-success{border-color:color-mix(in srgb,var(--color-success) 45%,var(--color-border))}.tone-success .indicator{background:var(--color-success);box-shadow:0 0 0 .22rem color-mix(in srgb,var(--color-success) 15%,transparent)}.tone-danger{border-color:color-mix(in srgb,var(--color-danger) 45%,var(--color-border))}.tone-danger .indicator{background:var(--color-danger);box-shadow:0 0 0 .22rem color-mix(in srgb,var(--color-danger) 15%,transparent)}@media(max-width:38rem){.webview-card{align-items:stretch;flex-direction:column}.webview-card :deep(button){width:100%}}
</style>
