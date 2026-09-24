import { parseReadingTarget, type ReadingTarget } from './reading-target';
import { parseContentResource, type ContentResourceReference } from './content-resource';

export const proseContainerKinds = ['group', 'inlineGroup', 'paragraph', 'quote', 'figure', 'figureCaption', 'unorderedList', 'listItem', 'table', 'tableHead', 'tableBody', 'tableFoot', 'row', 'caption', 'emphasis', 'strong', 'strike', 'subscript', 'superscript', 'code', 'preformatted', 'break', 'separator', 'ruby', 'rubyText', 'rubyFallback'] as const;
export type ProseContainerKind = typeof proseContainerKinds[number];
export type { ReadingTarget } from './reading-target';
interface NodeAttributes {
  id?: string; language?: string; direction?: 'ltr' | 'rtl' | 'auto'; role?: 'footnote' | 'endnote' | 'noteref';
  children: StructuredProseNode[];
}
export type StructuredProseNode = NodeAttributes & (
  | { kind: ProseContainerKind }
  | { kind: 'text'; text: string }
  | { kind: 'heading'; level: number }
  | { kind: 'orderedList'; start?: number }
  | { kind: 'cell' | 'headerCell'; colSpan?: number; rowSpan?: number }
  | { kind: 'link'; target?: ReadingTarget; url?: string; unavailable: boolean }
  | { kind: 'image'; resource: ContentResourceReference; alt?: string; width?: number; height?: number }
);
// Client-only discriminator: wire version stays on the content envelope. Legacy
// callers can keep their existing documents without guessing from node fields.
export interface StructuredProseDocument { kind: 'prose'; structureVersion: 2; title: string; coverPlaceholder?: boolean; blocks: StructuredProseNode[] }
export interface StructuredChapterContent { version: 2; contentRevision: number; document: StructuredProseDocument; offlineCopy: boolean }

function object(value: unknown): Record<string, unknown> {
  if (!value || typeof value !== 'object' || Array.isArray(value)) throw new Error('Invalid structured prose object');
  return value as Record<string, unknown>;
}
function text(value: unknown): string {
  if (typeof value !== 'string') throw new Error('Invalid structured prose text');
  return value;
}
function integer(value: unknown, min: number, max = Number.MAX_SAFE_INTEGER): number {
  if (typeof value !== 'number' || !Number.isSafeInteger(value) || value < min || value > max) throw new Error('Invalid structured prose number');
  return value;
}
function choice<T extends string>(value: unknown, choices: readonly T[]): T {
  if (typeof value !== 'string' || !choices.includes(value as T)) throw new Error('Invalid structured prose kind or attribute');
  return value as T;
}
function externalURL(value: unknown): string {
  const raw = text(value);
  const url = new URL(raw);
  if (!['http:', 'https:'].includes(url.protocol) || url.username || url.password) throw new Error('Invalid prose external link');
  return raw;
}

/** Strict version-2 boundary; legacy content retains its separate version-1 parser. */
export function parseStructuredChapterContent(input: unknown): StructuredChapterContent {
  const data = object(input);
  if (data.version !== 2) throw new Error('Unsupported structured prose version');
  const revision = integer(data.contentRevision, 0);
  return { version: 2, contentRevision: revision, offlineCopy: Boolean(data.offlineCopy), document: parseStructuredProseDocument(data.document, revision) };
}

/** Without a publication revision, internal targets are forbidden (import preview). */
export function parseStructuredProseDocument(input: unknown, revision?: number): StructuredProseDocument {
  const doc = object(input);
  if (doc.kind !== 'prose' || !Array.isArray(doc.blocks)) throw new Error('Invalid structured prose document');
  // Match preparation's XML tree bounds. This HTTP boundary must not recursively
  // mount arbitrarily deep or large JSON even if a server response is malformed.
  let nodes = 0;
  const parse = (input: unknown, depth: number): StructuredProseNode => {
    if (++nodes > 200000 || depth > 128) throw new Error('Structured prose limit exceeded');
    const value = object(input);
    if (value.children !== undefined && !Array.isArray(value.children)) throw new Error('Invalid prose children');
    const attrs: NodeAttributes = {
      ...(value.id !== undefined ? { id: text(value.id) } : {}),
      ...(value.language !== undefined ? { language: text(value.language) } : {}),
      ...(value.direction !== undefined ? { direction: choice(value.direction, ['ltr', 'rtl', 'auto'] as const) } : {}),
      ...(value.role !== undefined ? { role: choice(value.role, ['footnote', 'endnote', 'noteref'] as const) } : {}),
      children: ((value.children ?? []) as unknown[]).map(child => parse(child, depth + 1)),
    };
    if (['text', 'image', 'break', 'separator'].includes(String(value.kind)) && attrs.children.length) throw new Error('Prose leaf has children');
    switch (value.kind) {
      case 'text': return { ...attrs, kind: 'text', text: text(value.text) };
      case 'heading': return { ...attrs, kind: 'heading', level: integer(value.level, 1, 6) };
      case 'orderedList': return { ...attrs, kind: 'orderedList', ...(value.start !== undefined ? { start: integer(value.start, -2147483648, 2147483647) } : {}) };
      case 'cell': case 'headerCell': return { ...attrs, kind: value.kind,
        ...(value.colSpan !== undefined ? { colSpan: integer(value.colSpan, 1, 1000) } : {}),
        ...(value.rowSpan !== undefined ? { rowSpan: integer(value.rowSpan, 1, 1000) } : {}),
      };
      case 'image': {
        const resource = parseContentResource(value.resource);
        choice(resource.mediaType, ['image/png', 'image/jpeg', 'image/webp']);
        return { ...attrs, kind: 'image', resource, width: integer(value.width, 1, 16384), height: integer(value.height, 1, 16384), ...(value.alt !== undefined ? { alt: text(value.alt) } : {}) };
      }
      case 'link': {
        if (value.unavailable !== undefined && typeof value.unavailable !== 'boolean') throw new Error('Invalid prose link state');
        const unavailable = value.unavailable === true;
        let target: ReadingTarget | undefined;
        if (value.target !== undefined) {
          if (revision === undefined) throw new Error('Unexpected published target in preview');
          target = parseReadingTarget(value.target, revision);
        }
        const url = value.url !== undefined ? externalURL(value.url) : undefined;
        if ((unavailable && (target || url)) || (!unavailable && Number(Boolean(target)) + Number(Boolean(url)) !== 1)) throw new Error('Invalid prose link action');
        return { ...attrs, kind: 'link', unavailable, ...(target ? { target } : {}), ...(url ? { url } : {}) };
      }
      default: return { ...attrs, kind: choice(value.kind, proseContainerKinds) };
    }
  };
  if (doc.coverPlaceholder !== undefined && typeof doc.coverPlaceholder !== 'boolean') throw new Error('Invalid cover placeholder');
  return { kind: 'prose', structureVersion: 2, title: text(doc.title), ...(doc.coverPlaceholder ? { coverPlaceholder: true } : {}), blocks: doc.blocks.map(node => parse(node, 1)) };
}

/** Traverse only displayed text; identifiers, resources and actions stay intact. */
export function mapStructuredText(document: StructuredProseDocument, convert: (text: string) => string): StructuredProseDocument {
  const visit = (node: StructuredProseNode): StructuredProseNode => ({ ...node,
    ...(node.kind === 'text' ? { text: convert(node.text) } : {}),
    ...(node.kind === 'image' && node.alt ? { alt: convert(node.alt) } : {}),
    children: node.children.map(visit),
  });
  return { ...document, title: convert(document.title), blocks: document.blocks.map(visit) };
}

export function structuredTextValues(document: StructuredProseDocument): string[] {
  const texts = [document.title];
  const visit = (node: StructuredProseNode) => {
    if (node.kind === 'text') texts.push(node.text);
    if (node.kind === 'image' && node.alt) texts.push(node.alt);
    node.children.forEach(visit);
  };
  document.blocks.forEach(visit);
  return texts;
}
