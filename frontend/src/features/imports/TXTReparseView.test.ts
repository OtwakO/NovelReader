import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { mount, flushPromises, type VueWrapper } from '@vue/test-utils';
import { createMemoryHistory, createRouter } from 'vue-router';
import * as api from '../../api/txt-imports';
import { ApiError } from '../../api/transport';
import TXTReparseView from './TXTReparseView.vue';

let view: VueWrapper;
let status: api.TXTReparseStatus;
let impact: api.TXTReparseImpact;
const preview: api.TXTPreview = { analysisVersion: 2, encoding: 'utf-8', preset: 'generated-sections', parserVersion: 1, reviewReasons: ['no-headings'], totalSections: 1, headings: [{index:0,title:'Merged section',generated:true}], hasMore:false, sample:'<script>literal prose</script>', sampleTruncated:false };
function button(key:string) { return view.findAll('button').find(item=>item.text()===`imports.reparse.${key}`)!; }
async function refresh() { await view.get('.reparse-recovery button').trigger('click'); await flushPromises(); }
beforeEach(() => {
 status={name:'Sample novel',author:'Sample author',activeGeneration:1,contentRevision:1,stateVersion:4,activeOptions:{encoding:'',preset:''},candidate:{generation:2,state:'needs_review',options:{encoding:'',preset:'generated-sections'},baseContentRevision:1,hasError:false}};
 impact={generation:2,activeGeneration:1,contentRevision:1,stateVersion:4,totalSections:1,resume:null,preservedBookmarks:1,unresolvedBookmarks:1};
 vi.spyOn(api,'getTXTReparse').mockImplementation(async()=>structuredClone(status));
 vi.spyOn(api,'previewTXTReparse').mockResolvedValue(preview);
 vi.spyOn(api,'getTXTReparseSection').mockImplementation(async (_id, generation, index) => ({ generation, index, title: 'Merged section', text: preview.sample }));
 vi.spyOn(api,'impactTXTReparse').mockImplementation(async()=>structuredClone(impact));
});
afterEach(()=>{view?.unmount();vi.restoreAllMocks();});
async function open() {
 const router=createRouter({history:createMemoryHistory(),routes:[{path:'/books/:bookId/txt/reparse',component:TXTReparseView},{path:'/books/:bookId',name:'book-detail',component:{template:'<div />'}},{path:'/books/:bookId/read/:chapterIndex?',name:'reader',component:{template:'<div />'}}]});
 await router.push('/books/sample/txt/reparse');await router.isReady();
 view=mount(TXTReparseView,{global:{plugins:[router],mocks:{$t:(key:string)=>key}}});await flushPromises();
}

it('requires a chosen resume and explicit confirmation, then refreshes stale impact without losing drafts',async()=>{
 const apply=vi.spyOn(api,'applyTXTReparse').mockRejectedValueOnce(new ApiError(409,{code:'state_changed'})).mockImplementation(async()=>{
  status={...status,activeGeneration:2,contentRevision:2,stateVersion:6,candidate:undefined};
  return {libraryId:'sample',contentRevision:2,stateVersion:6,alreadyApplied:false};
 });
 await open();
 expect(view.get('pre').text()).toBe('<script>literal prose</script>');expect(view.find('script').exists()).toBe(false);
 expect(button('apply').attributes('disabled')).toBeDefined();
 await button('beginning').trigger('click');await button('apply').trigger('click');
 expect(apply).not.toHaveBeenCalled();
 await button('confirmApply').trigger('click');await flushPromises();
 expect(view.text()).toContain('imports.reparse.stateChanged');expect(button('apply').attributes('disabled')).toBeDefined();
 // Status recovery must not overwrite an unsent encoding draft.
 await view.findAll('select')[0]!.setValue('big5');impact.stateVersion=5;await refresh();
 expect((view.findAll('select')[0]!.element as HTMLSelectElement).value).toBe('big5');expect(button('apply').attributes('disabled')).toBeDefined();
 await button('savedOptions').trigger('click');await button('apply').trigger('click');await button('confirmApply').trigger('click');await flushPromises();
 expect(apply).toHaveBeenLastCalledWith('sample',{generation:2,activeGeneration:1,contentRevision:1,stateVersion:5,resumeChapter:0},expect.any(AbortSignal));
 expect(view.text()).toContain('imports.reparse.applied');expect(view.find('pre').exists()).toBe(false);
});

it('recovers an uncertain Apply from active generation without resending it',async()=>{
 const apply=vi.spyOn(api,'applyTXTReparse').mockImplementation(async()=>{
  status={...status,activeGeneration:2,contentRevision:2,stateVersion:5,candidate:undefined};
  throw new Error('response lost');
 });
 await open();await button('beginning').trigger('click');await button('apply').trigger('click');await button('confirmApply').trigger('click');await flushPromises();
 expect(view.text()).toContain('imports.reparse.refreshRequired');expect(view.text()).not.toContain('imports.reparse.applied');
 await refresh();expect(view.text()).toContain('imports.reparse.applied');expect(apply).toHaveBeenCalledOnce();
  expect(view.find('.reparse-recovery').exists()).toBe(false);
});

it('prepares with exact candidate guards and discards only preparation, never the original',async()=>{
 const prepare=vi.spyOn(api,'prepareTXTReparse').mockImplementation(async()=>{status.candidate={...status.candidate!,generation:3,state:'queued'};return {generation:3};});
 const discard=vi.spyOn(api,'discardTXTReparse').mockImplementation(async()=>{status.candidate=undefined;});
 const pendingDiscard=vi.spyOn(api,'discardTXT');
 await open();await button('replace').trigger('click');await flushPromises();
 expect(prepare).toHaveBeenCalledWith('sample',1,2,{encoding:'',preset:'generated-sections',pattern:''},expect.any(AbortSignal));
 expect(view.find('pre').exists()).toBe(false);
 await button('discard').trigger('click');expect(discard).not.toHaveBeenCalled();
 await button('confirmDiscard').trigger('click');await flushPromises();
 expect(discard).toHaveBeenCalledWith('sample',1,3,expect.any(AbortSignal));expect(pendingDiscard).not.toHaveBeenCalled();
 expect(view.text()).toContain('imports.reparse.readCurrent');
});

it('uses shared failure guidance for a failed reparse without suggesting encoding for storage failures', async () => {
  status.candidate = { ...status.candidate!, state: 'analysis_failed', hasError: true, errorCode: 'txt_storage_error' };
  await open();
  expect(view.text()).toContain('imports.analysisErrors.storage');
  expect(view.text()).not.toContain('imports.analysisHint');
  expect(view.find('pre').exists()).toBe(false);
});

it('keeps preview browsing separate from the explicit resume action', async () => {
  await open();
  expect(view.find('.reparse-recovery').exists()).toBe(false);
  expect(view.text()).not.toContain('imports.refresh');
  const links = view.findAll('.feature-heading a');
  expect(links).toHaveLength(2);
  expect(links[1]!.text()).toBe('imports.reparse.backToDetails');
  expect(links[1]!.classes()).toContain('app-button--secondary');
  expect(links[1]!.attributes('href')).toBe('/books/sample');
  expect(view.findAll('.reparse-current dt').map(label => label.text())).toEqual(['imports.encoding', 'imports.preset']);
  const read = links[0]!;
  expect(read.classes()).toContain('app-button--secondary');
  expect(read.attributes('href')).toBe('/books/sample/read');
  expect(view.text()).toContain('Sample author');
  await view.get('.preview-entries button').trigger('click'); await flushPromises();
  expect(button('apply').attributes('disabled')).toBeDefined();
  const resume = view.get('.preview-resume button');
  expect(resume.attributes('aria-pressed')).toBe('false');
  await resume.trigger('click');
  expect(resume.attributes('aria-pressed')).toBe('true');
  expect(resume.get('.icon-check').attributes('aria-hidden')).toBe('true');
  expect(button('apply').attributes('disabled')).toBeUndefined();
});

it('offers inline status recovery after initial loading fails', async () => {
  vi.mocked(api.getTXTReparse).mockRejectedValueOnce(new Error('offline'));
  await open();
  expect(view.get('.reparse-recovery button').text()).toBe('imports.reparse.updateStatus');
  expect(view.get('[role="alert"]').text()).toBe('imports.reparse.requestFailed');
  expect(view.findAll('.feature-heading a')).toHaveLength(1);
  await refresh();
  expect(view.find('.reparse-recovery').exists()).toBe(false);
  expect(view.find('.reparse-current').exists()).toBe(true);
});
