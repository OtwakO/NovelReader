import { flushPromises, mount, type VueWrapper } from '@vue/test-utils';
import { afterEach, expect, it, vi } from 'vitest';
import * as api from '../../api/txt-imports';
import TXTPreview from './TXTPreview.vue';

let view: VueWrapper;
afterEach(() => { view?.unmount(); vi.restoreAllMocks(); });
function page(start = 0): api.TXTPreview {
  return { analysisVersion: 1, encoding: 'utf-8', preset: 'english-chapters', parserVersion: 1, reviewReasons: [], totalSections: 27, headings: Array.from({ length: Math.min(25, 27 - start) }, (_, i) => ({ index: start + i, title: `Chapter ${start + i + 1}`, generated: false })), hasMore: start === 0, sample: 'Short summary', sampleTruncated: true };
}
const options = { props: { sourceId: 'receipt', preview: page() }, global: { mocks: { $t: (key: string) => key } } };

it('shows a whole selected chapter and follows it across bounded contents pages', async () => {
  const text = '<script>Literal text</script>\n' + 'Full chapter.\n'.repeat(400);
  const section = vi.spyOn(api, 'getTXTPreviewSection').mockImplementation(async (_id, generation, index) => ({ generation, index, title: `Chapter ${index + 1}`, text }));
  const summary = vi.spyOn(api, 'previewTXT').mockImplementation(async (_id, _version, start) => page(start));
  view = mount(TXTPreview, options); await flushPromises();
  expect(view.get('pre').text()).toBe(text.trim());
  expect(view.find('script').exists()).toBe(false);
  expect(view.get('button[aria-label="imports.sectionPreview.previous"]').attributes('disabled')).toBeDefined();
  await view.findAll('.preview-entries button')[24]!.trigger('click'); await flushPromises();
  await view.get('button[aria-label="imports.sectionPreview.next"]').trigger('click'); await flushPromises();
  expect(summary).toHaveBeenCalledExactlyOnceWith('receipt', 1, 25, expect.any(AbortSignal));
  expect(section).toHaveBeenLastCalledWith('receipt', 1, 25, expect.any(AbortSignal));
  expect(view.get('[aria-current="true"]').text()).toBe('Chapter 26');
  await view.get('button[aria-label="imports.sectionPreview.previous"]').trigger('click'); await flushPromises();
  expect(summary).toHaveBeenLastCalledWith('receipt', 1, 0, expect.any(AbortSignal));
  expect(view.get('[aria-current="true"]').text()).toBe('Chapter 25');
});

it('does not replace a newer section with a late response', async () => {
  let resolve!: (value: api.TXTPreviewSection) => void;
  const old = new Promise<api.TXTPreviewSection>(done => { resolve = done; });
  const section = vi.spyOn(api, 'getTXTPreviewSection').mockReturnValueOnce(old).mockResolvedValue({ generation: 1, index: 1, title: 'Chapter 2', text: 'Selected chapter' });
  view = mount(TXTPreview, options); await flushPromises();
  await view.findAll('.preview-entries button')[1]!.trigger('click'); await flushPromises();
  expect(section.mock.calls[0]![3].aborted).toBe(true);
  resolve({ generation: 1, index: 0, title: 'Chapter 1', text: 'Old chapter' }); await flushPromises();
  expect(view.get('pre').text()).toBe('Selected chapter');
});
