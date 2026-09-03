import { beforeEach, describe, expect, it, vi } from 'vitest';
import { getSource, resetSourceInteraction, runSourceInteractionAction, startSourceBrowser, updateSource, updateSourcePreferences } from './sources';

const request = vi.fn();
vi.mock('./transport', () => ({ request: (...args: unknown[]) => request(...args) }));

describe('source transport', () => {
  beforeEach(() => request.mockReset());
  it('loads one complete definition only when requested', async () => {
    request.mockResolvedValue({});
    await getSource('source/id');
    expect(request).toHaveBeenCalledWith('/sources/source%2Fid');
  });
  it('patches source preferences without replacing the definition', async () => {
    request.mockResolvedValue({});
    await updateSourcePreferences('source/id', { enabled: false });
    expect(request).toHaveBeenCalledWith('/sources/source%2Fid', { method: 'PATCH', body: JSON.stringify({ enabled: false }) });
  });
  it('updates a definition through its immutable Source ID', async () => {
    const source={bookSourceUrl:'https://source.example',bookSourceName:'Source',enabled:true,enabledExplore:false,unknown:'kept'};
    request.mockResolvedValue(source); await updateSource('source-id',source);
    expect(request).toHaveBeenCalledWith('/sources?id=source-id',{method:'PUT',body:JSON.stringify(source)});
  });
  it('starts browser sessions at the client surface dimensions', async () => {
    request.mockResolvedValue({});
    await startSourceBrowser('source/id', 'request-1', 1180, 760, 2);
    expect(request).toHaveBeenCalledWith('/sources/source%2Fid/interaction/browser', { method: 'POST', body: JSON.stringify({ browserRequestId: 'request-1', width: 1180, height: 760, deviceScaleFactor: 2 }) });
  });
  it('executes and resets interaction state by immutable Source ID', async () => {
    request.mockResolvedValue({});
    await runSourceInteractionAction('source/id', 'revision', 'action-2', { User: 'reader' });
    expect(request).toHaveBeenCalledWith('/sources/source%2Fid/interaction/actions', { method: 'POST', body: JSON.stringify({ revision: 'revision', actionId: 'action-2', values: { User: 'reader' }, isLongClick: false }) });
    await resetSourceInteraction('source/id', 'login');
    expect(request).toHaveBeenLastCalledWith('/sources/source%2Fid/interaction/login', { method: 'DELETE' });
  });
});
