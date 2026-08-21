const books = [
  { id: 'fanren', title: '凡人修仙传', author: '忘语', chapter: '第二章 青牛镇', progress: 12, cover: '', shelved: true },
  { id: 'guimi', title: '诡秘之主', author: '爱潜水的乌贼', chapter: '第三十二章 灰雾之上', progress: 4, cover: 'mystery', shelved: true },
  { id: 'dafeng', title: '大奉打更人', author: '卖报小郎君', chapter: '第六章 铜锣', progress: 28, cover: 'city', shelved: true },
  { id: 'qing', title: '庆余年', author: '猫腻', chapter: '第一章 京都来信', progress: 43, cover: 'pale', shelved: true },
  { id: 'daogui', title: '道诡异仙', author: '狐尾的笔', chapter: '第十九章 坐忘道', progress: 18, cover: 'red', shelved: true },
];

const searchResults = [
  { id: 'fanren', title: '凡人修仙传', author: '忘语', sources: 12, latest: '第二千四百四十六章 飞升仙界', description: '一个普通山村穷小子，偶然之下跨入江湖小门派，从此走上修仙之路。', cover: '' },
  { id: 'xianjie', title: '凡人修仙之仙界篇', author: '忘语', sources: 8, latest: '第一千三百九十四章 轮回', description: '韩立飞升仙界后的全新旅程，旧人、新敌与更广阔的天地。', cover: 'city' },
  { id: 'motian', title: '魔天记', author: '忘语', sources: 5, latest: '第七卷 原始轮回', description: '一名亡命少年在凶险世界中挣扎求存，追寻力量与身世真相。', cover: 'mystery' },
  { id: 'xuanjie', title: '玄界之门', author: '忘语', sources: 4, latest: '第一千章 尾声', description: '少年石牧踏过异域星海，寻找通向武道巅峰的道路。', cover: 'red' },
];

const sources = [
  { name: '有度中文（优+）', latency: '820 ms', status: '已验证正文', latest: '第二千四百四十六章', recommended: true },
  { name: '趣悦小说（优++）', latency: '1.2 s', status: '目录可用', latest: '第二千四百四十六章' },
  { name: '快眼看书（优+）', latency: '1.6 s', status: '已验证正文', latest: '第二千四百四十五章' },
  { name: '天悦小说（优）', latency: '2.1 s', status: '目录可用', latest: '第二千四百四十章' },
];

const chapters = [
  {
    title: '第一章 山边小村',
    text: [
      '二愣子睁大着双眼，直直望着茅草和烂泥糊成的黑屋顶，身上盖着的旧棉被早已看不出原来的颜色。',
      '在他身边，二哥睡得很熟，不时传来轻轻的鼾声。屋外天还没有全亮，远处的山影压在晨雾里，像一堵沉默的墙。',
      '他本名韩立，是青牛镇附近一个普通山村里的孩子。家里人口多，日子过得紧巴巴，但父母总盼着孩子中能有人走出这片山。'
    ]
  },
  {
    title: '第二章 青牛镇',
    text: [
      '青牛镇不算大，却是附近十几个村落最热闹的地方。每逢集日，长街两旁挤满了卖山货、布匹和农具的小摊。',
      '韩立跟着三叔穿过人群，第一次见到这么多陌生面孔。他不敢四处张望，只把包袱抱得更紧，默默记住来时的路。',
      '七玄门招收弟子的消息早已传遍附近村镇。对许多人来说，这不仅是一条学本事的路，也可能让一家人从此少受些穷困。',
      '马车离开青牛镇时，韩立回头看了一眼。远处低矮的屋舍渐渐被尘土遮住，他心里既害怕，又有一点说不清的期待。'
    ]
  },
  {
    title: '第三章 七玄门',
    text: [
      '山路越走越陡，车上的孩子们渐渐安静下来。直到日头偏西，一座依山而建的巨大院落才出现在众人眼前。',
      '门前石阶宽阔，两侧立着持械弟子。韩立抬头望着匾额上的三个大字，第一次感觉自己离原来的生活已经很远。',
      '接下来的测试并不轻松。有人因根骨出众被当场留下，也有人哭着被送回山下。韩立站在队伍里，手心全是汗。'
    ]
  }
];

const state = {
  route: 'home',
  previousRoute: 'home',
  selectedBook: 'fanren',
  searchQuery: '凡人修仙传',
  searching: false,
  checkedSources: 84,
  foundResults: 3,
  searchMode: 'search',
  searchTimer: null,
  exploreSource: 'qidian',
  exploreCategory: '',
  explorePage: 1,
  currentChapter: 1,
  readerChrome: true,
  readerNight: false,
  readerSize: 18,
  readerLine: 1.95,
  readerPara: 1,
  sourcePanel: false,
  settingsPanel: false,
  tocPanel: false,
  rescanDialog: false,
  scanning: false,
  scanCount: 84,
  foundSources: [...sources],
  currentSource: '有度轻说（优+）',
  currentSourceFailed: false,
  simulatedFetchFailureShown: false,
  sourceMessage: '',
};

const app = document.querySelector('#app');
const toastRegion = document.querySelector('#toast-region');

function icon(symbol) {
  const paths = {
    back: '<path d="M15 18l-6-6 6-6"/><path d="M9 12h10"/>',
    search: '<circle cx="11" cy="11" r="6.5"/><path d="m16 16 4 4"/>',
    settings: '<circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.7 1.7 0 0 0 .3 1.9l.1.1-2.8 2.8-.1-.1a1.7 1.7 0 0 0-1.9-.3 1.7 1.7 0 0 0-1 1.6v.2h-4V21a1.7 1.7 0 0 0-1-1.6 1.7 1.7 0 0 0-1.9.3l-.1.1L4.2 17l.1-.1a1.7 1.7 0 0 0 .3-1.9A1.7 1.7 0 0 0 3 14H2.8v-4H3a1.7 1.7 0 0 0 1.6-1 1.7 1.7 0 0 0-.3-1.9L4.2 7 7 4.2l.1.1A1.7 1.7 0 0 0 9 4.6 1.7 1.7 0 0 0 10 3v-.2h4V3a1.7 1.7 0 0 0 1 1.6 1.7 1.7 0 0 0 1.9-.3l.1-.1L19.8 7l-.1.1a1.7 1.7 0 0 0-.3 1.9 1.7 1.7 0 0 0 1.6 1h.2v4H21a1.7 1.7 0 0 0-1.6 1Z"/>',
    shelf: '<path d="M4 5h4v14H4zM10 5h4v14h-4zM16 5h4v14h-4z"/><path d="M3 20h18"/>',
    discover: '<circle cx="12" cy="12" r="9"/><path d="m15.5 8.5-2.2 4.8-4.8 2.2 2.2-4.8 4.8-2.2Z"/>',
    bookmark: '<path d="M7 4h10v16l-5-3-5 3V4Z"/>',
    more: '<circle cx="5" cy="12" r="1" fill="currentColor" stroke="none"/><circle cx="12" cy="12" r="1" fill="currentColor" stroke="none"/><circle cx="19" cy="12" r="1" fill="currentColor" stroke="none"/>',
    close: '<path d="m7 7 10 10M17 7 7 17"/>',
    source: '<path d="M7 7h11l-3-3M17 17H6l3 3"/><path d="m18 7-3 3M6 17l3-3"/>',
    toc: '<path d="M9 6h11M9 12h11M9 18h11"/><circle cx="5" cy="6" r="1" fill="currentColor" stroke="none"/><circle cx="5" cy="12" r="1" fill="currentColor" stroke="none"/><circle cx="5" cy="18" r="1" fill="currentColor" stroke="none"/>'
  };
  return `<svg class="ui-icon" viewBox="0 0 24 24" aria-hidden="true" focusable="false">${paths[symbol] || paths.more}</svg>`;
}

function bookCover(book, extra = '') {
  return `<div class="cover ${book.cover || ''} ${extra}" aria-hidden="true"><span class="cover-title">${book.title}</span></div>`;
}

function nav(active) {
  return `<nav class="bottom-nav" aria-label="主要导航"><div class="bottom-nav-inner">
    <button class="nav-button ${active === 'home' ? 'active' : ''}" data-route="home"><span class="nav-icon">${icon('shelf')}</span>书架</button>
    <button class="nav-button ${active === 'discover' ? 'active' : ''}" data-route="discover"><span class="nav-icon">${icon('discover')}</span>发现</button>
    <button class="nav-button ${active === 'search' ? 'active' : ''}" data-route="search" data-focus-search="true"><span class="nav-icon">${icon('search')}</span>搜索</button>
  </div></nav>`;
}

function topbar(title, back = false) {
  return `<header class="topbar">
    <div style="display:flex;align-items:center;gap:8px">${back ? `<button class="back-button" data-action="back" aria-label="返回">${icon('back')}</button>` : ''}<h1>${title}</h1></div>
    <div class="toolbar-actions"><button class="icon-button" data-route="search" data-focus-search="true" aria-label="搜索">${icon('search')}</button><button class="icon-button" data-action="prototype-info" aria-label="原型说明">${icon('settings')}</button></div>
  </header>`;
}

function homeView() {
  const current = books[0];
  return `<main class="page">
    ${topbar('NovelReader')}
    <div class="home-layout">
      <section class="section home-continue">
        <div class="section-heading"><h2>继续阅读</h2></div>
        <article class="continue-card card">
          ${bookCover(current)}
          <div class="continue-body"><div><h2>${current.title}</h2><div class="subtle">${current.author} · ${current.chapter}</div></div>
            <div class="progress" aria-label="阅读进度 ${current.progress}%"><span style="width:${current.progress}%"></span></div>
            <div class="subtle">已读 ${current.progress}% · 10 分钟前</div>
            <div class="continue-actions"><button class="btn primary full" data-action="continue-reading">继续阅读</button><button class="btn ghost" data-action="open-detail">书籍详情</button></div>
          </div>
        </article>
      </section>
      <section class="section home-library">
        <div class="section-heading"><div><h2>我的书架</h2><div class="subtle">最近阅读与收藏的书籍</div></div><button class="btn ghost small" data-action="manage-shelf">管理书架</button></div>
        <div class="book-grid">${books.slice(1).map(book => `<button class="book-tile" data-book="${book.id}">${bookCover(book)}<h3>${book.title}</h3><div class="subtle">${book.author}</div><div class="subtle">${book.chapter}</div></button>`).join('')}</div>
      </section>
    </div>
    ${nav('home')}
  </main>`;
}

function discoverView() {
  const exploreSources = [
    { id: 'qidian', name: '起点中文', group: '正版与综合', catalog: ['男生畅销榜', '女生畅销榜', '仙侠', '玄幻', '都市', '历史', '科幻', '轻小说'] },
    { id: 'fanqie', name: '番茄小说（优+++）', group: '综合', catalog: ['推荐', '男频', '女频', '完本', '新书', '短故事'] },
    { id: 'youdu', name: '有度中文（优+）', group: '常用书源', catalog: ['热门小说', '仙侠奇缘', '东方玄幻', '都市生活'] }
  ];
  const selected = exploreSources.find(source => source.id === state.exploreSource) || exploreSources[0];
  const pageBooks = state.exploreCategory ? searchResults.slice(0, state.explorePage === 1 ? 3 : 4) : [];
  return `<main class="page">
    ${topbar('发现')}
    <div class="explore-source-bar card">
      <div><span class="subtle">当前发现书源</span><h2>${selected.name}</h2><div class="subtle">${selected.group} · 分类与分页由此书源提供</div></div>
      <label class="source-select-label"><span>切换书源</span><select id="explore-source-select" class="source-select" aria-label="选择发现书源">${exploreSources.map(source => `<option value="${source.id}" ${source.id === selected.id ? 'selected' : ''}>${source.name}</option>`).join('')}</select></label>
    </div>
    ${state.exploreCategory ? `<section class="section">
      <div class="explore-breadcrumb"><button class="btn ghost small" data-action="explore-home">${icon('back')} 返回 ${selected.name}</button><div><h2>${state.exploreCategory}</h2><div class="subtle">${selected.name} · 第 ${state.explorePage} 页</div></div></div>
      <div class="result-list">${pageBooks.map(result => `<article class="result-card card">${bookCover(result)}<div class="result-copy"><h3>${result.title}</h3><div class="subtle">${result.author}</div><p>${result.description}</p><div class="subtle" style="margin-bottom:9px">${result.latest}</div><div class="result-actions"><button class="btn small secondary" data-action="result-detail" data-result="${result.id}">查看详情</button><button class="btn small primary" data-action="quick-shelve" data-result="${result.id}">加入书架</button></div></div></article>`).join('')}</div>
      <div class="explore-pagination"><span class="subtle">当前页面来自 ${selected.name}，不与其他书源合并</span><button class="btn secondary" data-action="explore-next-page">${state.explorePage === 1 ? '加载下一页' : '已加载全部示例'}</button></div>
    </section>` : `<section class="section">
      <div class="section-heading"><div><h2>${selected.name} 的分类</h2><div class="subtle">保持书源原生标题、顺序和导航结构</div></div><button class="btn ghost small" data-action="refresh-explore">刷新分类</button></div>
      <div class="native-catalog">${selected.catalog.map((name, index) => `<button class="native-entry" data-action="open-explore-category" data-category="${name}"><span class="entry-index">${String(index + 1).padStart(2, '0')}</span><span><strong>${name}</strong><small>${index < 2 ? '书源榜单' : '浏览分类'}</small></span><span class="entry-arrow">${icon('back')}</span></button>`).join('')}</div>
      <div class="explore-note"><strong>每次只浏览一个书源</strong><span>切换书源会开启该来源自己的发现目录；分类、控件、分页和错误不会跨来源混合。</span></div>
    </section>`}
    ${nav('discover')}
  </main>`;
}

function searchView() {
  const visibleResults = searchResults.slice(0, state.foundResults);
  const percent = Math.round(state.checkedSources / 286 * 100);
  return `<main class="page">
    ${topbar('搜索')}
    <div class="discover-layout">
      <div class="search-panel">
        <form class="search-form" id="search-form"><input id="search-input" class="search-input" value="${state.searchQuery}" aria-label="搜索书名或作者" placeholder="输入书名或作者"/><button class="btn primary" type="submit">搜索</button></form>
        <button class="btn ghost full" style="margin-top:9px" data-action="search-settings">搜索设置 · 全部 286 个书源</button>
        <section class="search-state card">
          <div class="search-state-head"><div><strong>${state.searching ? '搜索进行中…' : '搜索已暂停'}</strong><div class="subtle">已检查 ${state.checkedSources} / 286 个书源 · 找到 ${state.foundResults} 个结果</div></div><button class="btn small ${state.searching ? 'danger' : 'secondary'}" data-action="toggle-search">${state.searching ? '停止搜索' : '继续搜索'}</button></div>
          <div class="progress"><span style="width:${percent}%"></span></div><div class="subtle">${state.searching ? '正在响应：有度中文、趣悦小说、快眼看书…' : '可以浏览已返回的结果，稍后继续。'}</div>
        </section>
      </div>
      <section class="section" style="margin-top:0">
        <div class="section-heading"><h2>搜索结果</h2><span class="badge">按书名与作者合并</span></div>
        <div class="result-list">${visibleResults.map(result => `<article class="result-card card">${bookCover(result)}<div class="result-copy"><div style="display:flex;justify-content:space-between;gap:8px"><div><h3>${result.title}</h3><div class="subtle">${result.author}</div></div><span class="badge ochre">${result.sources} 个书源</span></div><p>${result.description}</p><div class="subtle" style="margin-bottom:9px">最新：${result.latest}</div><div class="result-actions"><button class="btn small secondary" data-action="result-detail" data-result="${result.id}">查看详情</button><button class="btn small primary" data-action="quick-shelve" data-result="${result.id}">${books.find(b => b.id === result.id)?.shelved ? '已在书架' : '加入书架'}</button></div></div></article>`).join('')}</div>
        <div class="failure-note" style="margin-top:12px">17 个书源请求失败，搜索仍可继续。 <button class="btn small ghost" data-action="failure-detail">查看详情</button></div>
        <button class="btn full secondary" style="margin-top:12px" data-action="continue-scan">继续扫描剩余 ${Math.max(0, 286 - state.checkedSources)} 个书源</button>
      </section>
    </div>
    ${nav('search')}
  </main>`;
}

function detailView() {
  const book = books.find(b => b.id === state.selectedBook) || books[0];
  const isFanren = book.id === 'fanren';
  return `<main class="page detail-page">
    ${topbar('书籍详情', true)}
    <div class="detail-layout">
      <div>
        <section class="detail-hero">${bookCover(book)}<div class="detail-hero-copy"><span class="badge ${book.shelved ? '' : 'ochre'}">${book.shelved ? '已加入书架' : '尚未加入书架'}</span><h1 class="book-title">${book.title}</h1><div class="subtle detail-author">${book.author}</div><div class="metadata"><span class="badge">连载中</span><span class="badge">仙侠</span><span class="badge">幻想修仙</span></div><div class="subtle detail-updated">约 770 万字 · 3 小时前更新</div></div></section>
        <div class="sticky-actions"><button class="btn secondary" data-action="add-shelf" ${book.shelved ? 'disabled' : ''}>${book.shelved ? '已在书架' : '加入书架'}</button><button class="btn primary" data-action="start-reading">开始阅读</button></div>
        <section class="section"><div class="section-heading"><h2>当前书源</h2><span class="status ${state.currentSourceFailed ? 'bad' : 'good'}">${state.currentSourceFailed ? '需要恢复' : '正文已验证'}</span></div><div class="source-summary card"><div><strong>${state.currentSource}</strong><div class="subtle">${state.currentSourceFailed ? '目录为空 · 12 个已找到书源' : '820 ms · 12 个已找到书源 · 刚刚验证'}</div></div><button class="btn small secondary" data-action="open-sources">查看书源</button></div></section>
      </div>
      <div>
        <section class="section synopsis card" style="margin-top:0"><div class="section-heading"><h2>作品简介</h2></div><p>一个普通山村穷小子，偶然之下，跨入到一个江湖小门派，成了一名记名弟子。\n\n他以平庸的资质进入修仙者的行列，在弱肉强食的世界中谨慎前行，凭借自己的努力与机缘走向漫漫仙途。</p></section>
        <section class="section"><div class="section-heading"><h2>目录预览</h2><button class="btn ghost small" data-action="open-toc">查看全部目录</button></div><div class="chapter-list">${chapters.map((chapter, index) => `<button class="chapter-row" style="width:100%;border-left:0;border-right:0;background:transparent;text-align:left" data-chapter="${index}"><span>${chapter.title}</span><span class="subtle">${index === state.currentChapter ? '当前' : ''}</span></button>`).join('')}<div class="chapter-row"><span>第二千四百四十六章 飞升仙界</span><span class="badge ochre">最新</span></div></div></section>
      </div>
    </div>
  </main>`;
}

function readerView() {
  const chapter = chapters[state.currentChapter];
  const progress = 10 + state.currentChapter * 2;
  return `<main class="reader-page"><div class="reader-surface ${state.readerNight ? 'night' : ''}" style="--reader-size:${state.readerSize}px;--reader-line:${state.readerLine};--reader-para:${state.readerPara}em">
    <div class="reader-chrome reader-top ${state.readerChrome ? '' : 'hidden'}"><div class="reader-top-row"><button class="back-button" data-action="reader-back" aria-label="返回">${icon('back')}</button><div class="reader-title"><strong>凡人修仙传</strong><span>${chapter.title}</span></div><div><button class="icon-button" data-action="bookmark" aria-label="书签">${icon('bookmark')}</button><button class="icon-button" data-action="reader-settings" aria-label="更多">${icon('more')}</button></div></div></div>
    <article class="reader-text"><h1>${chapter.title}</h1>${chapter.text.map(p => `<p>${p}</p>`).join('')}<div class="reader-end">— 本章完 —</div></article>
    <div class="reader-tap-zone" aria-label="阅读触控区域"><button data-action="previous-chapter" aria-label="上一页"></button><button data-action="toggle-chrome" aria-label="显示或隐藏阅读控制"></button><button data-action="next-chapter" aria-label="下一页"></button></div>
    ${state.currentSourceFailed ? `<div class="reader-warning"><span>下一章加载失败 · 当前来源目录为空</span><button class="btn small secondary" data-action="open-sources">恢复书源</button></div>` : ''}
    <div class="reader-chrome reader-bottom ${state.readerChrome ? '' : 'hidden'}"><div class="reader-bottom-inner"><div class="reader-progress"><span>${progress}%</span><input type="range" min="0" max="100" value="${progress}" aria-label="全书进度" /></div><div class="reader-controls"><button data-action="previous-chapter"><span class="reader-control-icon previous-icon">${icon('back')}</span><span>上一章</span></button><button data-action="open-toc"><span class="reader-control-icon">${icon('toc')}</span><span>目录</span></button><button data-action="open-sources"><span class="reader-control-icon">${icon('source')}</span><span>书源</span></button><button data-action="reader-settings"><span class="reader-control-icon reader-aa">Aa</span><span>设置</span></button><button data-action="next-chapter"><span class="reader-control-icon next-icon">${icon('back')}</span><span>下一章</span></button></div></div></div>
  </div></main>`;
}

function sourceSheet() {
  const scanPercent = Math.round(state.scanCount / 286 * 100);
  return `<div class="overlay" data-overlay="source"><section class="sheet" role="dialog" aria-modal="true" aria-labelledby="source-title"><div class="sheet-handle"></div><div class="sheet-header"><div><h2 id="source-title">书源恢复</h2><div class="subtle">凡人修仙传 · 切换会保留当前章节与阅读进度</div></div><button class="icon-button" data-action="close-sources" aria-label="关闭">${icon('close')}</button></div>
    <div class="current-source ${state.currentSourceFailed ? '' : 'good'}"><div style="display:flex;justify-content:space-between;gap:12px"><div><span class="status ${state.currentSourceFailed ? 'bad' : 'good'}">${state.currentSourceFailed ? '当前请求失败' : '当前来源'}</span><h3 style="margin-top:5px">${state.currentSource}</h3><div class="subtle">${state.currentSourceFailed ? '目录为空 · 刚刚尝试' : '当前请求可用 · 无后台健康检查'}</div></div>${state.currentSourceFailed ? '<button class="btn small secondary" data-action="retry-source">重新尝试</button>' : ''}</div></div>
    ${state.sourceMessage ? `<div class="failure-note" style="margin-top:12px">${state.sourceMessage}</div>` : ''}
    <div class="section-heading" style="margin-top:18px"><h3>已找到的可用来源</h3><span class="badge">${state.foundSources.length} 个</span></div>
    <div class="sort-row"><button class="sort-chip active">已验证正文</button><button class="sort-chip">响应速度</button><button class="sort-chip">最新章节</button></div>
    <div class="source-list">${state.foundSources.map(source => `<article class="source-row card"><div><div style="display:flex;align-items:center;gap:7px"><h3>${source.name}</h3>${source.recommended ? '<span class="badge ochre">推荐</span>' : ''}</div><div class="source-meta"><span>${source.latency}</span><span class="status good">${source.status}</span><span>最新 ${source.latest}</span></div></div><button class="btn small ${source.recommended ? 'primary' : 'secondary'}" data-switch-source="${source.name}">${source.name === state.currentSource ? '当前' : '切换'}</button></article>`).join('')}</div>
    <div class="scan-box"><div style="display:flex;justify-content:space-between;gap:10px"><strong>${state.scanning ? '正在扫描更多书源…' : '更多书源'}</strong><button class="btn small ${state.scanning ? 'danger' : 'secondary'}" data-action="toggle-source-scan">${state.scanning ? '停止扫描' : '继续扫描'}</button></div><div class="subtle">已扫描 ${state.scanCount} / 286 个书源 · 找到 ${state.foundSources.length} 个可用来源</div><div class="progress"><span style="width:${scanPercent}%"></span></div></div>
    <button class="btn full ghost" style="margin-top:14px" data-action="confirm-rescan">清除已找到来源并重新扫描</button>
  </section></div>`;
}

function settingsSheet() {
  return `<div class="overlay" data-overlay="settings"><section class="sheet" role="dialog" aria-modal="true" aria-labelledby="settings-title"><div class="sheet-handle"></div><div class="sheet-header"><div><h2 id="settings-title">阅读设置</h2><div class="subtle">设置只影响此原型会话</div></div><button class="icon-button" data-action="close-settings" aria-label="关闭">${icon('close')}</button></div><div class="settings-grid">
    <label class="setting-row"><span>字号</span><input data-setting="size" type="range" min="15" max="26" value="${state.readerSize}"/><output>${state.readerSize}px</output></label>
    <label class="setting-row"><span>行距</span><input data-setting="line" type="range" min="16" max="25" value="${Math.round(state.readerLine * 10)}"/><output>${state.readerLine.toFixed(1)}</output></label>
    <label class="setting-row"><span>段距</span><input data-setting="para" type="range" min="5" max="20" value="${Math.round(state.readerPara * 10)}"/><output>${state.readerPara.toFixed(1)}</output></label>
    <div class="setting-row"><span>阅读主题</span><button class="btn secondary" data-action="toggle-night">${state.readerNight ? '切换日间' : '切换夜间'}</button><span></span></div>
  </div></section></div>`;
}

function tocSheet() {
  return `<div class="overlay" data-overlay="toc"><section class="sheet" role="dialog" aria-modal="true" aria-labelledby="toc-title"><div class="sheet-handle"></div><div class="sheet-header"><div><h2 id="toc-title">目录</h2><div class="subtle">共 2,446 章 · 当前 ${chapters[state.currentChapter].title}</div></div><button class="icon-button" data-action="close-toc" aria-label="关闭">${icon('close')}</button></div><input class="search-input" style="margin-top:14px" placeholder="搜索章节"/> <div class="chapter-list" style="margin-top:14px">${chapters.map((chapter, index) => `<button class="chapter-row" style="width:100%;border-left:0;border-right:0;background:transparent;text-align:left" data-chapter="${index}"><span>${chapter.title}</span>${index === state.currentChapter ? '<span class="badge">当前</span>' : ''}</button>`).join('')}</div></section></div>`;
}

function rescanDialog() {
  return `<div class="overlay"><section class="dialog" role="alertdialog" aria-modal="true" aria-labelledby="rescan-title"><h2 id="rescan-title">清除并重新扫描？</h2><p class="subtle">这会清除已找到的替代来源，并从第一个书源重新开始。当前活动来源、书籍、目录、阅读进度和书签都会保留。</p><div class="dialog-actions"><button class="btn ghost" data-action="cancel-rescan">取消</button><button class="btn danger" data-action="do-rescan">清除并重扫</button></div></section></div>`;
}

function render() {
  const views = { home: homeView, discover: discoverView, search: searchView, detail: detailView, reader: readerView };
  app.innerHTML = (views[state.route] || homeView)() + (state.sourcePanel ? sourceSheet() : '') + (state.settingsPanel ? settingsSheet() : '') + (state.tocPanel ? tocSheet() : '') + (state.rescanDialog ? rescanDialog() : '');
}

function routeTo(route, options = {}) {
  state.previousRoute = state.route;
  state.route = route;
  render();
  window.scrollTo({ top: 0, behavior: 'smooth' });
  if (options.focusSearch) requestAnimationFrame(() => document.querySelector('#search-input')?.focus());
}

function toast(message) {
  const el = document.createElement('div');
  el.className = 'toast';
  el.textContent = message;
  toastRegion.append(el);
  setTimeout(() => el.remove(), 2800);
}

function startSearch() {
  clearInterval(state.searchTimer);
  state.searching = true;
  if (state.checkedSources >= 286) { state.checkedSources = 0; state.foundResults = 1; }
  state.searchTimer = setInterval(() => {
    if (!state.searching) return;
    state.checkedSources = Math.min(286, state.checkedSources + 13);
    state.foundResults = Math.min(searchResults.length, 1 + Math.floor(state.checkedSources / 55));
    if (state.checkedSources >= 286) { state.searching = false; clearInterval(state.searchTimer); toast('搜索完成，共合并出 4 个结果'); }
    render();
  }, 700);
  render();
}

function startSourceScan(reset = false) {
  if (reset) { state.scanCount = 0; state.foundSources = []; }
  state.scanning = true;
  const candidates = [...sources];
  const timer = setInterval(() => {
    if (!state.scanning) { clearInterval(timer); return; }
    state.scanCount = Math.min(286, state.scanCount + 22);
    const next = candidates.find(candidate => !state.foundSources.some(source => source.name === candidate.name));
    if (next) state.foundSources.push(next);
    if (state.scanCount >= 286) { state.scanning = false; clearInterval(timer); toast('书源扫描完成'); }
    render();
  }, 900);
  render();
}

function selectChapter(index) {
  state.currentChapter = Math.max(0, Math.min(chapters.length - 1, index));
  books[0].chapter = chapters[state.currentChapter].title;
  books[0].progress = 10 + state.currentChapter * 2;
  state.tocPanel = false;
  routeTo('reader');
  toast(`已定位到${chapters[state.currentChapter].title}`);
}

app.addEventListener('submit', event => {
  if (event.target.id !== 'search-form') return;
  event.preventDefault();
  state.searchQuery = document.querySelector('#search-input').value.trim() || '凡人修仙传';
  state.checkedSources = 0;
  state.foundResults = 1;
  startSearch();
});

app.addEventListener('input', event => {
  if (event.target.id === 'explore-source-select') {
    state.exploreSource = event.target.value;
    state.exploreCategory = '';
    state.explorePage = 1;
    render();
    toast('已打开所选书源的原生发现目录');
    return;
  }
  const setting = event.target.dataset.setting;
  if (!setting) return;
  if (setting === 'size') state.readerSize = Number(event.target.value);
  if (setting === 'line') state.readerLine = Number(event.target.value) / 10;
  if (setting === 'para') state.readerPara = Number(event.target.value) / 10;
  render();
});

app.addEventListener('click', event => {
  const routeButton = event.target.closest('[data-route]');
  if (routeButton) { routeTo(routeButton.dataset.route, { focusSearch: routeButton.dataset.focusSearch }); return; }
  const mode = event.target.closest('[data-mode]');
  if (mode) { state.searchMode = mode.dataset.mode; render(); return; }
  const bookTile = event.target.closest('[data-book]');
  if (bookTile) { state.selectedBook = bookTile.dataset.book; routeTo('detail'); return; }
  const sourceSwitch = event.target.closest('[data-switch-source]');
  if (sourceSwitch) {
    const source = sourceSwitch.dataset.switchSource;
    state.sourceMessage = `正在切换到 ${source}…`;
    render();
    setTimeout(() => { state.currentSource = source; state.currentSourceFailed = false; state.sourceMessage = `切换成功，已定位到${chapters[state.currentChapter].title}`; render(); toast(`已切换到 ${source}`); }, 900);
    return;
  }
  const chapterButton = event.target.closest('[data-chapter]');
  if (chapterButton) { selectChapter(Number(chapterButton.dataset.chapter)); return; }
  const resultButton = event.target.closest('[data-result]');
  const action = event.target.closest('[data-action]')?.dataset.action;
  if (!action) return;

  if (action === 'back') routeTo(state.previousRoute === 'reader' ? 'home' : state.previousRoute);
  if (action === 'prototype-info') toast('交互原型：所有状态只保存在当前浏览器会话中');
  if (action === 'manage-shelf') toast('书架管理将在下一轮原型中细化');
  if (action === 'open-detail') { state.selectedBook = 'fanren'; routeTo('detail'); }
  if (action === 'continue-reading') routeTo('reader');
  if (action === 'toggle-search') { state.searching ? (state.searching = false, render(), toast('已停止搜索')) : startSearch(); }
  if (action === 'continue-scan') startSearch();
  if (action === 'search-settings') toast('高级设置：书源组“全部”，并发 24；原型暂不展开');
  if (action === 'failure-detail') toast('示例失败：超时 9、解析失败 5、需要验证 3');
  if (action === 'open-explore-category') { state.exploreCategory = event.target.closest('[data-category]').dataset.category; state.explorePage = 1; render(); window.scrollTo({ top: 0, behavior: 'smooth' }); }
  if (action === 'explore-home') { state.exploreCategory = ''; state.explorePage = 1; render(); }
  if (action === 'explore-next-page') { if (state.explorePage === 1) { state.explorePage = 2; render(); toast('已按该书源的分页状态加载下一页'); } else toast('示例页面已全部加载'); }
  if (action === 'refresh-explore') toast('已重新打开当前书源的发现目录');
  if (action === 'result-detail') { state.selectedBook = resultButton.dataset.result === 'fanren' ? 'fanren' : 'fanren'; state.previousRoute = 'discover'; routeTo('detail'); }
  if (action === 'quick-shelve') toast('已加入书架；搜索结果保持在当前页面');
  if (action === 'add-shelf') { const book = books.find(b => b.id === state.selectedBook); if (book) book.shelved = true; render(); toast('已保存书籍信息和目录到书架'); }
  if (action === 'start-reading') { const book = books.find(b => b.id === state.selectedBook); if (book) book.shelved = true; toast('正文验证通过，已加入书架'); setTimeout(() => routeTo('reader'), 350); }
  if (action === 'open-sources') { state.sourcePanel = true; state.sourceMessage = ''; render(); }
  if (action === 'close-sources') { state.sourcePanel = false; render(); }
  if (action === 'retry-source') { state.sourceMessage = '重新尝试失败：目录为空。请选择其他来源。'; render(); }
  if (action === 'toggle-source-scan') state.scanning ? (state.scanning = false, render(), toast('已停止书源扫描')) : startSourceScan();
  if (action === 'confirm-rescan') { state.rescanDialog = true; render(); }
  if (action === 'cancel-rescan') { state.rescanDialog = false; render(); }
  if (action === 'do-rescan') { state.rescanDialog = false; state.sourceMessage = ''; startSourceScan(true); toast('已清除替代来源，正从第一个书源重新扫描'); }
  if (action === 'reader-back') routeTo('detail');
  if (action === 'toggle-chrome') { state.readerChrome = !state.readerChrome; render(); }
  if (action === 'bookmark') toast('已添加本章书签');
  if (action === 'reader-settings') { state.settingsPanel = true; render(); }
  if (action === 'close-settings') { state.settingsPanel = false; render(); }
  if (action === 'toggle-night') { state.readerNight = !state.readerNight; render(); }
  if (action === 'open-toc') { state.tocPanel = true; render(); }
  if (action === 'close-toc') { state.tocPanel = false; render(); }
  if (action === 'previous-chapter') { if (state.currentChapter > 0) selectChapter(state.currentChapter - 1); else toast('已经是第一章'); }
  if (action === 'next-chapter') {
    if (state.currentSourceFailed) { state.sourcePanel = true; render(); }
    else if (!state.simulatedFetchFailureShown && state.currentChapter === 1) {
      state.currentSourceFailed = true;
      state.simulatedFetchFailureShown = true;
      state.sourceMessage = '加载下一章时发现当前来源目录为空。已保留本章内容和阅读位置。';
      state.sourcePanel = true;
      render();
    }
    else if (state.currentChapter < chapters.length - 1) selectChapter(state.currentChapter + 1);
    else toast('示例章节已读完');
  }
});

app.addEventListener('click', event => {
  if (!event.target.classList.contains('overlay')) return;
  const overlay = event.target.dataset.overlay;
  if (overlay === 'source') state.sourcePanel = false;
  if (overlay === 'settings') state.settingsPanel = false;
  if (overlay === 'toc') state.tocPanel = false;
  if (overlay) render();
});

window.addEventListener('keydown', event => {
  if (state.route !== 'reader' || state.sourcePanel || state.settingsPanel || state.tocPanel) return;
  if (event.key === 'ArrowRight' || event.key === ' ') {
    event.preventDefault();
    if (state.currentSourceFailed) { state.sourcePanel = true; render(); }
    else if (!state.simulatedFetchFailureShown && state.currentChapter === 1) {
      state.currentSourceFailed = true;
      state.simulatedFetchFailureShown = true;
      state.sourceMessage = '加载下一章时发现当前来源目录为空。已保留本章内容和阅读位置。';
      state.sourcePanel = true;
      render();
    }
    else if (state.currentChapter < chapters.length - 1) selectChapter(state.currentChapter + 1);
  }
  if (event.key === 'ArrowLeft' && state.currentChapter > 0) selectChapter(state.currentChapter - 1);
  if (event.key.toLowerCase() === 'c') { state.tocPanel = true; render(); }
});

render();
