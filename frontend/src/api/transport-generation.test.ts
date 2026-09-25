import { afterEach, expect, it, vi } from 'vitest';
import { readerHomeGeneration, request, requestControl, resetReaderRequests } from './transport';

const response = (generation: string, status = 200) => new Response('{}', {
  status, headers: { 'X-Reader-Generation': generation },
});
afterEach(() => { resetReaderRequests(); vi.unstubAllGlobals(); });

it('pins the reported home generation and qualifies subsequent reader requests, not controls', async () => {
  const fetchMock = vi.fn().mockImplementation(() => Promise.resolve(response('home-a')));
  vi.stubGlobal('fetch', fetchMock);
  expect(readerHomeGeneration()).toBeUndefined();
  await request('/books');
  expect(readerHomeGeneration()).toBe('home-a');
  await request('/books/book/progress', { method: 'PUT', body: '{}' });
  expect(fetchMock.mock.calls[1]?.[1].headers.get('X-Reader-Generation')).toBe('home-a');
  await requestControl('/auth/me');
  expect(fetchMock.mock.calls[2]?.[1].headers.has('X-Reader-Generation')).toBe(false);
});

it('retires old parsing and future writes on replacement without replaying or adopting the new home', async () => {
  let finish!: (value: unknown) => void;
  const slow = response('home-a');
  const json = vi.spyOn(slow, 'json').mockImplementation(() => new Promise(resolve => { finish = resolve; }));
  const fetchMock = vi.fn().mockResolvedValueOnce(slow).mockResolvedValueOnce(response('home-b', 409));
  vi.stubGlobal('fetch', fetchMock);
  const old = request('/books');
  const rejected = expect(old).rejects.toMatchObject({ code: 'reader_home_changed' });
  await vi.waitFor(() => expect(json).toHaveBeenCalled());
  await expect(request('/books/book')).rejects.toMatchObject({ code: 'reader_home_changed' });
  finish({ stale: true });
  await rejected;
  await expect(request('/books/book', { method: 'DELETE' })).rejects.toMatchObject({ code: 'reader_home_changed' });
  expect(fetchMock).toHaveBeenCalledTimes(2);
  expect(readerHomeGeneration()).toBe('home-a');
  fetchMock.mockResolvedValueOnce(response('home-b'));
  await expect(requestControl('/backups/restores/operation')).resolves.toEqual({});
  expect(readerHomeGeneration()).toBe('home-a');

  resetReaderRequests();
  fetchMock.mockResolvedValueOnce(response('home-b'));
  await request('/books');
  expect(readerHomeGeneration()).toBe('home-b');
});
