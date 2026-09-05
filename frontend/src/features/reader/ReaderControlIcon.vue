<script lang="ts">
import { defineComponent, type PropType } from 'vue';

export type ReaderControlIconName = 'previous' | 'next' | 'toc' | 'source' | 'bookmark' | 'refresh' | 'settings';

export default defineComponent({
  name: 'ReaderControlIcon',
  props: { name: { type: String as PropType<ReaderControlIconName>, required: true } },
});
</script>

<template>
  <span class="icon" :class="`icon-${name}`" aria-hidden="true">
    <svg v-if="name === 'previous' || name === 'next'" viewBox="0 0 24 24"><path d="M15 18l-6-6 6-6" /><path d="M9 12h10" /></svg>
    <svg v-else-if="name === 'toc'" viewBox="0 0 24 24"><path d="M9 6h11M9 12h11M9 18h11" /><circle cx="5" cy="6" r="1" /><circle cx="5" cy="12" r="1" /><circle cx="5" cy="18" r="1" /></svg>
    <svg v-else-if="name === 'source'" viewBox="0 0 24 24"><path d="M7 7h11l-3-3M17 17H6l3 3" /><path d="m18 7-3 3M6 17l3-3" /></svg>
    <svg v-else-if="name === 'bookmark'" viewBox="0 0 24 24"><path d="M7 4h10v16l-5-3-5 3V4Z" /></svg>
    <svg v-else-if="name === 'refresh'" viewBox="0 0 24 24"><path d="M20 7v5h-5M4 17v-5h5" /><path d="M6.1 6.1A8 8 0 0 1 20 12M4 12a8 8 0 0 0 13.9 5.9" /></svg>
    <svg v-else-if="name === 'settings'" viewBox="0 0 24 24"><circle cx="12" cy="12" r="3" /><path d="M19.4 15a1.7 1.7 0 0 0 .3 1.9l.1.1-2.8 2.8-.1-.1a1.7 1.7 0 0 0-1.9-.3 1.7 1.7 0 0 0-1 1.6v.2h-4V21a1.7 1.7 0 0 0-1-1.6 1.7 1.7 0 0 0-1.9.3l-.1.1L4.2 17l.1-.1a1.7 1.7 0 0 0 .3-1.9A1.7 1.7 0 0 0 3 14H2.8v-4H3a1.7 1.7 0 0 0 1.6-1 1.7 1.7 0 0 0-.3-1.9L4.2 7 7 4.2l.1.1A1.7 1.7 0 0 0 9 4.6 1.7 1.7 0 0 0 10 3v-.2h4V3a1.7 1.7 0 0 0 1 1.6 1.7 1.7 0 0 0 1.9-.3l.1-.1L19.8 7l-.1.1a1.7 1.7 0 0 0-.3 1.9 1.7 1.7 0 0 0 1.6 1h.2v4H21a1.7 1.7 0 0 0-1.6 1Z" /></svg>
  </span>
</template>

<style scoped>
.icon{width:1.45rem;height:1.45rem;display:grid;place-items:center;flex:0 0 1.45rem}.icon svg{width:1.2rem;height:1.2rem;fill:none;stroke:currentColor;stroke-width:1.75;stroke-linecap:round;stroke-linejoin:round}.icon-bookmark svg{width:1.35rem;height:1.35rem;stroke-width:1.8}.icon circle[r="1"]{fill:currentColor;stroke:none}.icon-next{transform:rotate(180deg)}
</style>
