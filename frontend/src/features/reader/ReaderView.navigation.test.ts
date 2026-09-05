import { flushPromises, shallowMount } from '@vue/test-utils';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import ReaderView from './ReaderView.vue';
import { getBook, mergeBookSources } from '../../api/books';
import { getChapterContent, saveProgress, switchBookSource, waitForCatalog, type ChapterContent } from '../../api/reader';
import { resetProgressWriter, waitForProgressWrites } from './progress-writer';

vi.mock('../../api/books', () => ({ getBook:vi.fn(), mergeBookSources:vi.fn(), clearBookSources:vi.fn() }));
vi.mock('../../api/reader', () => ({ getChapterContent:vi.fn(), saveProgress:vi.fn(), switchBookSource:vi.fn(), waitForCatalog:vi.fn(), listFonts:vi.fn(), getFontUrl:vi.fn() }));
vi.mock('../../api/system', () => ({ getChineseConversionCapability:vi.fn(async()=>({available:false,modes:[]})) }));
const initialBook = { id:'book', name:'Novel', author:'Author', coverUrl:'', intro:'', kind:'', sourceId:'old', sourceUrl:'old', bookUrl:'/book', origin:'Source', lastChapter:'', durChapterIndex:0, durChapterPos:0, totalChapterNum:3, stateVersion:0, alternateSources:[] };
const chapter = (title:string):ChapterContent => ({version:1, offlineCopy:false, document:{kind:'prose',title,blocks:[]}});
const source = {sourceId:'new',sourceUrl:'new',bookUrl:'/new',sourceName:'New',name:'New',author:'Author'};
let wrapper:ReturnType<typeof shallowMount<typeof ReaderView>>;

beforeEach(()=>{
  vi.clearAllMocks();resetProgressWriter();localStorage.clear();
  localStorage.setItem('novelreader.reader.preferences.v1',JSON.stringify({prefetchNextChapter:false}));
  vi.mocked(getBook).mockResolvedValue({...initialBook});
  vi.mocked(mergeBookSources).mockResolvedValue({...initialBook});
  vi.mocked(waitForCatalog).mockResolvedValue([0,1,2].map(index=>({id:String(index),bookId:'book',index,title:String(index),url:'/chapter/'+index,isVolume:false})));
  vi.mocked(getChapterContent).mockImplementation(async(_book,index)=>chapter('old '+index));
  vi.mocked(saveProgress).mockImplementation(async(_book,_source,stateVersion)=>({status:'saved',stateVersion:stateVersion+1}));
});
afterEach(async()=>{wrapper?.unmount();await waitForProgressWrites('book');vi.unstubAllGlobals();});
async function open() {
  wrapper=shallowMount(ReaderView,{global:{mocks:{$t:(key:string)=>key,$route:{params:{bookId:'book',chapterIndex:'0'},query:{}},$router:{push:vi.fn(),replace:vi.fn()}},stubs:{RouterLink:true}}});
  await flushPromises();return wrapper.vm;
}

describe('reader navigation lifecycle',()=>{
  it('renders and reuses chapters without waiting for ordered progress acknowledgements',async()=>{
    const vm=await open();
    let release!:(value:{status:string;stateVersion:number})=>void;
    vi.mocked(saveProgress).mockReturnValueOnce(new Promise(resolve=>{release=resolve;}));
    await vm.navigate(1);
    expect(vm.displayContent?.document.title).toBe('old 1');
    await vm.navigate(0);
    expect(vm.displayContent?.document.title).toBe('old 0');
    expect(getChapterContent).toHaveBeenCalledTimes(2);
    release({status:'saved',stateVersion:1});
    await waitForProgressWrites('book');
    expect(vi.mocked(saveProgress).mock.calls.map(call=>call[3])).toEqual([0,1,1,0]);
  });

  it('drains prefetch before source switching and never displays its late document',async()=>{
    const vm=await open();
    let release!:(value:ChapterContent)=>void;
    vi.mocked(getChapterContent).mockReturnValueOnce(new Promise(resolve=>{release=resolve;}));
    vm.preferences.prefetchNextChapter=true;
    await flushPromises();
    const navigation=vm.navigate(1);
    vi.mocked(switchBookSource).mockImplementation(async()=>{
      vi.mocked(getChapterContent).mockImplementation(async(_book,index)=>chapter('new '+index));
      return {book:{...initialBook,sourceId:'new',sourceUrl:'new'},mapping:'title'};
    });
    const switching=vm.selectSource(source);
    await flushPromises();
    expect(switchBookSource).not.toHaveBeenCalled();
    release(chapter('stale old 1'));
    await Promise.all([navigation,switching]);
    expect(vm.displayContent?.document.title).toBe('new 0');
    expect(vm.$router.push).not.toHaveBeenCalled();
    await vm.navigate(1);
    expect(vm.displayContent?.document.title).toBe('new 1');
  });

  it('keeps progress identity on the visible chapter while the destination is converting',async()=>{
    const vm=await open();
    let release!:(value:{chapters:typeof vm.chapters;content:ChapterContent})=>void;
    vi.spyOn(vm,'convertDisplay').mockReturnValueOnce(new Promise(resolve=>{release=resolve;}));
    const navigation=vm.navigate(1);
    await flushPromises();
    expect(vm.currentIndex).toBe(0);
    expect(vm.content?.document.title).toBe('old 0');
    release({chapters:vm.chapters,content:chapter('converted 1')});
    await navigation;
    expect(vm.currentIndex).toBe(1);
    expect(vm.displayContent?.document.title).toBe('converted 1');
  });

  it('manual refetch bypasses cached documents while keeping position and recoverable display',async()=>{
    const vm=await open();
    vm.lastPosition=.4;
    vi.mocked(getChapterContent).mockRejectedValueOnce(new Error('source unavailable'));
    await vm.refetchChapter();
    expect(vm.displayContent?.document.title).toBe('old 0');
    expect(vm.error).toBe('source unavailable');
    vi.mocked(getChapterContent).mockResolvedValueOnce(chapter('fresh 0'));
    await vm.refetchChapter();
    expect(vm.displayContent?.document.title).toBe('fresh 0');
    expect(vm.lastPosition).toBe(.4);
    expect(vm.error).toBe('');
    expect(getChapterContent).toHaveBeenCalledTimes(3);
  });
});
