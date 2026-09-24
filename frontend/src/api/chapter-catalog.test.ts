import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { afterEach, expect, it, vi } from 'vitest';
import { parseChapterCatalog } from './chapter-catalog';
import { waitForCatalog } from './reader';

// Owned and checked by the Go reading projection; no real publication content.
const wire: { chapters: unknown[]; contentRevision: number; navigation: unknown } = JSON.parse(readFileSync(resolve(import.meta.dirname, '../../../backend/internal/reading/testdata/epub-navigation.json'), 'utf8'));
afterEach(() => vi.unstubAllGlobals());

it('retains the Go navigation contract through catalog transport without confusing TOC order with reading order', async () => {
  vi.stubGlobal('fetch', vi.fn(async () => new Response(JSON.stringify(wire), { status: 200 })));
  const catalog = await waitForCatalog('book');
  expect(catalog.chapters.map(chapter => chapter.index)).toEqual([0, 1, 2]);
  expect(catalog.navigation?.source).toBe('publication');
  const group = catalog.navigation!.entries[0]!;
  expect(group.target).toBeUndefined();
  expect(group.children.map(entry => entry.target?.chapterIndex)).toEqual([2, 0, 0, undefined]);
  expect(group.children[1]?.target).toEqual({ chapterIndex: 0, contentRevision: 9, anchor: 'a1' });
  expect(group.children[3]).toMatchObject({ unavailable: true, children: [{ label: 'Notes', target: { chapterIndex: 1, contentRevision: 9, anchor: 'a3' } }] });
  expect(catalog.chapters[1]?.title).toBe('');
});

it('distinguishes an explicit section-list fallback from a legacy catalog with no navigation', () => {
  expect(parseChapterCatalog(wire.chapters, 9)).not.toHaveProperty('navigation');
  expect(parseChapterCatalog(wire.chapters, 9, { source: 'sections', entries: [] }).navigation).toEqual({ source: 'sections', entries: [] });
});

it('rejects stale, missing, volume and contradictory targets at the catalog boundary', () => {
  const chapters = [...wire.chapters, { index: 3, title: 'Volume', isVolume: true }];
  const target = { chapterIndex: 0, contentRevision: 9 };
  for (const entry of [
    { label: 'Stale', target: { ...target, contentRevision: 8 } },
    { label: 'Missing', target: { ...target, chapterIndex: 99 } },
    { label: 'Volume', target: { ...target, chapterIndex: 3 } },
    { label: 'Conflicting', unavailable: true, target },
  ]) expect(() => parseChapterCatalog(chapters, 9, { source: 'publication', entries: [entry] })).toThrow();
});

it('bounds recursive navigation independently of the document tree', () => {
  let entry: unknown = { label: 'Leaf' };
  for (let depth = 0; depth < 129; depth++) entry = { label: 'Group', children: [entry] };
  for (const entries of [[entry], Array.from({ length: 10001 }, () => ({ label: 'Heading' }))]) {
    expect(() => parseChapterCatalog(wire.chapters, 9, { source: 'publication', entries })).toThrow('limit');
  }
});
