import { afterEach, expect, it, vi } from 'vitest';
import { request, requestControl, resetReaderRequests, suspendReaderRequests } from './transport';

afterEach(() => { resetReaderRequests(); vi.unstubAllGlobals(); });

it('blocks old and new reader requests during replacement but keeps outcome controls available', async () => {
  const fetchMock = vi.fn().mockResolvedValue(new Response('{}', { headers: { 'Content-Type': 'application/json' } }));
  vi.stubGlobal('fetch', fetchMock);
  resetReaderRequests();
  suspendReaderRequests();
  await expect(request('/books')).rejects.toThrow();
  expect(fetchMock).not.toHaveBeenCalled();
  await expect(requestControl('/backups/restores/op')).resolves.toEqual({});
  expect(fetchMock).toHaveBeenCalledTimes(1);
  resetReaderRequests();
  fetchMock.mockResolvedValue(new Response('{}'));
  await expect(request('/books')).resolves.toEqual({});
});

it('invalidates a late restore control response when reader identity changes', async () => {
  let resolve!: (response: Response) => void;
  vi.stubGlobal('fetch', vi.fn(() => new Promise<Response>(done => { resolve = done; })));
  const pending = requestControl('/backups/restores/op');
  const rejected = expect(pending).rejects.toThrow();
  resetReaderRequests();
  resolve(new Response('{}'));
  await rejected;
});
