import { h, type VNodeChild } from 'vue';
import type { ProseDocument } from '../../api/reader';
import type { ProseContainerKind, ReadingTarget, StructuredProseDocument, StructuredProseNode } from '../../api/structured-prose';

const tags = {
  group: 'div', inlineGroup: 'span', paragraph: 'p', quote: 'blockquote', figure: 'figure', figureCaption: 'figcaption',
  unorderedList: 'ul', listItem: 'li', table: 'table', tableHead: 'thead', tableBody: 'tbody', tableFoot: 'tfoot', row: 'tr', caption: 'caption',
  emphasis: 'em', strong: 'strong', strike: 's', subscript: 'sub', superscript: 'sup', code: 'code', preformatted: 'pre',
  break: 'br', separator: 'hr', ruby: 'ruby', rubyText: 'rt', rubyFallback: 'rp',
} satisfies Record<ProseContainerKind, string>;

/** One renderer vocabulary; the legacy adapter changes no canonical document. */
export function proseNodes(document: ProseDocument | StructuredProseDocument, showImages = true): StructuredProseNode[] {
  if ('structureVersion' in document) return document.blocks;
  return document.blocks.filter(block => showImages || block.kind !== 'image').map(block => block.kind === 'paragraph'
    ? { kind: 'paragraph', children: [{ kind: 'text', text: block.text, children: [] }] }
    : { kind: 'figure', children: [
      { kind: 'image', resource: block.resource, alt: block.alt, children: [] },
      ...(block.alt ? [{ kind: 'figureCaption' as const, children: [{ kind: 'text' as const, text: block.alt, children: [] }] }] : []),
    ] });
}

export interface ProseNavigation { target: ReadingTarget; note: boolean }

interface RenderOptions {
  showImages: boolean; fallbackImageAlt: string; imageUnavailable: string;
  failedResources: ReadonlySet<string>; imageFailed: (href: string) => void;
  targetHref?: (target: ReadingTarget) => string;
  navigate: (action: ProseNavigation) => void;
}

export function renderProseNodes(nodes: StructuredProseNode[], options: RenderOptions): VNodeChild[] {
  const render = (node: StructuredProseNode): VNodeChild => {
    // Do not spread source objects into DOM properties. Anchors are scoped data,
    // not global IDs or CSS selectors, and cannot clobber application elements.
    const attrs = { 'data-prose-anchor': node.id, 'data-prose-role': node.role, lang: node.language, dir: node.direction };
    const children = () => node.children.map(render);
    switch (node.kind) {
      case 'text': return node.text;
      case 'image': {
        if (!options.showImages) return null;
        if (options.failedResources.has(node.resource.href)) return h('span', { ...attrs, class: 'image-failure', role: 'status' }, node.alt ? `${node.alt} — ${options.imageUnavailable}` : options.imageUnavailable);
        return h('img', { ...attrs, src: node.resource.href, alt: node.alt || options.fallbackImageAlt, width: node.width, height: node.height,
          loading: 'lazy', decoding: 'async', onClick: (event: MouseEvent) => event.stopPropagation(), onError: () => options.imageFailed(node.resource.href) });
      }
      case 'link': {
        const href = node.url ?? (node.target && options.targetHref?.(node.target));
        if (node.unavailable || !href) return h('span', { ...attrs, role: 'link', 'aria-disabled': 'true' }, children());
        return h('a', { ...attrs, href, ...(node.url ? { target: '_blank', rel: 'noopener noreferrer', referrerpolicy: 'no-referrer' } : {}),
          onClick: (event: MouseEvent) => {
            event.stopPropagation();
            if (node.target && !event.ctrlKey && !event.metaKey && !event.shiftKey && !event.altKey && event.button === 0) {
              event.preventDefault(); options.navigate({ target: node.target, note: node.role === 'noteref' });
            }
          },
        }, children());
      }
      case 'heading': return h(`h${node.level}`, attrs, children());
      case 'orderedList': return h('ol', { ...attrs, start: node.start }, children());
      case 'cell': case 'headerCell': return h(node.kind === 'cell' ? 'td' : 'th', { ...attrs, colspan: node.colSpan, rowspan: node.rowSpan }, children());
      case 'figure':
        return h('figure', { ...attrs, class: 'prose-figure', onClick: (event: MouseEvent) => event.stopPropagation() }, children());
      case 'table': return h('div', { class: 'prose-table-scroll', tabindex: 0 }, [h('table', attrs, children())]);
      default: return h(tags[node.kind], attrs, children());
    }
  };
  return nodes.map(render);
}
