import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { getChapterContent } from './reader';
import { parseChapterCatalog } from './chapter-catalog';
import { parseStructuredChapterContent, mapStructuredText, structuredTextValues, type StructuredProseNode } from './structured-prose';
import { structuredProseFixture } from './structured-prose.fixture';

afterEach(() => vi.unstubAllGlobals());

describe('structured prose boundary', () => {
  it('projects finite nodes and display text without retaining publisher fields or changing actions', () => {
    const fixture = structuredProseFixture();
    Object.assign(fixture.document.blocks[0]!, { style: 'position:fixed', onclick: 'attack()', privatePath: 'OPS/main.xhtml' });
    const content = parseStructuredChapterContent(fixture);
    const original = JSON.stringify(content);
    const converted = mapStructuredText(content.document, text => `converted:${text}`);
    expect(structuredTextValues(converted)).toEqual(structuredTextValues(content.document).map(text => `converted:${text}`));
    expect(JSON.stringify(content)).toBe(original);
    expect(JSON.stringify(content)).not.toMatch(/onclick|privatePath|position:fixed/);
    expect(JSON.stringify(converted)).toContain('https://example.invalid/软件');
    expect(JSON.stringify(converted)).toContain('"anchor":"a1"');
    expect(content.document.structureVersion).toBe(2);
  });

  it('rejects unsafe actions, resource escapes, incoherent targets, unknown kinds and excessive depth', () => {
    const badNodes: unknown[] = [
      { kind: 'script', text: 'attack()' },
      { kind: 'text', text: 'visible', children: [{ kind: 'text', text: 'must not disappear' }] },
      { kind: 'link', url: 'javascript:alert(1)' },
      { kind: 'link', target: { chapterIndex: 1, contentRevision: 8 } },
      { kind: 'link', unavailable: true, url: 'https://example.invalid' },
      { kind: 'image', resource: { href: '/api/../outside', mediaType: 'image/png' }, width: 1, height: 1 },
      { kind: 'image', resource: { href: '/api/image', mediaType: 'image/svg+xml' }, width: 1, height: 1 },
    ];
    let deep: unknown = { kind: 'text', text: 'leaf' };
    for (let i = 0; i < 129; i++) deep = { kind: 'group', children: [deep] };
    for (const node of [...badNodes, deep]) {
      expect(() => parseStructuredChapterContent({ version: 2, contentRevision: 9, document: { kind: 'prose', title: 'Proof', blocks: [node] } })).toThrow();
    }
  });
});

// This fixture is checked against Go preparation/projection, not just Go types.
it('accepts prepared Go prose through transport and resolves its note target without losing semantic fields', async () => {
  const wire = JSON.parse(readFileSync(resolve(import.meta.dirname, '../../../backend/internal/reading/testdata/epub-reading.json'), 'utf8'));
  const catalog = parseChapterCatalog(wire.catalog.chapters, wire.catalog.contentRevision, wire.catalog.navigation);
  const fetch = vi.fn();
  (wire.contents as unknown[]).forEach(content => fetch.mockResolvedValueOnce(new Response(JSON.stringify(content), { status: 200 })));
  vi.stubGlobal('fetch', fetch);
  const results = await Promise.all((wire.contents as unknown[]).map((_, index) => getChapterContent('proof', index, 9)));
  const contents = results.map(content => {
    if (content.version !== 2) throw new Error('Expected structured reading content');
    return content;
  });
  expect(contents).toHaveLength(catalog.chapters.length);
  expect(catalog.chapters.map(chapter => chapter.auxiliary === true)).toEqual([false, true]);
  const nodes: StructuredProseNode[] = [];
  const visit = (node: StructuredProseNode) => { nodes.push(node); node.children.forEach(visit); };
  contents[0]!.document.blocks.forEach(visit);
  expect(nodes.find(node => node.kind === 'orderedList')).toMatchObject({ start: 0 });
  expect(nodes.find(node => node.kind === 'headerCell')).toMatchObject({ rowSpan: 2 });
  expect(nodes.find(node => node.kind === 'cell')).toMatchObject({ colSpan: 2 });
  expect(nodes.find(node => node.kind === 'image')).toMatchObject({ width: 40, height: 30, alt: 'A small map', resource: { mediaType: 'image/png', href: '/api/books/proof/chapters/0/resources/r1?contentRevision=9' } });
  const note = nodes.find(node => node.kind === 'link' && node.role === 'noteref');
  if (note?.kind !== 'link' || !note.target) throw new Error('Missing prepared note target');
  expect(note.target.contentRevision).toBe(catalog.contentRevision);
  nodes.length = 0;
  contents[note.target.chapterIndex]!.document.blocks.forEach(visit);
  expect(nodes.find(node => node.id === note.target!.anchor)).toMatchObject({ role: 'footnote' });
  expect(structuredTextValues(contents[0]!.document)).toContain('fallback text');
});
