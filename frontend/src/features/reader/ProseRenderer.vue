<script lang="ts">
import { defineComponent, h, type PropType } from 'vue';
import BookCover from '../books/BookCover.vue';
import type { ProseDocument } from '../../api/reader';
import type { ReadingTarget, StructuredProseDocument } from '../../api/structured-prose';
import { proseNodes, renderProseNodes, type ProseNavigation } from './prose-nodes';

export default defineComponent({
  name: 'ProseRenderer',
  props: {
    document: { type: Object as PropType<ProseDocument | StructuredProseDocument>, required: true },
    fallbackImageAlt: { type: String, required: true },
    imageUnavailable: { type: String, required: true },
    coverUnavailable: { type: String, default: '' },
    showImages: { type: Boolean, required: true },
    targetHref: { type: Function as PropType<(target: ReadingTarget, note: boolean) => string>, default: undefined },
  },
  emits: { navigate: (action: ProseNavigation) => Number.isSafeInteger(action.target.contentRevision) },
  data: () => ({ failedResources: new Set<string>() }),
  computed: { nodes() { return proseNodes(this.document, this.showImages); } },
  watch: { document() { this.failedResources = new Set(); } },
  methods: {
    findAnchor(anchor: string): HTMLElement | undefined {
      return Array.from((this.$el as HTMLElement).querySelectorAll<HTMLElement>('[data-prose-anchor]')).find(element => element.dataset.proseAnchor === anchor);
    },
  },
  render() {
    const structured = 'structureVersion' in this.document;
    return h('div', { class: ['prose-document', { 'structured-prose': structured }], role: 'article', 'aria-label': this.document.title }, [
      // Structured prose retains authored headings; metadata is an accessible
      // document label rather than a second, potentially duplicate heading.
      ...(!structured ? [h('h1', this.document.title)] : []),
      ...(structured && this.document.coverPlaceholder ? [h('figure', { class: 'prose-figure cover-placeholder', onClick: (event: MouseEvent) => event.stopPropagation() }, [
        ...(this.showImages ? [h(BookCover, { name: this.document.title, class: 'placeholder-cover', lazy: true })] : []),
        h('figcaption', this.coverUnavailable || this.imageUnavailable),
      ])] : []),
      ...renderProseNodes(this.nodes, {
        showImages: this.showImages, fallbackImageAlt: this.fallbackImageAlt, imageUnavailable: this.imageUnavailable,
        failedResources: this.failedResources, imageFailed: href => { this.failedResources = new Set(this.failedResources).add(href); },
        targetHref: this.targetHref, navigate: target => this.$emit('navigate', target),
      }),
    ]);
  },
});
</script>

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
  font: var(--weight-regular) .78rem/1.45 var(--font-ui);
  overflow-wrap: anywhere;
}

.placeholder-cover { width: min(100%, 16rem); }

.image-failure {
  display: inline-block;
  width: min(100%, 28rem);
  padding: 1rem;
  border: 1px solid color-mix(in srgb, currentColor 18%, transparent);
  border-radius: var(--radius-md);
  background: color-mix(in srgb, currentColor 5%, transparent);
  color: color-mix(in srgb, currentColor 72%, transparent);
  font: var(--weight-strong) .8rem/1.4 var(--font-ui);
}
.structured-prose :is(h1, h2, h3, h4, h5, h6) {
  margin: 1.5em 0 .65em;
  text-align: start;
  line-height: 1.35;
  overflow-wrap: anywhere;
}
.structured-prose h1 { font-size: 1.55em; }
.structured-prose h2 { font-size: 1.35em; }
.structured-prose :is(h3, h4, h5, h6) { font-size: 1.15em; }
.structured-prose :is(ul, ol) { padding-inline-start: 1.6em; margin-block: .85em; }
.structured-prose blockquote { margin: 1em 1.5em; }
.structured-prose pre { white-space: pre-wrap; overflow-wrap: anywhere; }
/* All displayed images, including authored inline images, are centered. */
.prose-document img { display: block; margin-inline: auto; }
.structured-prose img { max-width: 100%; height: auto; object-fit: contain; }
.structured-prose a { color: inherit; text-decoration: underline; text-underline-offset: .18em; }
.structured-prose [aria-disabled="true"] { text-decoration: underline dotted; }
.prose-table-scroll { max-width: 100%; overflow-x: auto; margin-block: 1em; }
.prose-table-scroll table { border-collapse: collapse; }
.prose-table-scroll :is(td, th) { border: 1px solid currentColor; padding: .4em .6em; text-align: start; }
.structured-prose :is(a, .prose-table-scroll):focus-visible { outline: 2px solid currentColor; outline-offset: 3px; }
</style>
