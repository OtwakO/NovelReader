<script setup lang="ts">
import { onBeforeUnmount, ref } from 'vue';
import { closeSourceBrowser, getSourceBrowserFrame, sendSourceBrowserInput, startSourceBrowser, type SourceBrowserFrame } from '../../api/sources';
import AppButton from '../../ui/components/AppButton.vue';
import { isInternalSourceBrowserLocation, sourceBrowserViewport } from './source-browser-display';
import { i18n } from '../../i18n';

const props = defineProps<{ sourceId: string; browserRequestId: string; title?: string }>();
const t = i18n.global.t;
const emit = defineEmits<{ close: [saved: boolean] }>();
const frame = ref<SourceBrowserFrame>();
const error = ref('');
const busy = ref(true);
const typedText = ref('');
const viewport = ref<HTMLElement>();
let timer: number | undefined;
let closed = false;
let touchY: number | undefined;

void start();

async function start() {
  try {
    const bounds = viewport.value?.getBoundingClientRect();
    const browserViewport = sourceBrowserViewport(bounds?.width || window.innerWidth - 32, window.devicePixelRatio);
    frame.value = await startSourceBrowser(props.sourceId, props.browserRequestId, browserViewport.width, browserViewport.height, browserViewport.deviceScaleFactor);
    schedule();
  } catch (cause) {
    error.value = cause instanceof Error ? cause.message : t('sources.interaction.browser.startFailed');
  } finally {
    busy.value = false;
  }
}

function schedule() {
  window.clearTimeout(timer);
  if (!closed) timer = window.setTimeout(refresh, 1200);
}

async function refresh() {
  if (!frame.value || closed) return;
  try { frame.value = await getSourceBrowserFrame(props.sourceId, frame.value.sessionId); schedule(); }
  catch (cause) { error.value = cause instanceof Error ? cause.message : t('sources.interaction.browser.ended'); }
}

async function click(event: MouseEvent) {
  if (!frame.value || busy.value) return;
  const image = event.currentTarget as HTMLImageElement;
  const bounds = image.getBoundingClientRect();
  busy.value = true;
  try {
    frame.value = await sendSourceBrowserInput(props.sourceId, frame.value.sessionId, {
      type: 'click',
      x: (event.clientX - bounds.left) * frame.value.width / bounds.width,
      y: (event.clientY - bounds.top) * frame.value.height / bounds.height,
    });
    schedule();
  } catch (cause) { error.value = cause instanceof Error ? cause.message : t('sources.interaction.browser.inputFailed'); }
  finally { busy.value = false; }
}

async function scrollRemote(deltaY: number) {
  if (!frame.value || busy.value || !deltaY) return;
  busy.value = true;
  try {
    frame.value = await sendSourceBrowserInput(props.sourceId, frame.value.sessionId, { type: 'scroll', y: deltaY });
    schedule();
  } catch (cause) { error.value = cause instanceof Error ? cause.message : t('sources.interaction.browser.scrollFailed'); }
  finally { busy.value = false; }
}

function wheel(event: WheelEvent) {
  event.preventDefault();
  void scrollRemote(Math.max(-900, Math.min(900, event.deltaY)));
}

function touchStart(event: TouchEvent) {
  touchY = event.touches[0]?.clientY;
}

function touchEnd(event: TouchEvent) {
  const endY = event.changedTouches[0]?.clientY;
  if (touchY !== undefined && endY !== undefined) void scrollRemote(Math.max(-900, Math.min(900, touchY - endY)));
  touchY = undefined;
}

async function typeText() {
  if (!frame.value || !typedText.value || busy.value) return;
  busy.value = true;
  try {
    frame.value = await sendSourceBrowserInput(props.sourceId, frame.value.sessionId, { type: 'type', text: typedText.value });
    typedText.value = '';
    schedule();
  } catch (cause) { error.value = cause instanceof Error ? cause.message : t('sources.interaction.browser.inputFailed'); }
  finally { busy.value = false; }
}

async function finish(save: boolean) {
  if (closed) return;
  closed = true;
  window.clearTimeout(timer);
  if (frame.value) {
    busy.value = true;
    try { await closeSourceBrowser(props.sourceId, frame.value.sessionId, save); }
    catch (cause) { error.value = cause instanceof Error ? cause.message : t('sources.interaction.browser.closeFailed'); closed = false; busy.value = false; return; }
  }
  emit('close', save);
}

onBeforeUnmount(() => {
  window.clearTimeout(timer);
  if (!closed && frame.value) void closeSourceBrowser(props.sourceId, frame.value.sessionId, false).catch(() => undefined);
});
</script>

<template>
  <section class="browser" role="dialog" aria-modal="true" :aria-label="title || t('sources.interaction.browser.title')">
    <header><div><h2>{{ frame?.title || title || t('sources.interaction.browser.title') }}</h2><small>{{ isInternalSourceBrowserLocation(frame?.url) ? t('sources.interaction.browser.sourceDocument') : frame?.url }}</small></div><button type="button" @click="finish(false)">{{ t('sources.close') }}</button></header>
    <div class="messages">
      <p class="privacy">{{ t('sources.interaction.browser.privacy') }}</p>
      <p v-if="error" class="error" role="alert">{{ error }}</p>
    </div>
    <div ref="viewport" class="viewport" :aria-busy="busy" @wheel.prevent="wheel">
      <img v-if="frame" :src="`data:${frame.mediaType};base64,${frame.image}`" :alt="t('sources.interaction.browser.imageAlt')" draggable="false" @click="click" @touchstart.passive="touchStart" @touchend="touchEnd">
      <p v-else-if="busy">{{ t('sources.interaction.browser.opening') }}</p>
    </div>
    <form class="typing" @submit.prevent="typeText"><input v-model="typedText" autocomplete="off" :placeholder="t('sources.interaction.browser.typePlaceholder')"><AppButton :disabled="!typedText" :busy="busy" type="submit">{{ t('sources.interaction.browser.type') }}</AppButton></form>
    <footer><AppButton variant="secondary" :disabled="busy" @click="finish(false)">{{ t('sources.cancel') }}</AppButton><AppButton :busy="busy" @click="finish(true)">{{ t('sources.interaction.browser.finish') }}</AppButton></footer>
  </section>
</template>

<style scoped>
.browser { position: fixed; z-index: 120; inset: 1rem; margin: auto; width: min(34rem, calc(100% - 2rem)); height: min(60rem, calc(100dvh - 2rem)); display: grid; grid-template-rows: auto auto minmax(0, 1fr) auto auto; gap: .625rem; padding: .875rem; overflow: hidden; border: 1px solid var(--color-border); border-radius: var(--radius-lg); background: var(--color-paper-raised); box-shadow: 0 18px 48px rgb(38 34 29 / .28); }
header, footer, .typing { display: flex; align-items: center; gap: .625rem; } header, footer { justify-content: space-between; } h2, p { margin: 0; } small { color: var(--color-ink-muted); overflow-wrap: anywhere; } header button { min-height: 2.5rem; border: 1px solid var(--color-border); border-radius: var(--radius-md); background: transparent; color: var(--color-ink); padding: .4rem .7rem; } .messages { display: grid; gap: .4rem; } .privacy { color: var(--color-ink-muted); } .error { padding: .6rem; border-radius: var(--radius-md); background: #f8e4df; color: var(--color-danger); }
.viewport { min-width: 0; min-height: 0; display: grid; place-items: center; overflow: auto; border: 1px solid var(--color-border); border-radius: var(--radius-md); background: #1e1c1a; } .viewport img { display: block; width: auto; height: auto; max-width: 100%; max-height: 100%; object-fit: contain; cursor: crosshair; image-rendering: auto; } .typing { min-width: 0; flex-wrap: nowrap; } .typing input { min-width: 0; flex: 1; min-height: 2.75rem; border: 1px solid var(--color-border); border-radius: var(--radius-md); padding: .6rem .7rem; } footer { flex: none; justify-content: flex-end; }
@media (max-width: 38rem) { .browser { inset: 0; width: 100%; height: 100dvh; grid-template-rows: auto auto minmax(0, 1fr) auto auto; gap: .5rem; padding: .625rem; border: 0; border-radius: 0; } .typing { align-items: stretch; } footer :deep(.app-button) { flex: 1; } }
</style>
