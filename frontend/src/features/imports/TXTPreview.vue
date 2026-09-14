<script lang="ts">
import { defineComponent, type PropType } from 'vue';
import type { TXTPreview } from '../../api/txt-imports';
import AppButton from '../../ui/components/AppButton.vue';
export default defineComponent({
  components: { AppButton },
  props: { preview: { type: Object as PropType<TXTPreview>, required: true }, start: { type: Number, default: 0 }, pattern: { type: String, default: '' }, busy: Boolean, compact: Boolean },
  emits: ['page'],
});
</script>
<template>
  <section class="import-section" aria-labelledby="preview-title">
    <h2 id="preview-title">{{ $t(compact ? 'imports.flow.preview' : 'imports.preview') }}</h2>
    <p v-if="compact">{{ $t('imports.flow.chapterCount', { count: preview.totalSections }) }}</p>
    <template v-else>
    <p>{{ $t('imports.previewSummary', { count: preview.totalSections, encoding: preview.encoding }) }}</p>
    <p>{{ $t('imports.preset') }}: {{ $t(`imports.presets.${preview.preset}`) }}</p>
    <p v-if="pattern">{{ $t('imports.savedPattern') }}: <code>{{ pattern }}</code></p>
    <p v-for="reason in preview.reviewReasons || []" :key="reason">{{ $t(`imports.reasons.${reason}`) }}</p>
    </template>
    <div class="import-preview">
      <div>
        <h3>{{ $t(compact ? 'imports.flow.chapters' : 'imports.headings') }}</h3>
        <ol :start="start + 1"><li v-for="heading in preview.headings" :key="heading.index">{{ heading.title }} <small v-if="heading.generated && !compact">({{ $t('imports.generated') }})</small> <slot name="heading-action" :heading="heading" /></li></ol>
        <div v-if="start > 0 || preview.hasMore" class="import-actions"><AppButton variant="secondary" :disabled="start === 0 || busy" @click="$emit('page', Math.max(0, start - 25))">{{ $t('imports.previous') }}</AppButton><AppButton variant="secondary" :disabled="!preview.hasMore || busy" @click="$emit('page', start + 25)">{{ $t('imports.next') }}</AppButton></div>
      </div>
      <div><h3>{{ $t(compact ? 'imports.flow.textPreview' : 'imports.sample') }}</h3><pre class="import-sample">{{ preview.sample }}</pre><p v-if="preview.sampleTruncated">{{ $t(compact ? 'imports.flow.sampleNote' : 'imports.truncated') }}</p></div>
    </div>
  </section>
</template>
