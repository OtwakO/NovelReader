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
    <select v-model="selectedLocale" :aria-label="$t('app.common.language')">
      <option v-for="locale in locales" :key="locale.code" :value="locale.code">{{ locale.label }}</option>
    </select>
  </label>
</template>

<style scoped>
.locale-switcher select { min-height: 2.75rem; max-width: 100%; border: 1px solid var(--color-border); border-radius: var(--radius-md); padding: .45rem .65rem; background: var(--color-paper-raised); color: var(--color-ink); }
</style>
