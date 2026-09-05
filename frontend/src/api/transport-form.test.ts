import { afterEach, describe, expect, it, vi } from 'vitest';
import { NetworkError, onAuthenticationLoss, requestForm } from './transport';

describe('multipart transport', () => {
  afterEach(() => { vi.unstubAllGlobals(); onAuthenticationLoss(); });
  it('normalizes fetch rejection at the transport seam', async () => {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new TypeError('Failed to fetch')));
    await expect(requestForm('/fonts', new FormData())).rejects.toBeInstanceOf(NetworkError);
  });
  it('posts FormData without overriding its content type', async () => {
    const fetchMock=vi.fn().mockResolvedValue(new Response(JSON.stringify({id:'font'}),{status:201,headers:{'Content-Type':'application/json'}}));vi.stubGlobal('fetch',fetchMock);const form=new FormData();form.append('name','Font');await expect(requestForm('/fonts',form)).resolves.toEqual({id:'font'});expect(fetchMock).toHaveBeenCalledWith('/api/fonts',expect.objectContaining({method:'POST',body:form}));expect(fetchMock.mock.calls[0]?.[1]?.headers).toBeUndefined();
  });
  it('uses central API errors and authentication-loss handling', async () => {
    const lost=vi.fn();onAuthenticationLoss(lost);vi.stubGlobal('fetch',vi.fn().mockResolvedValue(new Response(JSON.stringify({error:'session ended'}),{status:401,headers:{'Content-Type':'application/json'}})));await expect(requestForm('/fonts',new FormData())).rejects.toEqual(expect.objectContaining({status:401,message:'session ended'}));expect(lost).toHaveBeenCalledOnce();
  });
});
