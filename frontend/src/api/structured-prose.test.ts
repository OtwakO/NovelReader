import { describe, expect, it } from 'vitest';
import { parseStructuredChapterContent, mapStructuredText, structuredTextValues } from './structured-prose';
import { structuredProseFixture } from './structured-prose.fixture';

describe('candidate structured prose boundary', () => {
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
