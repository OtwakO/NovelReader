import { parseReadingTarget, type ReadingTarget } from './reading-target';
import type { Chapter } from './models';

export interface CatalogNavigationEntry {
  label: string;
  target?: ReadingTarget;
  unavailable: boolean;
  children: CatalogNavigationEntry[];
}
export interface CatalogNavigation {
  source: 'publication' | 'sections';
  entries: CatalogNavigationEntry[];
}

export function parseCatalogNavigation(input: unknown, chapters: Chapter[], revision: number): CatalogNavigation {
  if (!input || typeof input !== 'object') throw new Error('Invalid catalog navigation');
  const value = input as Record<string, unknown>;
  if ((value.source !== 'publication' && value.source !== 'sections') || !Array.isArray(value.entries)) throw new Error('Invalid catalog navigation');
  const readable = new Set(chapters.filter(chapter => !chapter.isVolume).map(chapter => chapter.index));
  let count = 0;
  const parse = (input: unknown, depth: number): CatalogNavigationEntry => {
    if (++count > 10000 || depth > 128) throw new Error('Catalog navigation limit exceeded');
    if (!input || typeof input !== 'object') throw new Error('Invalid navigation entry');
    const node = input as Record<string, unknown>;
    if (typeof node.label !== 'string' || (node.unavailable !== undefined && typeof node.unavailable !== 'boolean') || (node.children !== undefined && !Array.isArray(node.children))) throw new Error('Invalid navigation entry');
    const target = node.target === undefined ? undefined : parseReadingTarget(node.target, revision);
    const unavailable = node.unavailable === true;
    if (target && (unavailable || !readable.has(target.chapterIndex))) throw new Error('Invalid navigation target');
    return {
      label: node.label,
      ...(target ? { target } : {}),
      unavailable,
      children: ((node.children ?? []) as unknown[]).map(child => parse(child, depth + 1)),
    };
  };
  return { source: value.source, entries: value.entries.map(entry => parse(entry, 0)) };
}
