import { flushPromises, mount, type VueWrapper } from '@vue/test-utils';
import { afterEach, expect, it, vi } from 'vitest';
import * as api from '../../api/epub-review';
import { parseStructuredProseDocument } from '../../api/structured-prose';
import EPUBSectionPreview from './EPUBSectionPreview.vue';

let view: VueWrapper;
afterEach(() => { view?.unmount(); vi.restoreAllMocks(); });
const prose = (text: string) => parseStructuredProseDocument({ kind: 'prose', title: '', blocks: [{ kind: 'paragraph', children: [{ kind: 'text', text }] }] });
const options = { props: { receiptId: 'receipt', generation: 1, totalSections: 2 }, global: { mocks: { $t: (key: string) => key } } };

it('renders a cover and authored contents independently of empty section titles, then selects prose', async () => {
  vi.spyOn(api, 'getEPUBPreviewNavigation').mockResolvedValue({ source: 'publication', entries: [{ label: 'Authored chapter', target: { section: 1, anchor: 'anchor' }, unavailable: false, children: [] }] });
  const cover = parseStructuredProseDocument({ kind: 'prose', title: '', blocks: [{ kind: 'image', resource: { href: '/api/imports/epub/receipts/receipt/resources/image?generation=1', mediaType: 'image/png' }, width: 3, height: 2 }] });
  const section = vi.spyOn(api, 'getEPUBPreviewSection').mockResolvedValueOnce(cover).mockResolvedValue(prose('<script>Literal prose</script>'));
  view = mount(EPUBSectionPreview, options); await flushPromises();
  expect(view.get('img').attributes('src')).toContain('generation=1');
  expect(view.get('select').text()).toContain('Authored chapter');
  await view.get('select').setValue('0'); await flushPromises();
  expect(section).toHaveBeenLastCalledWith('receipt', 1, 1, expect.any(AbortSignal));
  expect(view.get('[role="article"]').text()).toBe('<script>Literal prose</script>');
  expect(view.find('script').exists()).toBe(false);
});

it('uses section navigation for omitted front matter and never displays a late response after receipt replacement', async () => {
  vi.spyOn(api, 'getEPUBPreviewNavigation').mockResolvedValue({ source: 'sections', entries: [{ label: '', target: { section: 0 }, unavailable: false, children: [] }] });
  let resolve!: (value: ReturnType<typeof prose>) => void;
  const old = new Promise<ReturnType<typeof prose>>(done => { resolve = done; });
  const section = vi.spyOn(api, 'getEPUBPreviewSection').mockReturnValueOnce(old).mockResolvedValue(prose('Current receipt'));
  view = mount(EPUBSectionPreview, options); await flushPromises();
  expect(view.get('select').text()).toContain('imports.epub.sectionNumber');
  await view.setProps({ receiptId: 'replacement' }); await flushPromises();
  resolve(prose('Old receipt')); await flushPromises();
  expect(view.text()).not.toContain('Old receipt');
  expect(view.text()).toContain('Current receipt');
  const next = view.findAll('button').find(button => button.text() === 'imports.epub.nextSection')!;
  await next.trigger('click'); await flushPromises();
  expect(section).toHaveBeenLastCalledWith('replacement', 1, 1, expect.any(AbortSignal));
});
