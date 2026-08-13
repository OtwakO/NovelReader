import { describe, expect, it } from 'vitest';
import { ExploreApiError } from '../../api/transport';
import { categorySelection, classifyExploreError, selectedCategoryAfterRefresh } from './explore-state';

describe('Explore state', () => {
  it('retains only selectable categories after native control refresh', () => { const entries=[{id:'a',title:'A',type:'url',selectable:true},{id:'b',title:'B',type:'url',selectable:false}]; expect(selectedCategoryAfterRefresh('a',entries)).toBe('a'); expect(selectedCategoryAfterRefresh('b',entries)).toBe(''); });
  it('uses source-session category cache without merging categories', () => { const cache={a:{results:[],nextPage:2,exhausted:false}}; expect(categorySelection('b','a',cache).kind).toBe('cached'); expect(categorySelection('a','a',cache).kind).toBe('current'); expect(categorySelection('a','c',cache).kind).toBe('load'); });
  it('classifies session expiry, page conflicts, and retryability', () => { expect(classifyExploreError(new ExploreApiError({code:'session_not_found',stage:'session',severity:'error',message:'expired'}))).toEqual({kind:'reopen'}); expect(classifyExploreError(new ExploreApiError({code:'page_conflict',stage:'page',severity:'error',message:'page',nextPage:3}))).toEqual({kind:'page',page:3}); expect(classifyExploreError(new ExploreApiError({code:'transport_failed',stage:'transport',severity:'error',message:'retry',retryable:true}))).toEqual({kind:'retry'}); });
});
