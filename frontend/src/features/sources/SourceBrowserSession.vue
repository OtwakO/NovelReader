<script lang="ts">
import { defineComponent, nextTick, type PropType } from 'vue';
import {
  closeSourceBrowser,
  getSourceBrowserFrame,
  sendSourceBrowserInput,
  startSourceBrowser,
  type SourceBrowserFrame,
  type SourceInteractionActionResult,
} from '../../api/sources';
import AppButton from '../../ui/components/AppButton.vue';
import { isInternalSourceBrowserLocation, sourceBrowserViewport } from './source-browser-display';

export default defineComponent({
  name: 'SourceBrowserSession',
  components: { AppButton },
  props: {
    sourceId: { type: String, required: true },
    browserRequestId: { type: String, required: true },
    title: { type: String as PropType<string | undefined>, default: undefined },
  },
  emits: ['close'],
  data: () => ({
    frame: undefined as SourceBrowserFrame | undefined,
    error: '',
    busy: true,
    typedText: '',
    timer: undefined as number | undefined,
    closed: false,
    touchY: undefined as number | undefined,
    frameRequest: 0,
    pendingScrollY: 0,
    scrolling: false,
  }),
  mounted() {
    void this.start();
  },
  beforeUnmount() {
    window.clearTimeout(this.timer);
    if (!this.closed && this.frame) {
      void closeSourceBrowser(this.sourceId, this.frame.sessionId, false).catch(() => undefined);
    }
  },
  methods: {
    isInternalSourceBrowserLocation,
    async start() {
      try {
        await nextTick();
        const bounds = (this.$refs.viewport as HTMLElement | undefined)?.getBoundingClientRect();
        const viewport = sourceBrowserViewport(
          bounds?.width || window.innerWidth - 32,
          bounds?.height || window.innerHeight - 260,
          window.devicePixelRatio,
        );
        this.frame = await startSourceBrowser(
          this.sourceId,
          this.browserRequestId,
          viewport.width,
          viewport.height,
          viewport.deviceScaleFactor,
        );
        this.schedule();
      } catch (cause) {
        this.error = cause instanceof Error ? cause.message : this.$t('sources.interaction.browser.startFailed');
      } finally {
        this.busy = false;
      }
    },
    schedule() {
      window.clearTimeout(this.timer);
      if (!this.closed) this.timer = window.setTimeout(() => void this.refresh(), 1200);
    },
    async refresh() {
      if (!this.frame || this.closed || this.busy || this.scrolling) {
        this.schedule();
        return;
      }
      const request = ++this.frameRequest;
      try {
        const refreshed = await getSourceBrowserFrame(this.sourceId, this.frame.sessionId);
        if (request === this.frameRequest && !this.closed) this.frame = refreshed;
        this.schedule();
      } catch (cause) {
        if (request === this.frameRequest) {
          this.error = cause instanceof Error ? cause.message : this.$t('sources.interaction.browser.ended');
        }
      }
    },
    async clickFrame(event: MouseEvent) {
      if (!this.frame || this.busy) return;
      const image = event.currentTarget as HTMLImageElement;
      const bounds = image.getBoundingClientRect();
      const request = ++this.frameRequest;
      this.busy = true;
      window.clearTimeout(this.timer);
      try {
        const updated = await sendSourceBrowserInput(this.sourceId, this.frame.sessionId, {
          type: 'click',
          x: (event.clientX - bounds.left) * this.frame.width / bounds.width,
          y: (event.clientY - bounds.top) * this.frame.height / bounds.height,
        });
        if (request === this.frameRequest && !this.closed) this.frame = updated;
        this.schedule();
      } catch (cause) {
        this.error = cause instanceof Error ? cause.message : this.$t('sources.interaction.browser.inputFailed');
      } finally {
        this.busy = false;
      }
    },
    async scrollRemote(deltaY: number) {
      if (!this.frame || !deltaY || this.closed) return;
      this.pendingScrollY = Math.max(-1800, Math.min(1800, this.pendingScrollY + deltaY));
      if (this.scrolling) return;
      this.scrolling = true;
      window.clearTimeout(this.timer);
      try {
        while (this.pendingScrollY && this.frame && !this.closed) {
          const currentDelta = this.pendingScrollY;
          this.pendingScrollY = 0;
          const request = ++this.frameRequest;
          const updated = await sendSourceBrowserInput(this.sourceId, this.frame.sessionId, {
            type: 'scroll',
            y: currentDelta,
          });
          if (request === this.frameRequest && !this.closed) this.frame = updated;
        }
        this.schedule();
      } catch (cause) {
        this.error = cause instanceof Error ? cause.message : this.$t('sources.interaction.browser.scrollFailed');
      } finally {
        this.scrolling = false;
      }
    },
    wheel(event: WheelEvent) {
      event.preventDefault();
      void this.scrollRemote(Math.max(-900, Math.min(900, event.deltaY)));
    },
    touchStart(event: TouchEvent) {
      this.touchY = event.touches[0]?.clientY;
    },
    touchEnd(event: TouchEvent) {
      const endY = event.changedTouches[0]?.clientY;
      if (this.touchY !== undefined && endY !== undefined) {
        void this.scrollRemote(Math.max(-900, Math.min(900, this.touchY - endY)));
      }
      this.touchY = undefined;
    },
    async typeText() {
      if (!this.frame || !this.typedText || this.busy) return;
      const request = ++this.frameRequest;
      this.busy = true;
      window.clearTimeout(this.timer);
      try {
        const updated = await sendSourceBrowserInput(this.sourceId, this.frame.sessionId, {
          type: 'type',
          text: this.typedText,
        });
        if (request === this.frameRequest && !this.closed) this.frame = updated;
        this.typedText = '';
        this.schedule();
      } catch (cause) {
        this.error = cause instanceof Error ? cause.message : this.$t('sources.interaction.browser.inputFailed');
      } finally {
        this.busy = false;
      }
    },
    async finish(save: boolean) {
      if (this.closed) return;
      this.closed = true;
      this.frameRequest++;
      window.clearTimeout(this.timer);
      let resumed: SourceInteractionActionResult | undefined;
      if (this.frame) {
        this.busy = true;
        try {
          ({ resumed } = await closeSourceBrowser(this.sourceId, this.frame.sessionId, save));
        } catch (cause) {
          this.error = cause instanceof Error ? cause.message : this.$t('sources.interaction.browser.closeFailed');
          this.closed = false;
          this.busy = false;
          return;
        }
      }
      this.$emit('close', save, resumed);
    },
  },
});
</script>

<template>
  <section class="browser" role="dialog" aria-modal="true" :aria-label="title || $t('sources.interaction.browser.title')">
    <header>
      <div>
        <h2>{{ frame?.title || title || $t('sources.interaction.browser.title') }}</h2>
        <small>{{ isInternalSourceBrowserLocation(frame?.url) ? $t('sources.interaction.browser.sourceDocument') : frame?.url }}</small>
      </div>
      <button type="button" @click="finish(false)">{{ $t('sources.close') }}</button>
    </header>
    <div class="messages">
      <p class="privacy">{{ $t('sources.interaction.browser.privacy') }}</p>
      <p v-if="error" class="error" role="alert">{{ error }}</p>
    </div>
    <div ref="viewport" class="viewport" :aria-busy="busy" @wheel.prevent="wheel">
      <img
        v-if="frame"
        :src="`data:${frame.mediaType};base64,${frame.image}`"
        :alt="$t('sources.interaction.browser.imageAlt')"
        :style="{ width: `${frame.width}px` }"
        draggable="false"
        @click="clickFrame"
        @touchstart.passive="touchStart"
        @touchend="touchEnd"
      >
      <p v-else-if="busy">{{ $t('sources.interaction.browser.opening') }}</p>
    </div>
    <form class="typing" @submit.prevent="typeText">
      <input v-model="typedText" autocomplete="off" :placeholder="$t('sources.interaction.browser.typePlaceholder')">
      <AppButton :disabled="!typedText" :busy="busy" type="submit">{{ $t('sources.interaction.browser.type') }}</AppButton>
    </form>
    <footer>
      <AppButton variant="secondary" :disabled="busy" @click="finish(false)">{{ $t('sources.cancel') }}</AppButton>
      <AppButton :busy="busy" @click="finish(true)">{{ $t('sources.interaction.browser.finish') }}</AppButton>
    </footer>
  </section>
</template>

<style scoped>
.browser { position: fixed; z-index: 120; inset: 1rem; margin: auto; width: min(34rem, calc(100% - 2rem)); height: min(60rem, calc(100dvh - 2rem)); display: grid; grid-template-rows: auto auto minmax(0, 1fr) auto auto; gap: .625rem; padding: .875rem; overflow: hidden; border: 1px solid var(--color-border); border-radius: var(--radius-lg); background: var(--color-paper-raised); box-shadow: 0 18px 48px rgb(38 34 29 / .28); }
header, footer, .typing { display: flex; align-items: center; gap: .625rem; } header, footer { justify-content: space-between; } h2, p { margin: 0; } small { color: var(--color-ink-muted); overflow-wrap: anywhere; } header button { min-height: 2.5rem; border: 1px solid var(--color-border); border-radius: var(--radius-md); background: transparent; color: var(--color-ink); padding: .4rem .7rem; } .messages { display: grid; gap: .4rem; } .privacy { color: var(--color-ink-muted); } .error { padding: .6rem; border-radius: var(--radius-md); background: #f8e4df; color: var(--color-danger); }
.viewport { min-width: 0; min-height: 0; display: grid; place-items: center; overflow: hidden; border: 1px solid var(--color-border); border-radius: var(--radius-md); background: #1e1c1a; } .viewport img { display: block; height: auto; max-width: 100%; max-height: 100%; object-fit: contain; cursor: crosshair; image-rendering: auto; touch-action: none; } .typing { min-width: 0; flex-wrap: nowrap; } .typing input { min-width: 0; flex: 1; min-height: 2.75rem; border: 1px solid var(--color-border); border-radius: var(--radius-md); padding: .6rem .7rem; } footer { flex: none; justify-content: flex-end; }
@media (max-width: 38rem) { .browser { inset: 0; width: 100%; height: 100dvh; grid-template-rows: auto auto minmax(0, 1fr) auto auto; gap: .5rem; padding: .625rem; border: 0; border-radius: 0; } .typing { align-items: stretch; } footer :deep(.app-button) { flex: 1; } }
</style>
