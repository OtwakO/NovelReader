import { afterEach, expect, it, vi } from 'vitest';
import { onAuthenticationLoss, request, resetReaderRequests } from './transport';

afterEach(() => { onAuthenticationLoss(); resetReaderRequests(); vi.unstubAllGlobals(); });

it.each([200, 401])('rejects a previous identity response during body parsing (%s)', async (status) => {
  let resolve!: (value: unknown) => void;
  const json = vi.fn(() => new Promise(done => { resolve = done; }));
  const fetchMock = vi.fn().mockResolvedValue({ ok: status === 200, status, json });
  const lost = vi.fn();
  onAuthenticationLoss(lost);
  vi.stubGlobal('fetch', fetchMock);
  const pending = request('/books');
  const rejected = expect(pending).rejects.toMatchObject({ name: 'AbortError' });
  await vi.waitFor(() => expect(json).toHaveBeenCalled());
  resetReaderRequests();
  expect(fetchMock.mock.calls[0]?.[1].signal.aborted).toBe(true);
  resolve(status === 200 ? { private: 'previous reader' } : { error: 'expired' });
  await rejected;
  expect(lost).not.toHaveBeenCalled();
  fetchMock.mockResolvedValue(new Response(JSON.stringify({ current: true })));
  await expect(request('/books')).resolves.toEqual({ current: true });
});
