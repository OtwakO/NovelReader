import { describe, expect, it } from 'vitest';
import { isSearchEligible, parseSourceImport, selectedImportJSON, sourceCapabilities } from './source-import';

describe('BookSource import preview', () => {
  it('accepts arrays, wrappers, and single sources while removing duplicate URLs', () => {
    const values=parseSourceImport(JSON.stringify({sources:[{bookSourceUrl:' a ',bookSourceName:' A ',enabled:true,enabledExplore:false},{bookSourceUrl:'a',bookSourceName:'duplicate'},{bookSourceUrl:'b',bookSourceName:'B'}]}));
    expect(values.map(item=>item.key)).toEqual(['a','b']); expect(values[0]?.source.bookSourceName).toBe('A');
    expect(parseSourceImport(JSON.stringify({bookSourceUrl:'c',bookSourceName:'C'}))).toHaveLength(1);
  });
  it('serializes only explicitly selected sources', () => { const previews=parseSourceImport(JSON.stringify([{bookSourceUrl:'a',bookSourceName:'A'},{bookSourceUrl:'b',bookSourceName:'B'}])); previews[1]!.selected=true; expect(JSON.parse(selectedImportJSON(previews))).toEqual([{bookSourceUrl:'b',bookSourceName:'B'}]); });
  it('identifies user-visible advanced capabilities', () => { expect(sourceCapabilities({bookSourceUrl:'a',bookSourceName:'A',enabled:true,enabledExplore:true,searchUrl:'/s',exploreUrl:'/e',header:'{}',jsLib:'x'})).toEqual(['search','explore','headers','javascript']); });
  it('matches backend text-search eligibility for string and structured rules', () => { const base={bookSourceUrl:'a',bookSourceName:'A',enabled:true,enabledExplore:true,bookSourceType:0,searchUrl:'/search',ruleSearch:'{}'};expect(isSearchEligible(base)).toBe(true);expect(isSearchEligible({...base,ruleSearch:{bookList:'.result'}})).toBe(true);expect(isSearchEligible({...base,enabled:false})).toBe(false);expect(isSearchEligible({...base,bookSourceType:1})).toBe(false);expect(isSearchEligible({...base,searchUrl:''})).toBe(false);expect(isSearchEligible({...base,ruleSearch:''})).toBe(false);expect(isSearchEligible({...base,ruleSearch:null})).toBe(false); });
});
