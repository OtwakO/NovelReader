<script lang="ts">
import { defineComponent, nextTick } from 'vue';
import ReaderControlIcon from './ReaderControlIcon.vue';

export default defineComponent({
  name: 'ReaderActionsMenu',
  components: { ReaderControlIcon },
  props: { refreshDisabled: Boolean, refreshing: Boolean },
  emits: ['bookmarks', 'refetch'],
  data: () => ({ open: false, menuId: '' }),
  mounted() { this.menuId = `reader-actions-${this.$.uid}`; document.addEventListener('pointerdown', this.onOutside); },
  beforeUnmount() { document.removeEventListener('pointerdown', this.onOutside); },
  methods: {
    close(focus = false) { this.open=false; if(focus)(this.$refs.trigger as HTMLButtonElement).focus(); },
    onOutside(event:PointerEvent) { if(!(this.$el as HTMLElement).contains(event.target as Node))this.close(); },
    onFocusOut(event:FocusEvent) { if(!(this.$el as HTMLElement).contains(event.relatedTarget as Node|null))this.close(); },
    async toggle() { this.open=!this.open; if(this.open){await nextTick();this.items()[0]?.focus();} },
    items(): HTMLButtonElement[] { return Array.from((this.$el as HTMLElement).querySelectorAll<HTMLButtonElement>('[role="menuitem"]:not(:disabled)')); },
    onKeydown(event:KeyboardEvent) {
      if(event.key==='Escape'&&this.open){event.preventDefault();event.stopPropagation();this.close(true);return;}
      if(!this.open||!['ArrowDown','ArrowUp','Home','End'].includes(event.key))return;
      event.preventDefault();event.stopPropagation();
      const items=this.items();const index=items.indexOf(document.activeElement as HTMLButtonElement);
      const next=event.key==='Home'?0:event.key==='End'?items.length-1:(index+(event.key==='ArrowDown'?1:-1)+items.length)%items.length;
      items[next]?.focus();
    },
    choose(action:'bookmarks'|'refetch') { this.close(true);this.$emit(action); },
  },
});
</script>

<template>
  <div class="reader-actions" @keydown="onKeydown" @focusout="onFocusOut">
    <button ref="trigger" class="trigger" type="button" :aria-label="$t('reader.actions.title')" aria-haspopup="menu" :aria-expanded="open" :aria-controls="menuId" @click="toggle" @keydown.down.prevent="!open && toggle()">
      <svg width="22" height="22" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true"><circle cx="5" cy="12" r="1.8" /><circle cx="12" cy="12" r="1.8" /><circle cx="19" cy="12" r="1.8" /></svg>
    </button>
    <div v-if="open" :id="menuId" class="actions-panel" role="menu" :aria-label="$t('reader.actions.title')">
      <button type="button" role="menuitem" @click="choose('bookmarks')"><ReaderControlIcon name="bookmark" />{{ $t('reader.bookmarks.title') }}</button>
      <button type="button" role="menuitem" :disabled="refreshDisabled || refreshing" @click="choose('refetch')"><ReaderControlIcon name="refresh" />{{ $t(refreshing ? 'reader.actions.refetching' : 'reader.actions.refetch') }}</button>
    </div>
  </div>
</template>

<style scoped>
.reader-actions{position:relative}
.trigger{display:grid;place-items:center;width:2.75rem;height:2.75rem;border:0;border-radius:var(--radius-md);background:transparent;color:inherit;cursor:pointer}
.actions-panel{position:absolute;inset-block-start:calc(100% + .4rem);inset-inline-end:0;z-index:1;display:grid;width:max-content;max-width:calc(100vw - 2rem);padding:.35rem;border:1px solid color-mix(in srgb,var(--reader-text) 20%,transparent);border-radius:var(--radius-md);background:var(--reader-bg);box-shadow:0 .5rem 1.5rem color-mix(in srgb,var(--reader-text) 16%,transparent)}
.actions-panel button{display:flex;align-items:center;gap:.55rem;min-height:2.75rem;padding:.65rem .8rem;border:0;border-radius:var(--radius-md);background:transparent;color:inherit;font:inherit;text-align:start;cursor:pointer}
button:hover:not(:disabled),button:focus-visible{background:color-mix(in srgb,var(--reader-text) 10%,transparent)}
button:focus-visible{outline:2px solid currentColor;outline-offset:2px}
button:disabled{opacity:.5;cursor:default}
</style>
