<script lang="ts">
import { defineComponent, markRaw, nextTick } from 'vue';
import { clearBookSources, getBookSource, mergeBookSources, type Book, type LibraryBook } from '../../api/books';
import type { CatalogNavigation } from '../../api/catalog-navigation';
import type { AltSource, Chapter } from '../../api/models';
import { getFontUrl, listFonts, switchBookSource, waitForCatalog, type ReadingContent, type Font } from '../../api/reader';
import { getChineseConversionCapability, type ChineseConversionCapability } from '../../api/system';
import AppButton from '../../ui/components/AppButton.vue';
import { createReaderDisplayConverter } from './chinese-conversion';
import { createChapterLoader } from './chapter-loader';
import { checkReaderLink, isReaderRevisionConflict, loadReaderSnapshot, readerLocation, ReaderCatalogError, ReaderRevisionConflict } from './reader-session';
import { adjacentChapterIndex, clampProgress, isMainChapter, normalizedScroll, resolveChapterIndex, scrollTopForProgress } from './reading-progress';
import { createReaderNavigation, followReaderTarget, returnFromReaderNote, canRecordReaderProgress, type ReaderNavigation } from './reader-navigation';
import type { ReadingTarget } from '../../api/structured-prose';
import { readerKeyboardAction } from './reader-keyboard';
import { isAtScrollBoundary, readerTapAction } from './reader-tap-navigation';
import { invalidateReadingState, queueProgressWrite, setProgressVersion, waitForProgressWrites } from './progress-writer';
import { loadReaderPreferences, saveReaderPreferences, type ReaderPreferences } from './reader-preferences';
import ProseRenderer from './ProseRenderer.vue';
import ReaderBookmarksSheet from './ReaderBookmarksSheet.vue';
import ReaderActionsMenu from './ReaderActionsMenu.vue';
import AppIcon from '../../ui/components/AppIcon.vue';
import ReaderSettingsSheet from './ReaderSettingsSheet.vue';
import ReaderSourceSheet from './ReaderSourceSheet.vue';
import ReaderTocSheet from './ReaderTocSheet.vue';
import { createReaderWakeLock } from './wake-lock';

export default defineComponent({
  name: 'ReaderView', components: { AppButton, ProseRenderer, ReaderActionsMenu, ReaderBookmarksSheet, AppIcon, ReaderSettingsSheet, ReaderSourceSheet, ReaderTocSheet },
  data() { return { catalogNavigation: undefined as CatalogNavigation | undefined, displayNavigation: undefined as CatalogNavigation | undefined, navigation: null as ReaderNavigation | null, chapterLoader: null as ReturnType<typeof createChapterLoader> | null, convertDisplay: markRaw(createReaderDisplayConverter()), navigating: false, refetching: false, book: null as LibraryBook | null, nativeBook: null as Book | null, chapters: [] as Chapter[], displayChapters: [] as Chapter[], content: null as ReadingContent | null, displayContent: null as ReadingContent | null, fonts: [] as Font[], fontsLoaded: false, fontsLoading: null as Promise<void> | null, conversionCapability: null as ChineseConversionCapability | null, currentIndex: 0, sourceUrl: '', contentRevision: -1, catalogRevision: 0, revisionConflict: false, loading: true, error: '', catalogFailed: false, catalogRetrying: false, progressError: '', conversionError: '', sourceError: '', sourceMessage: '', switching: false, chromeVisible: true, activeSheet: '' as ''|'toc'|'settings'|'bookmarks'|'sources', preferences: loadReaderPreferences(), lastPosition: 0, restoring: false, generation: 0, conversionGeneration: 0, wakeLockController: null as ReturnType<typeof createReaderWakeLock>|null, wakeLockWarning: '', progressTimer: undefined as ReturnType<typeof setTimeout>|undefined, persistence: Promise.resolve() as Promise<void>, suppressRouteLoad: false };  },
  computed: {
    bookId(): string { return String(this.$route.params.bookId || ''); },
    routeChapter(): number | undefined { const value=Number(this.$route.params.chapterIndex); return Number.isInteger(value)&&value>=0?value:undefined; },
    routePosition(): number | undefined { const value=Number(this.$route.query.position); return Number.isFinite(value)?clampProgress(value):undefined; },
    previousIndex(): number|null { return this.revisionConflict||this.navigation?.current.note?null:adjacentChapterIndex(this.chapters,this.currentIndex,-1); },
    nextIndex(): number|null { return this.revisionConflict||this.navigation?.current.note?null:adjacentChapterIndex(this.chapters,this.currentIndex,1); },
    routeAnchor(): string|undefined { return typeof this.$route.query.anchor==='string'?this.$route.query.anchor:undefined; },
    mainSections(): Chapter[] { return this.chapters.filter(isMainChapter); },
    readingNote(): boolean { return Boolean(this.navigation?.current.note||this.chapters.find(chapter=>chapter.index===this.currentIndex)?.auxiliary); },
    readerCatalog() { return { chapters: this.chapters, contentRevision: this.catalogRevision }; },
    recordsProgress(): boolean { return !this.revisionConflict && Boolean(this.navigation && canRecordReaderProgress(this.readerCatalog,this.navigation)); },
    currentTitle(): string { return this.displayContent?.document.title || this.displayChapters.find(chapter=>chapter.index===this.currentIndex)?.title || ''; },
    fontFamily(): string { return this.preferences.fontId==='system'?'var(--font-literary)':`reader-font-${this.preferences.fontId}, var(--font-literary)`; },
    readerStyle(): Record<string,string> { return { '--reader-bg':this.preferences.background,'--reader-text':this.preferences.textColor,'--reader-size':`${this.preferences.fontSize}px`,'--reader-line':String(this.preferences.lineHeight),'--reader-weight':String(this.preferences.fontWeight),'--reader-width':`${this.preferences.pageWidth}px`,'--reader-font':this.fontFamily }; },
    recoveryNeeded(): boolean { return Boolean(this.error); },
  },
  watch: {
    '$route.fullPath'() { if (this.suppressRouteLoad) { this.suppressRouteLoad=false; return; } void this.load(); },
    preferences: { deep:true, handler(value:ReaderPreferences) { saveReaderPreferences(value); } },
    'preferences.chineseConversion'() { void this.changeConversion(); },
    'preferences.prefetchNextChapter'(enabled:boolean) { if(enabled)this.prefetchNext(); },
    'preferences.keepScreenAwake'() { this.wakeLockWarning=''; void this.wakeLockController?.sync(); },
  },
  async mounted() { window.addEventListener('keydown', this.onKeydown); this.wakeLockController=createReaderWakeLock(()=>this.preferences.keepScreenAwake,()=>{this.wakeLockWarning=this.$t('reader.settings.wakeLockUnavailable')});await this.wakeLockController.sync();void this.loadConversionCapability();await this.load(); if(this.preferences.fontId!=='system')void this.loadFonts(); },
  beforeUnmount() { this.generation+=1; this.conversionGeneration+=1; void this.chapterLoader?.dispose(true); window.removeEventListener('keydown', this.onKeydown);void this.wakeLockController?.destroy();if(this.progressTimer)clearTimeout(this.progressTimer); void this.persistProgress(); },
  methods: {
    async load(retry = false) {
      if(this.progressTimer)clearTimeout(this.progressTimer);
      const request=++this.generation; this.conversionGeneration+=1;
      this.navigation=null; this.catalogNavigation=undefined; this.displayNavigation=undefined; this.restoring=false; this.loading=true; this.content=null; this.displayContent=null; this.displayChapters=[]; this.navigating=false;
      this.error=''; this.catalogFailed=false; this.revisionConflict=false; this.activeSheet='';
      if(this.book?.id!==this.bookId){this.book=null;this.nativeBook=null;this.chapters=[];}
      try {
        await this.chapterLoader?.dispose(); if(request!==this.generation)return;
        this.chapterLoader=null; this.convertDisplay=markRaw(createReaderDisplayConverter());
        await waitForProgressWrites(this.bookId);
        const {book,catalog}=await loadReaderSnapshot(this.bookId,{retry,isCurrent:()=>request===this.generation});
        if(request!==this.generation)return;
        this.book=book;
        checkReaderLink(this.$route.query.contentRevision,catalog.contentRevision);
        if(this.routeAnchor!==undefined&&this.$route.query.contentRevision===undefined)throw new ReaderRevisionConflict();
        const nativeBook=book.provider==='booksource'?await getBookSource(this.bookId):null;
        if(request!==this.generation)return;
        this.nativeBook=nativeBook; this.sourceUrl=nativeBook?.sourceUrl||'';
        this.catalogRevision=catalog.contentRevision; this.chapters=catalog.chapters; this.catalogNavigation=catalog.navigation;
        setProgressVersion(this.bookId,book.stateVersion);
        const index=resolveChapterIndex(this.chapters,this.routeChapter,book.durChapterIndex);
        if(index===null)throw new Error(this.$t('reader.errors.noReadable'));
        const position=this.routePosition ?? (index===book.durChapterIndex?book.durChapterPos:0);
        const initial=createReaderNavigation(this.readerCatalog,index,position);
        await this.loadContent(index,position,request,{...initial,current:{...initial.current,note:this.$route.query.note==='1',anchor:this.routeAnchor}});
        if(request!==this.generation)return;
        if(this.$route.query.contentRevision===undefined){
          this.suppressRouteLoad=true;
          await this.$router.replace(readerLocation(this.bookId,index,this.catalogRevision,position,initial.current));
        }
      } catch(cause){
        if(request!==this.generation)return;
        if(isReaderRevisionConflict(cause)){this.stopStaleSession();return;}
        this.catalogFailed=cause instanceof ReaderCatalogError;
        if(cause instanceof ReaderCatalogError){
          this.book=cause.book;
          // Catalog failure must not hide BookSource recovery. Metadata is safe
          // to display here, but cannot initialize a reading location/version.
          try { const native=cause.book.provider==='booksource'?await getBookSource(this.bookId):null;
            if(request!==this.generation)return; this.nativeBook=native; this.sourceUrl=native?.sourceUrl||'';
          } catch(sourceError) { if(request!==this.generation)return; this.nativeBook=null; this.sourceError=sourceError instanceof Error?sourceError.message:this.$t('reader.errors.load'); }
        }
        this.error=cause instanceof Error?cause.message:this.$t('reader.errors.load');
        this.loading=false; this.chromeVisible=true; if(this.nativeBook)this.activeSheet='sources';
      }
    },
    async retryCatalog(){if(this.catalogRetrying)return;this.catalogRetrying=true;try{await this.load(true)}finally{this.catalogRetrying=false}},
    stopStaleSession(){
      this.revisionConflict=true; this.generation++; this.conversionGeneration++;
      this.loading=false; this.navigating=false; this.activeSheet=''; this.chromeVisible=true;
      if(this.progressTimer)clearTimeout(this.progressTimer);
      invalidateReadingState(this.bookId);
      void this.chapterLoader?.dispose(true); this.chapterLoader=null;
      this.convertDisplay=markRaw(createReaderDisplayConverter());
      this.error=this.$t('reader.revisionChanged');
    },
    async reopenCurrent(){
      await waitForProgressWrites(this.bookId);
      // No ordinal/position: the coherent load chooses the current saved resume.
      const target={name:'reader',params:{bookId:this.bookId}};
      if(this.$route.params.chapterIndex===undefined && !Object.keys(this.$route.query).length)await this.load();
      else await this.$router.replace(target);
    },
    openBookmark(index:number,position:number,revision:number){
      if(revision!==this.catalogRevision){this.stopStaleSession();return;}
      void this.navigate(index,position);
    },
    async loadContent(index:number,position:number,request?:number,proposal?:ReaderNavigation) {
      request ??= this.generation;
      this.chapterLoader ??= markRaw(createChapterLoader(this.bookId,this.catalogRevision,this.stopStaleSession));
      const content=await this.chapterLoader.load(index);
      if(request!==this.generation)return;
      let mode:ReaderPreferences['chineseConversion'];
      let display;
      let conversionError:string;
      // A mode change during loading must not commit text converted for the old preference.
      do {
        mode=this.preferences.chineseConversion;
        try { display=await this.convertDisplay(this.chapters,content,mode,this.catalogNavigation);conversionError=''; }
        catch { display={chapters:this.chapters,content,navigation:this.catalogNavigation};conversionError=this.$t('reader.settings.conversionFailed'); }
      } while(request===this.generation&&mode!==this.preferences.chineseConversion);
      if(request!==this.generation)return;
      this.conversionGeneration+=1;
      const previous={displayNavigation:this.displayNavigation,navigation:this.navigation,currentIndex:this.currentIndex,content:this.content,contentRevision:this.contentRevision,lastPosition:this.lastPosition,displayChapters:this.displayChapters,displayContent:this.displayContent,conversionError:this.conversionError};
      this.restoring=true;
      // Commit document, chapter identity, and position together: progress always describes visible text.
      this.navigation=proposal??createReaderNavigation(this.readerCatalog,index,position);
      this.currentIndex=index;this.content=content;this.contentRevision=content.contentRevision;this.lastPosition=position;
      this.displayChapters=display.chapters;this.displayContent=display.content;this.displayNavigation=display.navigation;this.conversionError=conversionError;
      this.loading=false;this.error='';
      try { await this.restore(position,request,proposal?.current.anchor); }
      catch(cause) {
        if(request!==this.generation)return;
        Object.assign(this,previous);
        await this.restore(previous.lastPosition,request);
        throw cause;
      }
      if(request!==this.generation)return;
      position=this.lastPosition;
      // Displaying main content counts as reading, even at its unchanged starting position.
      if(this.book)void this.queueProgress(index,position);
      if(this.book&&this.recordsProgress){this.book.durChapterIndex=index;this.book.durChapterPos=position;}
      this.prefetchNext();
      document.title=`${this.displayContent?.document.title || content.document.title} · ${this.book?.name || 'NovelReader'}`;
    },
    async changeConversion(){if(this.navigating||this.switching||this.refetching)return;const request=this.generation;const host=this.$refs.scrollHost as HTMLElement|undefined;const position=host?normalizedScroll(host.scrollTop,host.scrollHeight,host.clientHeight):this.lastPosition;await this.refreshDisplay();await nextTick();if(request!==this.generation||!host)return;this.restoring=true;host.scrollTop=scrollTopForProgress(position,host.scrollHeight,host.clientHeight);this.lastPosition=position;requestAnimationFrame(()=>{this.restoring=false});},
    async refreshDisplay(){const request=++this.conversionGeneration;try{const display=await this.convertDisplay(this.chapters,this.content,this.preferences.chineseConversion,this.catalogNavigation);if(request!==this.conversionGeneration)return;this.displayChapters=display.chapters;this.displayContent=display.content;this.displayNavigation=display.navigation;this.conversionError='';if(display.content)document.title=`${display.content.document.title} · ${this.book?.name || 'NovelReader'}`;}catch{if(request!==this.conversionGeneration)return;this.displayChapters=this.chapters;this.displayContent=this.content;this.displayNavigation=this.catalogNavigation;this.conversionError=this.$t('reader.settings.conversionFailed');}},
    async restore(position:number,request:number,anchor?:string){
      await nextTick();
      await new Promise<void>(resolve=>requestAnimationFrame(()=>resolve()));
      if(request!==this.generation)return;
      const host=this.$refs.scrollHost as HTMLElement|undefined;if(!host)return;
      this.restoring=true;
      if(anchor!==undefined){
        const renderer=this.$refs.proseRenderer as InstanceType<typeof ProseRenderer>|undefined;
        const target=renderer?.findAnchor(anchor);
        if(!target)throw new Error(this.$t('reader.anchorUnavailable'));
        host.scrollTop+=target.getBoundingClientRect().top-host.getBoundingClientRect().top;
        target.tabIndex=-1;target.focus({preventScroll:true});
        this.lastPosition=normalizedScroll(host.scrollTop,host.scrollHeight,host.clientHeight);
      }else{host.scrollTop=scrollTopForProgress(position,host.scrollHeight,host.clientHeight);this.lastPosition=position;host.focus({preventScroll:true});}
      await new Promise<void>(resolve=>requestAnimationFrame(()=>resolve()));
      if(request===this.generation)this.restoring=false;
    },
    onScroll(){if(this.revisionConflict||this.loading||this.navigating||this.switching||this.refetching||this.restoring||!this.content)return;const host=this.$refs.scrollHost as HTMLElement;this.lastPosition=normalizedScroll(host.scrollTop,host.scrollHeight,host.clientHeight);if(this.progressTimer)clearTimeout(this.progressTimer);this.progressTimer=setTimeout(()=>void this.persistProgress(),600)},
    onKeydown(event:KeyboardEvent){const action=readerKeyboardAction(event,Boolean(this.activeSheet),window.getSelection()?.toString()||'');if(action==='none')return;event.preventDefault();if(action==='escape'){if(this.activeSheet){this.activeSheet='';return}void this.$router.push(`/books/${encodeURIComponent(this.bookId)}`);return}if(action==='previous'){void this.navigate(this.previousIndex);return}if(action==='next'){void this.navigate(this.nextIndex);return}const host=this.$refs.scrollHost as HTMLElement|undefined;if(!host)return;if(action==='top'){host.scrollTo({top:0,behavior:'smooth'});return}if(action==='bottom'){host.scrollTo({top:host.scrollHeight,behavior:'smooth'});return}host.scrollBy({top:(action==='page-up'?-1:1)*host.clientHeight*.82,behavior:'smooth'});},
    onReaderTap(event:MouseEvent){
      if(this.loading||!this.content||this.activeSheet)return;
      const host=this.$refs.scrollHost as HTMLElement|undefined;if(!host)return;
      const target=event.target as HTMLElement;if(target.closest('a,button,input,select,textarea,[role="button"]'))return;
      const sideNavigation=!window.matchMedia('(pointer: coarse)').matches;
      const action=readerTapAction(event.clientX,event.clientY,host.getBoundingClientRect(),sideNavigation);
      if(action==='none')return;
      if(action==='toggle-controls'){this.chromeVisible=!this.chromeVisible;return}
      if(action==='show-controls'){this.chromeVisible=true;return}
      if(action==='previous'){void this.navigate(this.previousIndex);return}
      if(action==='next'){void this.navigate(this.nextIndex);return}
      const direction=action==='scroll-up'?-1:1;
      if(isAtScrollBoundary(direction,host.scrollTop,host.scrollHeight,host.clientHeight)){void this.navigate(direction<0?this.previousIndex:this.nextIndex,direction<0?1:0);return}
      host.scrollBy({top:direction*host.clientHeight*.82,behavior:'smooth'});
    },
    persistProgress(){if(this.restoring||this.revisionConflict||!this.content||!this.book||this.contentRevision!==this.catalogRevision)return Promise.resolve();return this.queueProgress(this.currentIndex,this.lastPosition)},
    async queueProgress(index:number,position:number){
      if(!this.recordsProgress)return;
      const bookId=this.book?.id||this.bookId;const contentRevision=this.contentRevision;
      try{await queueProgressWrite(bookId,{contentRevision,chapterIndex:index,position});if(bookId===this.bookId&&contentRevision===this.catalogRevision)this.progressError=''}
      catch(cause){if(bookId!==this.bookId||contentRevision!==this.catalogRevision)return;if(isReaderRevisionConflict(cause))this.stopStaleSession();else this.progressError=this.$t('reader.errors.progress')}
    },
    async navigate(index:number|null,position=0,proposal?:ReaderNavigation) {
      if(this.restoring||this.revisionConflict||index===null||this.switching||this.refetching)return;
      if(this.progressTimer)clearTimeout(this.progressTimer);
      void this.persistProgress();
      this.activeSheet='';
      const request=++this.generation;
      this.conversionGeneration+=1;
      this.navigating=true;
      this.error='';
      try {
        await this.loadContent(index,position,request,proposal);
        if(request!==this.generation)return;
        if(this.navigation?.current.note)return;
        if(index!==this.routeChapter||this.lastPosition!==(this.routePosition??0)||proposal?.current.anchor!==this.routeAnchor||this.$route.query.note!==undefined) {
          this.suppressRouteLoad=true;
          await this.$router.push(readerLocation(this.bookId,index,this.catalogRevision,this.lastPosition,proposal?.current));
        }
      } catch(cause) {
        if(request!==this.generation)return;
        if(isReaderRevisionConflict(cause)){this.stopStaleSession();return;}
        this.loading=false;
        this.error=cause instanceof Error?cause.message:this.$t('reader.errors.load');
        this.chromeVisible=true;
        if(this.nativeBook)this.activeSheet='sources';
      } finally { if(request===this.generation)this.navigating=false; }
    },
    async returnFromNote() {
      if(!this.navigation||this.restoring||this.revisionConflict||this.navigating)return;
      const proposal=returnFromReaderNote(this.readerCatalog,this.navigation);
      if(proposal)await this.navigate(proposal.current.chapterIndex,proposal.current.position,proposal);
    },
    targetHref(target:ReadingTarget,note:boolean): string {
      return this.$router.resolve(readerLocation(this.bookId,target.chapterIndex,target.contentRevision,undefined,{anchor:target.anchor,note})).href;
    },
    async followTarget(target:ReadingTarget,note:boolean) {
      if(!this.navigation||this.restoring||this.revisionConflict||this.navigating)return;
      try {
        const proposal=followReaderTarget(this.readerCatalog,this.navigation,target,this.lastPosition,note);
        await this.navigate(target.chapterIndex,0,proposal);
      } catch(cause) {
        if(isReaderRevisionConflict(cause))this.stopStaleSession();
        else this.error=cause instanceof Error?cause.message:this.$t('reader.errors.load');
      }
    },
    prefetchNext() {
      if(!this.revisionConflict&&this.preferences.prefetchNextChapter&&this.content&&!this.error&&!this.loading&&!this.switching&&!this.refetching&&this.nextIndex!==null)this.chapterLoader?.prefetch(this.nextIndex);
    },
    async refetchChapter() {
      if(this.revisionConflict||!this.content||this.navigating||this.switching||this.refetching)return;
      this.refetching=true;
      if(this.progressTimer)clearTimeout(this.progressTimer);
      const request=++this.generation;
      this.conversionGeneration+=1;
      const position=this.lastPosition;
      try {
        await this.chapterLoader?.dispose();
        if(request!==this.generation)return;
        this.chapterLoader=null;
        this.convertDisplay=markRaw(createReaderDisplayConverter());
        await this.loadContent(this.currentIndex,position,request,this.navigation?{...this.navigation,current:{...this.navigation.current,position,anchor:undefined}}:undefined);
      } catch(cause) {
        if(request===this.generation){if(isReaderRevisionConflict(cause))this.stopStaleSession();else this.error=cause instanceof Error?cause.message:this.$t('reader.errors.load');}
      } finally { this.refetching=false; if(request===this.generation)this.prefetchNext(); }
    },
    async captureBookmark(){const host=this.$refs.scrollHost as HTMLElement;if(this.restoring||this.revisionConflict||this.navigating||this.switching||this.refetching||!this.content||!host||this.contentRevision!==this.catalogRevision)throw new Error(this.$t('reader.errors.position'));this.lastPosition=normalizedScroll(host.scrollTop,host.scrollHeight,host.clientHeight);if(this.progressTimer)clearTimeout(this.progressTimer);const location={contentRevision:this.contentRevision,chapterIndex:this.currentIndex,position:this.lastPosition};await this.persistProgress();return location},
    async openSettings(){this.activeSheet='settings';await Promise.all([this.loadFonts(),this.loadConversionCapability()])},
    async loadConversionCapability(){try{this.conversionCapability=await getChineseConversionCapability();if(!this.conversionCapability.available&&this.preferences.chineseConversion!=='original')this.preferences={...this.preferences,chineseConversion:'original'}}catch{/* retry when Typography opens */}},
    loadFonts(){if(this.fontsLoaded)return Promise.resolve();if(this.fontsLoading)return this.fontsLoading;this.fontsLoading=(async()=>{try{this.fonts=await listFonts();this.fontsLoaded=true;for(const font of this.fonts){const family=`reader-font-${font.id}`;const face=new FontFace(family,`url(${getFontUrl(font.id)})`);void face.load().then(loaded=>document.fonts.add(loaded)).catch(()=>undefined)}}catch{/* optional */}finally{this.fontsLoading=null}})();return this.fontsLoading},
    persistMatches(sources:AltSource[]){if(!this.book||!sources.length)return;this.persistence=this.persistence.then(async()=>{if(this.book)this.book=this.nativeBook=await mergeBookSources(this.book.id,sources)}).catch(cause=>{this.sourceError=cause instanceof Error?cause.message:this.$t('sourceRecovery.persistFailed')})},
    async clearAndRescan(){try{await this.persistence;if(!this.book)throw new Error(this.$t('reader.errors.load'));this.book=this.nativeBook=await clearBookSources(this.book.id);this.sourceMessage=this.$t('sourceRecovery.cleared');this.sourceError=''}catch(cause){this.sourceError=cause instanceof Error?cause.message:this.$t('sourceRecovery.clearFailed');throw cause}},
    async selectSource(source:AltSource) {
      if(!this.book||this.switching||this.refetching)return;
      this.switching=true;
      this.navigating=false;
      const request=++this.generation;
      this.conversionGeneration+=1;
      this.sourceError='';this.sourceMessage='';
      if(this.progressTimer)clearTimeout(this.progressTimer);
      let switched=false;
      try {
        await this.chapterLoader?.dispose();
        if(request!==this.generation)return;
        await this.persistProgress();
        await waitForProgressWrites(this.bookId);
        this.book=this.nativeBook=await mergeBookSources(this.book.id,[source]);
        const result=await switchBookSource(this.book.id,source.sourceId,source.sourceUrl,source.bookUrl);
        if(request!==this.generation)return;
        switched=true;
        this.book=result.book;this.nativeBook=result.book;this.catalogRevision=result.book.contentRevision;this.sourceUrl=result.book.sourceUrl;
        setProgressVersion(this.bookId,result.book.stateVersion);
        this.chapterLoader=null;
        this.convertDisplay=markRaw(createReaderDisplayConverter());
        this.content=null;this.displayContent=null;this.loading=true;
        const catalog=await waitForCatalog(this.bookId,{isCurrent:()=>request===this.generation});
        if(request!==this.generation)return;
        if(catalog.contentRevision!==result.book.contentRevision)throw new ReaderRevisionConflict();
        const chapters=catalog.chapters;this.catalogRevision=catalog.contentRevision;
        this.chapters=chapters;this.catalogNavigation=catalog.navigation;
        const mapped=resolveChapterIndex(chapters,result.book.durChapterIndex,result.book.durChapterIndex);
        if(mapped===null)throw new Error(this.$t('reader.errors.noReadable'));
        this.sourceMessage=result.mapping==='title'?this.$t('sourceRecovery.switchedTitle'):this.$t('sourceRecovery.switchedIndex');
        this.error='';
        await this.loadContent(mapped,result.book.durChapterPos||0,request);
        if(request!==this.generation)return;
        if(mapped!==this.routeChapter||String(this.catalogRevision)!==this.$route.query.contentRevision||(result.book.durChapterPos||0)!==(this.routePosition??0)){
          this.suppressRouteLoad=true;await this.$router.replace(readerLocation(this.bookId,mapped,this.catalogRevision,result.book.durChapterPos||0));
        }
      } catch(cause) {
        if(request!==this.generation)return;
        if(isReaderRevisionConflict(cause)){this.stopStaleSession();return;}
        const message=cause instanceof Error?cause.message:this.$t('sourceRecovery.switchFailed');
        this.sourceError=switched?this.$t('reader.errors.switchedReload',{message}):message;
        if(!this.content)this.error=this.sourceError;
        this.loading=false;
        if(!switched)this.chapterLoader=null;
      } finally { this.switching=false; if(request===this.generation)this.prefetchNext(); }
    },
  },
});
</script>

<template><section class="reader" :style="readerStyle"><header v-show="chromeVisible" class="reader-header"><RouterLink :to="`/books/${encodeURIComponent(bookId)}`" class="back">{{ $t('reader.back') }}</RouterLink><div><strong>{{ book?.name || $t('reader.title') }}</strong><span>{{ currentTitle }}</span></div><div class="app-actions reader-header-actions"><span v-if="content && !revisionConflict" class="chapter-position"><template v-if="readingNote">{{ $t('reader.note') }}</template><template v-else>{{ mainSections.findIndex(chapter=>chapter.index===currentIndex) + 1 }} / {{ mainSections.length }}</template></span><ReaderActionsMenu v-if="!revisionConflict" :refresh-disabled="!content || navigating || switching" :refreshing="refetching" @bookmarks="activeSheet='bookmarks'" @refetch="refetchChapter" /></div></header><main ref="scrollHost" class="reader-scroll" tabindex="0" :aria-label="$t('reader.readingArea')" @scroll.passive="onScroll" @click="onReaderTap"><article class="prose"><p v-if="loading && !content" class="state">{{ $t('reader.loading') }}</p><section v-else-if="error && !content" class="failure"><h1>{{ currentTitle || book?.name || $t('reader.errors.chapter') }}</h1><p role="alert">{{ error }}</p><div class="failure-actions"><AppButton v-if="revisionConflict" @click="reopenCurrent">{{ $t('reader.reopenCurrent') }}</AppButton><AppButton v-else-if="catalogFailed" :busy="catalogRetrying" @click="retryCatalog">{{ $t('reader.retryCatalog') }}</AppButton><AppButton v-else :busy="loading" @click="load()">{{ $t('app.common.retry') }}</AppButton><AppButton v-if="book?.provider==='booksource'" variant="secondary" @click="activeSheet='sources'">{{ $t('reader.recover') }}</AppButton></div></section><template v-else-if="displayContent"><aside v-if="error" class="content-failure" role="alert"><span>{{ error }}</span><AppButton v-if="revisionConflict" @click="reopenCurrent">{{ $t('reader.reopenCurrent') }}</AppButton><AppButton v-if="book?.provider==='booksource'" variant="secondary" @click="activeSheet='sources'">{{ $t('reader.recover') }}</AppButton></aside><p v-if="refetching" class="conversion-warning" role="status">{{ $t('reader.actions.refetching') }}</p><p v-if="conversionError" class="conversion-warning" role="status">{{ conversionError }}</p><p v-if="wakeLockWarning" class="conversion-warning" role="status">{{ wakeLockWarning }}</p><p v-if="displayContent.offlineCopy" class="offline" role="status">{{ $t('reader.offline') }}</p><ProseRenderer ref="proseRenderer" :target-href="targetHref" :document="displayContent.document" :fallback-image-alt="$t('reader.imageAlt',{title:displayContent.document.title})" :image-unavailable="$t('reader.imageUnavailable')" :cover-unavailable="$t('reader.coverUnavailable')" :show-images="preferences.showImages" @navigate="followTarget($event.target,$event.note)" /><p class="chapter-end">{{ $t('reader.chapterEnd') }}</p></template></article></main><p v-if="progressError" class="progress-error" role="status">{{ progressError }}</p><footer v-show="chromeVisible" class="reader-controls"><div class="reader-controls-inner"><button v-if="navigation?.returns.length" :disabled="revisionConflict || navigating || refetching || switching" @click="returnFromNote"><AppIcon name="previous" /><span>{{ $t('reader.returnFromNote') }}</span></button><button v-else :disabled="previousIndex===null" @click="navigate(previousIndex)"><AppIcon name="previous" /><span>{{ $t('reader.previous') }}</span></button><button :disabled="revisionConflict" @click="activeSheet='toc'"><AppIcon name="toc" /><span>{{ $t('reader.toc.title') }}</span></button><button v-if="book?.provider==='booksource'" :class="{attention:recoveryNeeded}" @click="activeSheet='sources'"><AppIcon name="source" /><span>{{ $t('reader.sources.short') }}</span></button><button @click="openSettings"><span class="reader-aa" aria-hidden="true">Aa</span><span>{{ $t('reader.settings.title') }}</span></button><button :disabled="nextIndex===null" @click="navigate(nextIndex)"><AppIcon name="next" /><span>{{ $t('reader.next') }}</span></button></div></footer><ReaderSettingsSheet v-if="activeSheet==='settings'" v-model="preferences" :fonts="fonts" :conversion-capability="conversionCapability" @close="activeSheet=''" /><ReaderTocSheet v-if="activeSheet==='toc'" :navigation="displayNavigation" :current-anchor="navigation?.current.anchor" :chapters="displayChapters" :current-index="currentIndex" @open-target="followTarget($event,false)" @open="navigate" @close="activeSheet=''" /><ReaderBookmarksSheet v-if="activeSheet==='bookmarks'&&book" :key="book.id" :current-revision="catalogRevision" :book-id="book.id" :capture="captureBookmark" @open="openBookmark" @stale="stopStaleSession" @close="activeSheet=''" /><ReaderSourceSheet v-if="activeSheet==='sources'&&nativeBook" :book="nativeBook" :current-source="nativeBook.origin||nativeBook.sourceUrl" :switching="switching" :action-error="sourceError||error" :action-message="sourceMessage" :on-clear-and-rescan="clearAndRescan" @matches="persistMatches" @select="selectSource" @close="activeSheet=''" /></section></template>

<style scoped>.reader{position:fixed;z-index:80;inset:0;background:var(--reader-bg);color:var(--reader-text)}.reader-header{position:absolute;z-index:20;inset:0 0 auto;display:grid;grid-template-columns:auto minmax(0,1fr) auto;gap:1rem;align-items:center;padding:max(.65rem,env(safe-area-inset-top)) max(1rem,env(safe-area-inset-right)) .65rem max(1rem,env(safe-area-inset-left));border-bottom:1px solid color-mix(in srgb,var(--reader-text) 15%,transparent);background:color-mix(in srgb,var(--reader-bg) 94%,transparent);backdrop-filter:blur(12px)}.reader-header div{min-width:0;text-align:center}.reader-header strong,.reader-header span{display:block;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.reader-header span{font-size:var(--text-caption);opacity:.7}.reader-header button,.back{min-height:2.75rem;display:inline-flex;align-items:center;padding:.5rem;color:inherit;text-decoration:none}.reader-header button{border:0;background:transparent}.back{justify-content:center;border:1px solid color-mix(in srgb,var(--reader-text) 28%,transparent);border-radius:var(--radius-md);padding:.5rem .8rem;background:color-mix(in srgb,var(--reader-bg) 88%,var(--reader-text) 4%);box-shadow:0 .2rem .5rem color-mix(in srgb,var(--reader-text) 8%,transparent);font-weight:var(--weight-strong);transition:background .18s ease-out,border-color .18s ease-out,box-shadow .18s ease-out}.back:hover,.back:focus-visible{border-color:color-mix(in srgb,var(--reader-text) 48%,transparent);background:color-mix(in srgb,var(--reader-bg) 78%,var(--reader-text) 8%);box-shadow:0 .35rem .7rem color-mix(in srgb,var(--reader-text) 12%,transparent)}.reader-header-actions{justify-content:flex-end}.reader-header-actions button{width:2.75rem;min-width:2.75rem;height:2.75rem;justify-content:center;padding:0;border-radius:var(--radius-md)}.reader-header-actions button:hover,.reader-header-actions button:focus-visible{background:color-mix(in srgb,var(--reader-text) 8%,transparent)}.chapter-position{min-width:3.6rem;padding-inline:.35rem;text-align:right;font-size:var(--text-caption);font-variant-numeric:tabular-nums;opacity:.72}.reader-scroll{position:absolute;inset:0;overflow:auto;scroll-behavior:auto;cursor:default;-webkit-tap-highlight-color:transparent}.prose{width:min(var(--reader-width),calc(100% - 2rem));min-height:100%;margin:0 auto;padding:max(clamp(1.6rem,5vw,4rem),calc(4.85rem + env(safe-area-inset-top))) 0 max(7rem,calc(5rem + env(safe-area-inset-bottom)));font:var(--reader-weight) var(--reader-size)/var(--reader-line) var(--reader-font);letter-spacing:.015em}.state,.failure{text-align:center;padding:4rem 1rem}.failure p{text-align:center;color:var(--color-danger)}.content-failure{display:flex;align-items:center;justify-content:space-between;gap:.75rem;margin:0 0 1.5rem;padding:.75rem;border:1px solid color-mix(in srgb,var(--color-danger) 40%,transparent);border-radius:var(--radius-md);background:color-mix(in srgb,var(--color-danger) 8%,var(--reader-bg));color:var(--color-danger);font:var(--weight-strong) var(--text-small) var(--font-ui)}.offline{padding:.75rem;border:1px solid var(--color-warm);border-radius:var(--radius-md);color:var(--color-warm);font-family:var(--font-ui);font-size:var(--text-small)}.chapter-end{text-align:center!important;opacity:.45;margin-top:3rem!important;font-family:var(--font-ui);font-size:var(--text-caption)}.conversion-warning{padding:.7rem .85rem;border:1px solid color-mix(in srgb,var(--reader-text) 20%,transparent);border-radius:var(--radius-md);background:color-mix(in srgb,var(--reader-bg) 84%,#b37a2d 16%);font:var(--weight-strong) var(--text-small) var(--font-ui);text-align:left!important}.progress-error{position:fixed;z-index:50;left:50%;bottom:5.4rem;translate:-50% 0;margin:0;padding:.5rem .8rem;border-radius:999px;background:var(--color-danger);color:white;font:var(--weight-strong) var(--text-caption) var(--font-ui)}.reader-controls{position:absolute;z-index:20;inset:auto 0 0;padding:.55rem max(.75rem,env(safe-area-inset-right)) max(.55rem,env(safe-area-inset-bottom)) max(.75rem,env(safe-area-inset-left));border-top:1px solid color-mix(in srgb,var(--reader-text) 14%,transparent);background:color-mix(in srgb,var(--reader-bg) 97%,transparent);box-shadow:0 -10px 30px color-mix(in srgb,var(--reader-text) 6%,transparent)}.reader-controls-inner{width:min(100%,45rem);margin:0 auto;display:grid;grid-auto-flow:column;grid-auto-columns:minmax(0,1fr);gap:.25rem}.reader-controls button{min-width:0;min-height:3.45rem;display:flex;flex-direction:column;align-items:center;justify-content:center;gap:.22rem;border:0;border-radius:.55rem;padding:.25rem .1rem;background:transparent;color:inherit;font:var(--weight-strong) var(--text-caption)/1.15 var(--font-ui);letter-spacing:.01em}.reader-controls button:hover,.reader-controls button:focus-visible{background:color-mix(in srgb,var(--color-accent) 10%,transparent)}.reader-controls button>span:last-child{max-width:100%;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.reader-controls button:disabled{opacity:.3}.reader-controls .attention{color:var(--color-danger);background:color-mix(in srgb,var(--color-danger) 9%,transparent)}.reader-aa{width:1.45rem;height:1.45rem;display:grid;place-items:center;font:var(--weight-strong) var(--text-small)/1 var(--font-literary);letter-spacing:-.04em}@media(max-width:36rem){.prose{width:min(var(--reader-width),calc(100% - 2rem));padding-top:max(4.85rem,calc(4.35rem + env(safe-area-inset-top)))}.reader-controls{padding-inline:.35rem}.reader-controls-inner{gap:0}.reader-controls button{min-height:3.3rem;font-size:var(--text-caption)}}@media(max-height:30rem) and (orientation:landscape){.reader-header{padding:.25rem 1rem}.reader-controls{padding-block:.25rem}.reader-controls button{min-height:2.8rem;gap:.05rem}.prose{padding-top:3.7rem}}</style>
