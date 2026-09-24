import { afterEach, expect, it, vi } from 'vitest';
import { getEPUBPreviewNavigation, getEPUBPreviewSection } from './epub-review';

const document = { kind: 'prose', title: '', blocks: [{ kind: 'link', unavailable: true, children: [{ kind: 'text', text: 'Note' }] }] };
const response = (body: unknown) => new Response(JSON.stringify(body), { status: 200 });
afterEach(() => vi.unstubAllGlobals());

it('parses generation-qualified contents and safe prose without manufacturing a reading revision', async () => {
  const fetch = vi.fn()
    .mockResolvedValueOnce(response({ generation: 3, source: 'publication', entries: [{ label: 'Chapter', target: { section: 1, anchor: 'a1' }, children: [] }] }))
    .mockResolvedValueOnce(response({ generation: 3, section: 1, version: 2, document }));
  vi.stubGlobal('fetch', fetch);
  const signal = new AbortController().signal;
  const contents = await getEPUBPreviewNavigation('receipt', 3, 2, signal);
  expect(contents.entries[0]?.target).toEqual({ section: 1, anchor: 'a1' });
  const prose = await getEPUBPreviewSection('receipt', 3, 1, signal);
  expect(prose.blocks[0]).toMatchObject({ kind: 'link', unavailable: true });
  expect(JSON.stringify(prose)).not.toContain('contentRevision');
});

it('rejects stale preview identity and published internal targets', async () => {
  vi.stubGlobal('fetch', vi.fn()
    .mockResolvedValueOnce(response({ generation: 2, section: 1, version: 2, document }))
    .mockResolvedValueOnce(response({ generation: 3, section: 1, version: 2, document: { ...document, blocks: [{ kind: 'link', target: { chapterIndex: 1, contentRevision: 3 } }] } })));
  const signal = new AbortController().signal;
  await expect(getEPUBPreviewSection('receipt', 3, 1, signal)).rejects.toMatchObject({ code: 'epub_state_changed' });
  await expect(getEPUBPreviewSection('receipt', 3, 1, signal)).rejects.toThrow('Unexpected published target');
});
