import { afterEach, expect, it, vi } from 'vitest';
import { acquireTXT, acceptTXT, analyzeTXT, resolveTXTInbox } from './txt-imports';
import { resetReaderRequests } from './transport';
afterEach(() => { resetReaderRequests(); vi.unstubAllGlobals(); });

it('sends the original file body through the reader-owned transport with an encoded filename', async () => {
  const fetch = vi.fn().mockResolvedValue(new Response(JSON.stringify({ receipt: { id: 'ticket' } }), { status: 201 }));
  vi.stubGlobal('fetch', fetch);
  const file = new File(['synthetic'], '书名 & sample.txt');
  await acquireTXT('ticket', file, new AbortController().signal);
  const [url, options] = fetch.mock.calls[0]!;
  expect(new URL(url, 'https://reader.test').searchParams.get('filename')).toBe(file.name);
  expect(options.method).toBe('PUT'); expect(options.body).toBe(file);
  expect(options.headers['Content-Type']).toBe('application/octet-stream');
  const signal = options.signal as AbortSignal; resetReaderRequests(); expect(signal.aborted).toBe(true);
});

it('keeps version-qualified admission separate from opaque-token inbox resolution', async () => {
  const fetch = vi.fn().mockResolvedValueOnce(new Response(JSON.stringify({ libraryId: 'book' }))).mockResolvedValueOnce(new Response(null, { status: 204 }));
  vi.stubGlobal('fetch', fetch); const signal = new AbortController().signal;
  await acceptTXT('receipt', 7, 'Title', 'Author', signal);
  await resolveTXTInbox('opaque-token', 'release', signal);
  expect(JSON.parse(fetch.mock.calls[0]![1].body)).toEqual({ analysisVersion: 7, name: 'Title', author: 'Author' });
  expect(fetch.mock.calls[1]![0]).toBe('/api/imports/txt/inbox/reviews/opaque-token/release');
  expect(fetch.mock.calls[1]![1].body).toBeUndefined();
});

it('sends the exact custom expression with its reviewed generation', async () => {
  const fetch = vi.fn().mockResolvedValue(new Response('{}'));
  vi.stubGlobal('fetch', fetch);
  const options = { encoding: 'utf-8' as const, preset: 'custom' as const, pattern: String.raw`(?i)part\s+\d+` };
  await analyzeTXT('receipt', 7, options, new AbortController().signal);
  expect(JSON.parse(fetch.mock.calls[0]![1].body)).toEqual({ analysisVersion: 7, ...options });
});
