<script lang="ts">
import { defineComponent, type PropType } from 'vue';
import type { CatalogNavigationEntry } from '../../api/catalog-navigation';
import type { ReadingTarget } from '../../api/reading-target';
import { readerLocation } from './reader-session';

export default defineComponent({
  name: 'TocNavigationList',
  props: {
    entries: { type: Array as PropType<CatalogNavigationEntry[]>, required: true },
    currentTarget: { type: Object as PropType<ReadingTarget>, default: undefined },
    bookId: { type: String, default: '' },
    interactive: { type: Boolean, default: true },
    depth: { type: Number, default: 0 },
  },
  emits: ['open'],
  methods: { readerLocation },
});
</script>

<template>
  <ol class="navigation-list" role="list">
    <li v-for="(entry, index) in entries" :key="index">
      <div class="navigation-row" :class="{ current: !!entry.target && entry.target === currentTarget, grouping: !entry.target }" :data-current="!!entry.target && entry.target === currentTarget" :style="{ '--toc-depth': Math.min(depth, 6) }">
        <RouterLink v-if="entry.target && !entry.unavailable && interactive && bookId" :to="readerLocation(bookId, entry.target.chapterIndex, entry.target.contentRevision, undefined, { anchor: entry.target.anchor })" :aria-current="entry.target === currentTarget ? 'location' : undefined">{{ entry.label || $t('reader.toc.untitledEntry') }}</RouterLink>
        <button v-else-if="entry.target && !entry.unavailable && interactive" type="button" :aria-current="entry.target === currentTarget ? 'location' : undefined" @click="$emit('open', entry.target)">{{ entry.label || $t('reader.toc.untitledEntry') }}</button>
        <span v-else>{{ entry.label || $t('reader.toc.untitledEntry') }}<small v-if="entry.unavailable">{{ $t('reader.toc.unavailable') }}</small></span>
      </div>
      <TocNavigationList v-if="entry.children.length" :entries="entry.children" :current-target="currentTarget" :book-id="bookId" :interactive="interactive" :depth="depth + 1" @open="$emit('open', $event)" />
    </li>
  </ol>
</template>

<style scoped>
.navigation-list{list-style:none;margin:0;padding:0;min-width:0}
.navigation-row{border-bottom:1px solid var(--color-border);min-width:0}
.navigation-row>a,.navigation-row>button,.navigation-row>span{display:block;box-sizing:border-box;width:100%;min-height:3.5rem;padding:.75rem 1rem;padding-inline-start:calc(1rem + var(--toc-depth)*.75rem);border:0;background:transparent;color:var(--color-ink);text-align:start;text-decoration:none;line-height:1.5;overflow-wrap:anywhere}
.navigation-row>a,.navigation-row>button{cursor:pointer}
.navigation-row>a:hover,.navigation-row>button:hover{background:var(--color-accent-soft)}
.navigation-row>a:focus-visible,.navigation-row>button:focus-visible{outline:2px solid var(--color-accent);outline-offset:-2px}
.navigation-row.current{background:var(--color-accent-soft);font-weight:var(--weight-strong)}
.navigation-row.grouping{background:var(--color-paper-muted);font-weight:var(--weight-strong)}
.navigation-row small{display:block;color:var(--color-ink-muted);font-size:var(--text-caption);font-weight:var(--weight-regular)}
</style>
