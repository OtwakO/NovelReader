import { flushPromises, mount } from '@vue/test-utils';
import { createPinia, disposePinia } from 'pinia';
import { expect, it, vi } from 'vitest';
import * as api from '../../api/file-imports';
import ImportQueuePanel from './ImportQueuePanel.vue';
import ImportInboxPanel from './ImportInboxPanel.vue';
import { useImportQueue } from './import-queue';

it('shares the image choice across browser/inbox controls without changing queued imports', async () => {
  localStorage.clear();
  const pinia = createPinia();
  const queue = useImportQueue(pinia); queue.pause();
  vi.spyOn(api, 'scanInbox').mockResolvedValue({ directory: 'inbox', items: [{ name: 'Inbox.epub', size: 1, modifiedAt: 0 }] });
  vi.spyOn(api, 'listInboxClaims').mockResolvedValue({ items: [] });
  const global = { plugins: [pinia], mocks: { $t: (key: string) => key }, stubs: { RouterLink: true } };
  const browser = mount(ImportQueuePanel, { global });
  const inbox = mount(ImportInboxPanel, { global });
  try {
    await flushPromises();
    await inbox.get('select').setValue('epub'); await flushPromises();
    const browserChoice = browser.findAll('input[type="checkbox"]')[1]!;
    await browserChoice.setValue(true);
    expect(inbox.get<HTMLInputElement>('input[aria-describedby="inbox-image-hint"]').element.checked).toBe(true);
    const input = browser.get('input[type="file"]');
    Object.defineProperty(input.element, 'files', { value: [new File(['synthetic'], 'Browser.epub')] });
    await input.trigger('change');
    await inbox.get('input[aria-describedby="inbox-image-hint"]').setValue(false);
    expect((browserChoice.element as HTMLInputElement).checked).toBe(false);
    await inbox.get('.import-inbox-file input').setValue(true);
    await inbox.findAll('button').find(button => button.text() === 'imports.acquireSelected')!.trigger('click');
    expect(queue.entries.map(entry => entry.imageMode)).toEqual(['optimized', 'original']);
  } finally {
    browser.unmount(); inbox.unmount(); disposePinia(pinia); vi.restoreAllMocks(); localStorage.clear();
  }
});
