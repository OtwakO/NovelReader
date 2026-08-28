<script lang="ts">
import { defineComponent, type PropType } from 'vue';
import type { SourceCollection, SyncInterval } from '../../api/source-collections';
import AppButton from '../../ui/components/AppButton.vue';

export default defineComponent({
  name: 'SourceCollectionDialog',
  components: { AppButton },
  props: {
    collection: { type: Object as PropType<SourceCollection | null>, default: null },
    busy: Boolean,
    error: { type: String, default: '' },
  },
  emits: ['submit', 'close'],
  data() {
    return {
      mode: this.collection ? 'edit' : 'upload',
      name: this.collection?.name || '',
      url: this.collection?.originUrl || '',
      interval: (this.collection?.syncInterval || 'manual') as SyncInterval,
      file: null as File | null,
    };
  },
  computed: {
    canSubmit(): boolean {
      if (!this.name.trim()) return false;
      if (this.collection) return true;
      return this.mode === 'upload' ? Boolean(this.file) : Boolean(this.url.trim());
    },
  },
  methods: {
    chooseFile(event: Event) {
      const input = event.target as HTMLInputElement;
      this.file = input.files?.[0] || null;
      if (!this.name.trim() && this.file) this.name = this.file.name.replace(/\.json$/i, '');
    },
    submit() {
      if (!this.canSubmit) return;
      this.$emit('submit', {
        mode: this.collection ? 'edit' : this.mode,
        name: this.name.trim(),
        url: this.url.trim(),
        interval: this.interval,
        file: this.file,
      });
    },
  },
});
</script>

<template>
  <div class="overlay" @click.self="$emit('close')" @keydown.esc="$emit('close')">
    <section class="dialog" role="dialog" aria-modal="true" :aria-label="$t('sources.collections.dialogTitle')">
      <header>
        <div>
          <p class="eyebrow">{{ $t('sources.collections.eyebrow') }}</p>
          <h2>{{ collection ? $t('sources.collections.editTitle') : $t('sources.collections.addTitle') }}</h2>
        </div>
        <AppButton variant="quiet" :disabled="busy" @click="$emit('close')">{{ $t('sources.close') }}</AppButton>
      </header>
      <div v-if="!collection" class="mode-tabs">
        <button type="button" :class="{ active: mode === 'upload' }" @click="mode = 'upload'">{{ $t('sources.collections.upload') }}</button>
        <button type="button" :class="{ active: mode === 'url' }" @click="mode = 'url'">{{ $t('sources.collections.url') }}</button>
      </div>
      <label><span>{{ $t('sources.collections.name') }}</span><input v-model="name" maxlength="100" :disabled="busy"></label>
      <template v-if="!collection && mode === 'upload'">
        <label><span>{{ $t('sources.collections.file') }}</span><input type="file" accept=".json,application/json" :disabled="busy" @change="chooseFile"></label>
      </template>
      <template v-if="(!collection && mode === 'url') || collection?.originKind === 'url'">
        <label v-if="!collection"><span>{{ $t('sources.collections.remoteUrl') }}</span><input v-model="url" type="url" :disabled="busy" placeholder="https://example.com/sources.json"></label>
        <label><span>{{ $t('sources.collections.schedule') }}</span><select v-model="interval" :disabled="busy"><option value="manual">{{ $t('sources.collections.manual') }}</option><option value="daily">{{ $t('sources.collections.daily') }}</option><option value="weekly">{{ $t('sources.collections.weekly') }}</option></select></label>
      </template>
      <p v-if="error" class="error" role="alert">{{ error }}</p>
      <footer><AppButton variant="secondary" :disabled="busy" @click="$emit('close')">{{ $t('sources.cancel') }}</AppButton><AppButton :busy="busy" :disabled="!canSubmit" @click="submit">{{ $t('sources.collections.save') }}</AppButton></footer>
    </section>
  </div>
</template>

<style scoped>
.overlay{position:fixed;z-index:90;inset:0;display:grid;place-items:center;padding:1rem;background:rgb(0 0 0/.42)}.dialog{width:min(32rem,100%);display:grid;gap:1rem;padding:1rem;border-radius:var(--radius-lg);background:var(--color-paper-raised)}header,footer{display:flex;justify-content:space-between;align-items:center;gap:1rem}h2,p{margin:0}h2{font:700 1.25rem var(--font-literary)}.eyebrow{color:var(--color-warm);font-size:.7rem;font-weight:800;letter-spacing:.1em;text-transform:uppercase}.mode-tabs{display:grid;grid-template-columns:1fr 1fr;padding:.25rem;border-radius:var(--radius-md);background:var(--color-paper-muted)}.mode-tabs button{min-height:2.5rem;border:0;border-radius:calc(var(--radius-md) - .2rem);background:transparent;color:var(--color-ink-muted);font-weight:700}.mode-tabs button.active{background:white;color:var(--color-ink);box-shadow:0 1px 3px rgb(0 0 0/.08)}label{display:grid;gap:.35rem}label span{font-size:.78rem;font-weight:700}input,select{min-height:2.75rem;border:1px solid var(--color-border);border-radius:var(--radius-md);padding:.55rem .7rem;background:white;color:var(--color-ink)}.error{color:var(--color-danger)}footer{justify-content:flex-end}@media(max-width:34rem){.overlay{align-items:end;padding:0}.dialog{border-radius:var(--radius-lg) var(--radius-lg) 0 0}}
</style>
