<script lang="ts">
import { defineComponent, type PropType } from 'vue';
import { addBookmark, deleteBookmark, listBookmarks, type Bookmark } from '../../api/reader';
import AppButton from '../../ui/components/AppButton.vue';
import { isReaderRevisionConflict } from './reader-session';
import { queueReadingStateWrite } from './progress-writer';

export default defineComponent({
  name: 'ReaderBookmarksSheet',
  components: { AppButton },
  props: {
    bookId: { type: String, required: true },
    currentRevision: { type: Number, required: true },
    capture: {
      type: Function as PropType<() => Promise<{ contentRevision: number; chapterIndex: number; position: number }>>,
      required: true,
    },
  },
  emits: ['open', 'close', 'stale'],
  data() { return { bookmarks: [] as Bookmark[], note: '', loading: true, busy: false, error: '' }; },
  async mounted() {
    try { this.bookmarks = await listBookmarks(this.bookId); }
    catch (cause) { this.error = cause instanceof Error ? cause.message : this.$t('reader.bookmarks.loadFailed'); }
    finally { this.loading = false; }
  },
  methods: {
    async submit() {
      if (this.busy) return;
      this.busy = true;
      this.error = '';
      try {
        const bookId = this.bookId;
        const location = await this.capture();
        const bookmark = { id: crypto.randomUUID(), ...location, note: this.note };
        const mark = await queueReadingStateWrite(bookId, stateVersion => addBookmark(bookId, { ...bookmark, stateVersion }));
        if (mark) {
          this.bookmarks = [mark, ...this.bookmarks];
          this.note = '';
        }
      } catch (cause) {
        if (isReaderRevisionConflict(cause)) this.$emit('stale');
        this.error = cause instanceof Error ? cause.message : this.$t('reader.bookmarks.saveFailed');
      } finally { this.busy = false; }
    },
    async remove(mark: Bookmark) {
      try {
        const bookId = this.bookId;
        // An orphan retains its old location revision; deletion guards current library state.
        const revision = this.currentRevision;
        const saved = await queueReadingStateWrite(bookId, stateVersion => deleteBookmark(bookId, mark.id, revision, stateVersion));
        if (saved) this.bookmarks = this.bookmarks.filter(item => item.id !== mark.id);
      } catch (cause) {
        if (isReaderRevisionConflict(cause)) this.$emit('stale');
        this.error = cause instanceof Error ? cause.message : this.$t('reader.bookmarks.deleteFailed');
      }
    },
  },
});
</script>
<template><div class="overlay" @click.self="$emit('close')" @keydown.esc="$emit('close')"><section class="sheet" role="dialog" aria-modal="true" :aria-label="$t('reader.bookmarks.title')"><header><h2>{{ $t('reader.bookmarks.title') }}</h2><AppButton variant="quiet" @click="$emit('close')">{{ $t('reader.close') }}</AppButton></header><form @submit.prevent="submit"><textarea v-model="note" maxlength="1000" :placeholder="$t('reader.bookmarks.note')" /><AppButton type="submit" :busy="busy">{{ $t('reader.bookmarks.add') }}</AppButton></form><p v-if="error" class="error" role="status">{{ error }}</p><p v-if="loading">{{ $t('reader.bookmarks.loading') }}</p><p v-else-if="!bookmarks.length">{{ $t('reader.bookmarks.empty') }}</p><ul v-else><li v-for="mark in bookmarks" :key="mark.id" :class="{orphaned:mark.orphaned}"><div><strong>{{ mark.chapterTitle }}</strong><small>{{ $t('reader.bookmarks.position',{percent:Math.round(mark.position*100)}) }}<template v-if="mark.orphaned"> · {{ $t('reader.bookmarks.orphaned') }}</template></small><p v-if="mark.note">{{ mark.note }}</p></div><div class="actions"><AppButton variant="secondary" :disabled="mark.orphaned" @click="$emit('open',mark.chapterIndex,mark.position,mark.contentRevision)">{{ $t('reader.bookmarks.open') }}</AppButton><AppButton variant="danger" @click="remove(mark)">{{ $t('reader.bookmarks.delete') }}</AppButton></div></li></ul></section></div></template>
<style scoped>.overlay{position:fixed;z-index:90;inset:0;display:flex;align-items:flex-end;justify-content:center;background:rgb(0 0 0/.38)}.sheet{width:min(46rem,100%);max-height:82dvh;overflow:auto;padding:1rem;border-radius:var(--radius-lg) var(--radius-lg) 0 0;background:var(--color-paper-raised);color:var(--color-ink)}header,.actions{display:flex;justify-content:space-between;align-items:center;gap:.5rem}h2{margin:0;font:var(--weight-strong) var(--text-subheading) var(--font-literary)}form{display:grid;gap:.6rem;margin:1rem 0}textarea{min-height:5rem;resize:vertical;border:1px solid var(--color-border);border-radius:var(--radius-md);padding:.7rem}ul{list-style:none;display:grid;gap:.6rem;margin:0;padding:0}li{display:grid;grid-template-columns:minmax(0,1fr) auto;gap:.8rem;padding:.75rem;border:1px solid var(--color-border);border-radius:var(--radius-md)}li strong,li small{display:block;overflow-wrap:anywhere}li small{margin-top:.2rem;color:var(--color-ink-muted)}li p{white-space:pre-wrap}.orphaned{opacity:.7}.error{color:var(--color-danger)}@media(max-width:36rem){li{grid-template-columns:1fr}.actions{justify-content:flex-start;flex-wrap:wrap}}</style>