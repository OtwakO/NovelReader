import type { Chapter } from './models';
import { parseCatalogNavigation, type CatalogNavigation } from './catalog-navigation';

export interface ChapterCatalog { sourceIdentity?: string; chapters: Chapter[]; contentRevision: number; navigation?: CatalogNavigation }

/** Main progression follows section order, not TOC order; filtering never renumbers indices. */
export function parseChapterCatalog(chapters: unknown[], contentRevision: number, navigation?: unknown, sourceIdentity?: unknown): ChapterCatalog {
  if (sourceIdentity !== undefined && (typeof sourceIdentity !== 'string' || sourceIdentity.length === 0)) throw new Error('Invalid source identity');
  const indices = new Set<number>();
  const parsed = chapters.map(input => {
    if (!input || typeof input !== 'object') throw new Error('Invalid catalog section');
    const value = input as Record<string, unknown>;
    if (typeof value.index !== 'number' || !Number.isSafeInteger(value.index) || value.index < 0 || indices.has(value.index) || typeof value.title !== 'string') throw new Error('Invalid catalog section');
    if (value.isVolume !== undefined && typeof value.isVolume !== 'boolean') throw new Error('Invalid catalog volume flag');
    if (value.auxiliary !== undefined && typeof value.auxiliary !== 'boolean') throw new Error('Invalid catalog auxiliary flag');
    if (value.isVolume && value.auxiliary) throw new Error('Volume cannot be an auxiliary section');
    indices.add(value.index);
    return { index: value.index, title: value.title, isVolume: value.isVolume === true, ...(value.auxiliary === undefined ? {} : { auxiliary: value.auxiliary }) };
  });
  return { ...(typeof sourceIdentity === 'string' ? { sourceIdentity } : {}), contentRevision, chapters: parsed, ...(navigation === undefined ? {} : { navigation: parseCatalogNavigation(navigation, parsed, contentRevision) }) };
}
