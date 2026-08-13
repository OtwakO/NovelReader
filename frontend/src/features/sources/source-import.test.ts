import { describe, expect, it } from 'vitest';
import { parseSourceImport, selectedImportJSON, sourceCapabilities } from './source-import';

describe('BookSource import preview', () => {
  it('accepts arrays, wrappers, and single sources while removing duplicate URLs', () => {
    const values=parseSourceImport(JSON.stringify({sources:[{bookSourceUrl:' a ',bookSourceName:' A ',enabled:true,enabledExplore:false},{bookSourceUrl:'a',bookSourceName:'duplicate'},{bookSourceUrl:'b',bookSourceName:'B'}]}));
    expect(values.map(item=>item.key)).toEqual(['a','b']); expect(values[0]?.source.bookSourceName).toBe('A');
    expect(parseSourceImport(JSON.stringify({bookSourceUrl:'c',bookSourceName:'C'}))).toHaveLength(1);
  });
  it('serializes only explicitly selected sources', () => { const previews=parseSourceImport(JSON.stringify([{bookSourceUrl:'a',bookSourceName:'A'},{bookSourceUrl:'b',bookSourceName:'B'}])); previews[1]!.selected=true; expect(JSON.parse(selectedImportJSON(previews))).toEqual([{bookSourceUrl:'b',bookSourceName:'B'}]); });
  it('identifies user-visible advanced capabilities', () => { expect(sourceCapabilities({bookSourceUrl:'a',bookSourceName:'A',enabled:true,enabledExplore:true,searchUrl:'/s',exploreUrl:'/e',header:'{}',jsLib:'x'})).toEqual(['search','explore','headers','javascript']); });
});
