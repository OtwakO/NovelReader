<script lang="ts">
import { defineComponent, type CSSProperties, type PropType } from 'vue';
import AppIcon from './AppIcon.vue';

interface SearchSelectOption { value: string; label: string; description?: string }

export default defineComponent({
  name: 'AppSearchSelect',
  components: { AppIcon },
  props: {
    id: { type: String, required: true },
    modelValue: { type: String, required: true },
    label: { type: String, required: true },
    placeholder: { type: String, required: true },
    searchLabel: { type: String, required: true },
    emptyLabel: { type: String, required: true },
    options: { type: Array as PropType<SearchSelectOption[]>, required: true },
    disabled: Boolean,
  },
  emits: ['update:modelValue'],
  data: () => ({ open: false, query: '', active: 0, panelStyle: {} as CSSProperties }),
  computed: {
    selected(): SearchSelectOption | undefined { return this.options.find(option => option.value === this.modelValue); },
    filtered(): SearchSelectOption[] {
      const query = this.query.trim().toLocaleLowerCase();
      return this.options.filter(option => `${option.label} ${option.description || ''}`.toLocaleLowerCase().includes(query));
    },
    activeId(): string | undefined { return this.filtered[this.active] ? `${this.id}-option-${this.active}` : undefined; },
  },
  watch: {
    query() { this.active = 0; this.revealActive(); },
    options() { this.active = 0; },
    disabled(value: boolean) { if (value) this.close(); },
  },
  mounted() {
    document.addEventListener('pointerdown', this.outside);
    window.addEventListener('resize', this.positionPanel);
    window.addEventListener('scroll', this.positionPanel, true);
    window.visualViewport?.addEventListener('resize', this.positionPanel);
    window.visualViewport?.addEventListener('scroll', this.positionPanel);
  },
  beforeUnmount() {
    document.removeEventListener('pointerdown', this.outside);
    window.removeEventListener('resize', this.positionPanel);
    window.removeEventListener('scroll', this.positionPanel, true);
    window.visualViewport?.removeEventListener('resize', this.positionPanel);
    window.visualViewport?.removeEventListener('scroll', this.positionPanel);
  },
  methods: {
    async show() {
      if (this.disabled) return;
      this.query = '';
      this.open = true;
      await this.$nextTick();
      this.active = Math.max(0, this.filtered.findIndex(option => option.value === this.modelValue));
      this.positionPanel();
      (this.$refs.search as HTMLInputElement | undefined)?.focus();
      this.revealActive();
    },
    positionPanel() {
      if (!this.open) return;
      const trigger = (this.$refs.trigger as HTMLElement).getBoundingClientRect();
      const viewport = window.visualViewport;
      const top = viewport?.offsetTop || 0;
      const bottom = top + (viewport?.height || window.innerHeight);
      const below = Math.max(0, bottom - trigger.bottom - 8);
      const above = Math.max(0, trigger.top - top - 8);
      const upward = below < 280 && above > below;
      this.panelStyle = {
        left: `${trigger.left}px`, width: `${trigger.width}px`,
        top: upward ? 'auto' : `${trigger.bottom + 4}px`,
        bottom: upward ? `${window.innerHeight - trigger.top + 4}px` : 'auto',
        maxHeight: `${upward ? above : below}px`,
      };
    },
    close(restoreFocus = false) {
      this.open = false;
      if (restoreFocus) (this.$refs.trigger as HTMLButtonElement).focus();
    },
    outside(event: PointerEvent) { if (!(this.$el as HTMLElement).contains(event.target as Node)) this.close(); },
    blur(event: FocusEvent) { if (!(this.$el as HTMLElement).contains(event.relatedTarget as Node | null)) this.close(); },
    choose(option: SearchSelectOption) {
      this.close(true);
      if (option.value !== this.modelValue) this.$emit('update:modelValue', option.value);
    },
    revealActive() {
      void this.$nextTick(() => {
        if (this.open && this.activeId) document.getElementById(this.activeId)?.scrollIntoView?.({ block: 'nearest' });
      });
    },
    keydown(event: KeyboardEvent) {
      if (event.isComposing) return;
      if (event.key === 'Escape') { event.preventDefault(); event.stopPropagation(); this.close(true); }
      else if (event.key === 'Tab') this.close(true);
      else if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
        event.preventDefault();
        this.active = Math.max(0, Math.min(this.filtered.length - 1, this.active + (event.key === 'ArrowDown' ? 1 : -1)));
        this.revealActive();
      } else if (event.key === 'Enter') {
        event.preventDefault();
        const option = this.filtered[this.active];
        if (option) this.choose(option);
      }
    },
  },
});
</script>

<template>
  <div class="search-select" @focusout="blur">
    <label :id="`${id}-label`" :for="id">{{ label }}</label>
    <button :id="id" ref="trigger" type="button" class="search-select-trigger" :disabled="disabled" :aria-labelledby="`${id}-label ${id}-value`" :aria-expanded="open" :aria-controls="`${id}-panel`" aria-haspopup="dialog" @click="open ? close() : show()" @keydown.down.prevent="show" @keydown.up.prevent="show">
      <span :id="`${id}-value`" class="selection"><span>{{ selected?.label || placeholder }}</span><small v-if="selected?.description">{{ selected.description }}</small></span>
      <svg aria-hidden="true" viewBox="0 0 20 20" :class="{ expanded: open }"><path d="m5 8 5 5 5-5" /></svg>
    </button>
    <div v-if="open" :id="`${id}-panel`" class="search-select-panel" :style="panelStyle" role="dialog" :aria-labelledby="`${id}-label`">
      <div class="search-field"><AppIcon name="search" /><input ref="search" v-model="query" role="combobox" type="text" autocomplete="off" :aria-label="searchLabel" :placeholder="searchLabel" aria-autocomplete="list" aria-expanded="true" :aria-controls="`${id}-list`" :aria-activedescendant="activeId" @keydown="keydown"></div>
      <ul :id="`${id}-list`" role="listbox" :aria-label="label">
        <li v-for="(option, index) in filtered" :id="`${id}-option-${index}`" :key="option.value" role="option" :aria-selected="option.value === modelValue" :class="{ active: index === active }" @mousedown.prevent @click="choose(option)">
          <span class="option-copy"><span>{{ option.label }}</span><small v-if="option.description">{{ option.description }}</small></span><AppIcon v-if="option.value === modelValue" name="check" />
        </li>
      </ul>
      <p v-if="!filtered.length" class="no-options" role="status">{{ emptyLabel }}</p>
    </div>
  </div>
</template>

<style scoped>
.search-select{position:relative;min-width:0;display:grid;gap:var(--space-2)}
.search-select>label{font-size:var(--text-small);color:var(--color-ink-muted)}
.search-select-trigger{width:100%;min-height:var(--control-height);display:flex;align-items:center;justify-content:space-between;gap:var(--space-3);padding:.625rem .75rem;border:1px solid var(--color-border);border-radius:var(--radius-md);background:var(--color-paper-raised);color:var(--color-ink);text-align:start;cursor:pointer}
.selection,.option-copy{min-width:0;display:grid;gap:.125rem}.selection>span{overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
small{font-size:var(--text-caption);color:var(--color-ink-muted);overflow-wrap:anywhere}
.search-select-trigger svg{flex:none;width:1rem;height:1rem;fill:none;stroke:currentColor;stroke-width:1.75}.expanded{transform:rotate(180deg)}
.search-select-trigger:disabled{opacity:.6;cursor:not-allowed}
.search-select-trigger:focus-visible{outline:2px solid var(--color-accent);outline-offset:3px}
.search-select-panel{position:fixed;z-index:100;display:flex;flex-direction:column;overflow:hidden;border:1px solid var(--color-border);border-radius:var(--radius-md);background:var(--color-paper-raised);box-shadow:0 .5rem 1.5rem rgb(54 39 26 / .15)}
.search-field{flex:none;display:flex;align-items:center;gap:var(--space-2);padding:var(--space-3);border-bottom:1px solid var(--color-border);color:var(--color-ink-muted)}
.search-field input{min-width:0;width:100%;min-height:var(--control-height);padding:.5rem;border:1px solid var(--color-border);border-radius:var(--radius-sm);background:var(--color-paper);color:var(--color-ink)}
ul{min-height:0;list-style:none;margin:0;padding:var(--space-2);max-height:min(18rem,35dvh);overflow-y:auto;overscroll-behavior:contain}
li{min-height:var(--control-height);display:flex;align-items:center;justify-content:space-between;gap:var(--space-2);padding:.625rem .75rem;border-radius:var(--radius-sm);cursor:pointer;overflow-wrap:anywhere}
li.active,li:hover{background:var(--color-paper-muted)}li[aria-selected="true"]{color:var(--color-accent-strong)}
.no-options{margin:0;padding:var(--space-4);color:var(--color-ink-muted);font-size:var(--text-small)}
</style>
