import { beforeEach, describe, expect, it, vi } from 'vitest';
import { updateSource } from './sources';

const request = vi.fn();
vi.mock('./transport', () => ({ request: (...args: unknown[]) => request(...args) }));

describe('source transport', () => {
  beforeEach(() => request.mockReset());
  it('updates a definition through its existing URL identity', async () => {
    const source={bookSourceUrl:'https://source.example',bookSourceName:'Source',enabled:true,enabledExplore:false,unknown:'kept'};
    request.mockResolvedValue(source); await updateSource(source.bookSourceUrl,source);
    expect(request).toHaveBeenCalledWith('/sources?url=https%3A%2F%2Fsource.example',{method:'PUT',body:JSON.stringify(source)});
  });
});
