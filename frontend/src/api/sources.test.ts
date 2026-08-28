import { beforeEach, describe, expect, it, vi } from 'vitest';
import { updateSource } from './sources';

const request = vi.fn();
vi.mock('./transport', () => ({ request: (...args: unknown[]) => request(...args) }));

describe('source transport', () => {
  beforeEach(() => request.mockReset());
  it('updates a definition through its immutable Source ID', async () => {
    const source={bookSourceUrl:'https://source.example',bookSourceName:'Source',enabled:true,enabledExplore:false,unknown:'kept'};
    request.mockResolvedValue(source); await updateSource('source-id',source);
    expect(request).toHaveBeenCalledWith('/sources?id=source-id',{method:'PUT',body:JSON.stringify(source)});
  });
});
