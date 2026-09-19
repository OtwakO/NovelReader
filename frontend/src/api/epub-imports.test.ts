import { afterEach, expect, it, vi } from 'vitest';
import { acquireEPUB, acceptEPUB } from './epub-imports';
import { requestImportAdmission } from './file-imports';
import { resetReaderRequests } from './transport';

afterEach(() => { resetReaderRequests(); vi.unstubAllGlobals(); });

it('uses shared admission, an unchanged file body and explicit EPUB policy/generation under reader cancellation', async () => {
  const fetch = vi.fn().mockImplementation(async () => new Response('{}'));
  vi.stubGlobal('fetch', fetch);
  const signal = new AbortController().signal;
  await requestImportAdmission(signal);
  const file = new File(['synthetic bytes'], '书名 & notes.epub');
  await acquireEPUB('ticket', file, 'optimized', signal);
  await acceptEPUB('ticket', 4, 'Title', 'Author', signal);
  expect(fetch.mock.calls[0]![0]).toBe('/api/imports/admission');
  const [url, options] = fetch.mock.calls[1]!;
  expect(new URL(url, 'https://reader.test').searchParams.get('filename')).toBe(file.name);
  expect(new URL(url, 'https://reader.test').searchParams.get('imageMode')).toBe('optimized');
  expect(options.body).toBe(file); expect(options.headers['Content-Type']).toBe('application/octet-stream');
  expect(JSON.parse(fetch.mock.calls[2]![1].body)).toEqual({ generation: 4, name: 'Title', author: 'Author' });
  resetReaderRequests(); expect(options.signal.aborted).toBe(true);
});
