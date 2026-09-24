<script lang="ts">
import { defineComponent, type PropType } from 'vue';
import type { TXTEncoding, TXTPreset } from '../../api/txt-imports';
export default defineComponent({
  props: {
    encoding: { type: String as PropType<TXTEncoding>, required: true },
    preset: { type: String as PropType<TXTPreset>, required: true },
    pattern: { type: String, default: '' }, busy: Boolean, patternError: Boolean,
  },
  emits: ['update:encoding', 'update:preset', 'update:pattern'],
  data: () => ({
    encodings: ['', 'utf-8', 'utf-16le', 'utf-16be', 'gb18030', 'big5'],
    presets: ['', 'chinese-chapters', 'english-chapters', 'generated-sections', 'custom'],
  }),
});
</script>
<template>
  <div class="import-fields">
    <label>{{ $t('imports.encoding') }}<select :value="encoding" :disabled="busy" @change="$emit('update:encoding', ($event.target as HTMLSelectElement).value)"><component :is="'button'" type="button"><selectedcontent /></component><option v-for="value in encodings" :key="value" :value="value">{{ value || $t('imports.automatic') }}</option></select></label>
    <label>{{ $t('imports.preset') }}<select :value="preset" :disabled="busy" @change="$emit('update:preset', ($event.target as HTMLSelectElement).value)"><component :is="'button'" type="button"><selectedcontent /></component><option v-for="value in presets" :key="value" :value="value">{{ value ? $t(`imports.presets.${value}`) : $t('imports.automatic') }}</option></select></label>
  </div>
  <div v-if="preset === 'custom'" class="import-pattern">
    <label for="heading-pattern">{{ $t('imports.pattern') }}</label>
    <textarea id="heading-pattern" :value="pattern" required rows="2" maxlength="2048" spellcheck="false" autocapitalize="off" :disabled="busy" :aria-invalid="patternError || undefined" :aria-describedby="patternError ? 'pattern-help pattern-error' : 'pattern-help'" @input="$emit('update:pattern', ($event.target as HTMLTextAreaElement).value)" />
    <p id="pattern-help">{{ $t('imports.patternHint') }}<br>{{ $t('imports.patternExample') }} <code>(?i)part [0-9]+.*</code></p>
    <p v-if="patternError" id="pattern-error" role="alert" class="import-error">{{ $t('imports.errors.pattern') }}</p>
  </div>
</template>
