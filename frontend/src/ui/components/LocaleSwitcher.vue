<script lang="ts">
import { defineComponent } from 'vue';
import { localeFromValue, setLocale } from '../../i18n';
import { supportedLocales } from '../../i18n/locales';

export default defineComponent({
  name: 'LocaleSwitcher',
  data() { return { locales: supportedLocales }; },
  computed: {
    selectedLocale: {
      get(): string { return String(this.$i18n.locale); },
      async set(value: string) { await setLocale(localeFromValue(value)); },
    },
  },
});
</script>

<template>
  <label class="locale-switcher">
    <span class="sr-only">{{ $t('app.common.language') }}</span>
    <select v-model="selectedLocale" :aria-label="$t('app.common.language')"><component :is="'button'" type="button"><selectedcontent /></component>
      <option v-for="locale in locales" :key="locale.code" :value="locale.code">{{ locale.label }}</option>
    </select>
  </label>
</template>

<style scoped>
.locale-switcher select { min-height: 2.75rem; max-width: 100%; border: 1px solid var(--color-border); border-radius: var(--radius-md); padding: .45rem .65rem; text-align: center; text-align-last: center; background: var(--color-paper-raised); color: var(--color-ink); }
@supports (appearance: base-select) {
  .locale-switcher select { display: grid; grid-template-columns: 1rem minmax(0, 1fr) 1rem; gap: .25rem; }
  .locale-switcher select > button { grid-column: 2; grid-row: 1; justify-content: center; }
  .locale-switcher select::picker-icon { grid-column: 3; grid-row: 1; justify-self: center; margin: 0; }
}
</style>
