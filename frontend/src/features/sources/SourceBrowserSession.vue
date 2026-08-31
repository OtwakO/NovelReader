<script setup lang="ts">
import { onBeforeUnmount, ref } from 'vue';
import { closeSourceBrowser, getSourceBrowserFrame, sendSourceBrowserInput, startSourceBrowser, type SourceBrowserFrame } from '../../api/sources';
import AppButton from '../../ui/components/AppButton.vue';
import { sourceBrowserLocation } from './source-browser-display';

const props = defineProps<{ sourceId: string; browserRequestId: string; title?: string }>();
const emit = defineEmits<{ close: [saved: boolean] }>();
const frame = ref<SourceBrowserFrame>();
const error = ref('');
const busy = ref(true);
const typedText = ref('');
const viewport = ref<HTMLElement>();
let timer: number | undefined;
let closed = false;

void start();

async function start() {
  try {
    const bounds = viewport.value?.getBoundingClientRect();
    const width = Math.round(bounds?.width || Math.min(window.innerWidth - 64, 1200));
    const height = Math.round(bounds?.height || Math.min(window.innerHeight - 280, 900));
    const deviceScaleFactor = Math.min(2, Math.max(1, window.devicePixelRatio || 1));
    frame.value = await startSourceBrowser(props.sourceId, props.browserRequestId, width, height, deviceScaleFactor);
    schedule();
  } catch (cause) {
    error.value = cause instanceof Error ? cause.message : 'Unable to start browser session';
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
  catch (cause) { error.value = cause instanceof Error ? cause.message : 'Browser session ended'; }
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
  } catch (cause) { error.value = cause instanceof Error ? cause.message : 'Browser input failed'; }
  finally { busy.value = false; }
}

async function typeText() {
  if (!frame.value || !typedText.value || busy.value) return;
  busy.value = true;
  try {
    frame.value = await sendSourceBrowserInput(props.sourceId, frame.value.sessionId, { type: 'type', text: typedText.value });
    typedText.value = '';
    schedule();
  } catch (cause) { error.value = cause instanceof Error ? cause.message : 'Browser input failed'; }
  finally { busy.value = false; }
}

async function finish(save: boolean) {
  if (closed) return;
  closed = true;
  window.clearTimeout(timer);
  if (frame.value) {
    busy.value = true;
    try { await closeSourceBrowser(props.sourceId, frame.value.sessionId, save); }
    catch (cause) { error.value = cause instanceof Error ? cause.message : 'Unable to close browser session'; closed = false; busy.value = false; return; }
  }
  emit('close', save);
}

onBeforeUnmount(() => {
  window.clearTimeout(timer);
  if (!closed && frame.value) void closeSourceBrowser(props.sourceId, frame.value.sessionId, false).catch(() => undefined);
});
</script>

<template>
  <section class="browser" role="dialog" aria-modal="true" :aria-label="title || 'Source login'">
    <header><div><h2>{{ frame?.title || title || 'Source login' }}</h2><small>{{ sourceBrowserLocation(frame?.url) }}</small></div><button type="button" @click="finish(false)">Close</button></header>
    <div class="messages">
      <p class="privacy">This page is provided by the source website. The session expires automatically when inactive.</p>
      <p v-if="error" class="error" role="alert">{{ error }}</p>
    </div>
    <div ref="viewport" class="viewport" :aria-busy="busy">
      <img v-if="frame" :src="`data:${frame.mediaType};base64,${frame.image}`" alt="Interactive source login page" @click="click">
      <p v-else-if="busy">Opening secure browser session…</p>
    </div>
    <form class="typing" @submit.prevent="typeText"><input v-model="typedText" autocomplete="off" placeholder="Type into the selected field"><AppButton :disabled="!typedText" :busy="busy" type="submit">Type</AppButton></form>
    <footer><AppButton variant="secondary" :disabled="busy" @click="finish(false)">Cancel</AppButton><AppButton :busy="busy" @click="finish(true)">Finish login</AppButton></footer>
  </section>
</template>

<style scoped>
.browser { position: fixed; z-index: 120; inset: 1rem; margin: auto; width: min(76rem, calc(100% - 2rem)); height: min(60rem, calc(100dvh - 2rem)); display: grid; grid-template-rows: auto auto minmax(0, 1fr) auto auto; gap: .625rem; padding: .875rem; overflow: hidden; border: 1px solid var(--color-border); border-radius: var(--radius-lg); background: var(--color-paper-raised); box-shadow: 0 18px 48px rgb(38 34 29 / .28); }
header, footer, .typing { display: flex; align-items: center; gap: .625rem; } header, footer { justify-content: space-between; } h2, p { margin: 0; } small { color: var(--color-ink-muted); overflow-wrap: anywhere; } header button { min-height: 2.5rem; border: 1px solid var(--color-border); border-radius: var(--radius-md); background: transparent; color: var(--color-ink); padding: .4rem .7rem; } .messages { display: grid; gap: .4rem; } .privacy { color: var(--color-ink-muted); } .error { padding: .6rem; border-radius: var(--radius-md); background: #f8e4df; color: var(--color-danger); }
.viewport { min-width: 0; min-height: 0; display: grid; place-items: center; overflow: auto; border: 1px solid var(--color-border); border-radius: var(--radius-md); background: #1e1c1a; } .viewport img { display: block; width: auto; height: auto; max-width: 100%; max-height: 100%; object-fit: contain; cursor: crosshair; image-rendering: auto; } .typing { min-width: 0; flex-wrap: nowrap; } .typing input { min-width: 0; flex: 1; min-height: 2.75rem; border: 1px solid var(--color-border); border-radius: var(--radius-md); padding: .6rem .7rem; } footer { flex: none; justify-content: flex-end; }
@media (max-width: 38rem) { .browser { inset: 0; width: 100%; height: 100dvh; grid-template-rows: auto auto minmax(0, 1fr) auto auto; gap: .5rem; padding: .625rem; border: 0; border-radius: 0; } .typing { align-items: stretch; } footer :deep(.app-button) { flex: 1; } }
</style>
