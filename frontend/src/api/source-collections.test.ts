import { beforeEach, describe, expect, it, vi } from 'vitest';

const request = vi.fn();
vi.mock('./transport', () => ({ request: (...args: unknown[]) => request(...args), requestForm: vi.fn() }));

import { updateSourceCollection } from './source-collections';

describe('source collection API', () => {
  beforeEach(() => request.mockReset());

  it('updates collection availability independently', async () => {
    request.mockResolvedValue({ id: 'collection-1', enabled: false });

    await updateSourceCollection('collection-1', { enabled: false });

    expect(request).toHaveBeenCalledWith('/source-collections/collection-1', {
      method: 'PATCH',
      body: JSON.stringify({ enabled: false }),
    });
  });
});
