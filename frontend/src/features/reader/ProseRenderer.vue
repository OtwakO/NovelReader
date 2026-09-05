<script lang="ts">
import { defineComponent, type PropType } from 'vue';
import type { ProseDocument } from '../../api/reader';

export default defineComponent({
  name: 'ProseRenderer',
  props: {
    document: { type: Object as PropType<ProseDocument>, required: true },
    fallbackImageAlt: { type: String, required: true },
    imageUnavailable: { type: String, required: true },
    showImages: { type: Boolean, required: true },
  },
  data: () => ({ failedResources: new Set<string>() }),
  watch: {
    document() {
      this.failedResources = new Set();
    },
  },
  methods: {
    imageAlt(alt?: string): string {
      return alt || this.fallbackImageAlt;
    },
    markImageFailed(href: string) {
      this.failedResources = new Set(this.failedResources).add(href);
    },
  },
});
</script>

<template>
  <div class="prose-document">
    <h1>{{ document.title }}</h1>
    <template v-for="(block, index) in document.blocks" :key="`${block.kind}-${index}`">
      <p v-if="block.kind === 'paragraph'">{{ block.text }}</p>
      <figure v-else-if="showImages" class="prose-figure" @click.stop>
        <div v-if="failedResources.has(block.resource.href)" class="image-failure" role="status">
          {{ imageUnavailable }}
        </div>
        <img
          v-else
          :src="block.resource.href"
          :alt="imageAlt(block.alt)"
          loading="lazy"
          decoding="async"
          @error="markImageFailed(block.resource.href)"
        >
        <figcaption v-if="block.alt">{{ block.alt }}</figcaption>
      </figure>
    </template>
  </div>
</template>

<style scoped>
.prose-document h1 {
  margin: 0 0 2.5rem;
  text-align: center;
  font-size: 1.55em;
  line-height: 1.3;
}

.prose-document p {
  margin: 0 0 .85em;
  text-align: justify;
  overflow-wrap: anywhere;
}

.prose-figure {
  display: grid;
  justify-items: center;
  margin: 1.5rem 0;
  text-align: center;
}

.prose-figure img {
  display: block;
  max-width: 100%;
  width: auto;
  height: auto;
  max-height: 85dvh;
  object-fit: contain;
}

.prose-figure figcaption {
  max-width: 36rem;
  margin-top: .6rem;
  color: color-mix(in srgb, currentColor 72%, transparent);
  font: 500 .78rem/1.45 var(--font-ui);
  overflow-wrap: anywhere;
}

.image-failure {
  width: min(100%, 28rem);
  padding: 1rem;
  border: 1px solid color-mix(in srgb, currentColor 18%, transparent);
  border-radius: var(--radius-md);
  background: color-mix(in srgb, currentColor 5%, transparent);
  color: color-mix(in srgb, currentColor 72%, transparent);
  font: 600 .8rem/1.4 var(--font-ui);
}
</style>
