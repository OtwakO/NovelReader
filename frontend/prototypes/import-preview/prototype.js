// Disposable presentation mock. No fetches, storage, actual imports or progress writes.
const variants = [
  { key: 'A', name: 'Compact toolbar', description: 'A single framed preview: contents selector and Previous/Next above the text. The reading area has one continuous boundary.' },
  { key: 'B', name: 'Visible contents', description: 'Chapter navigation stays visible beside the text. On mobile the bounded chapter list sits above it; both areas scroll separately.' },
  { key: 'C', name: 'Reading first', description: 'A quiet reading surface with a Contents toggle. The chapter list opens in place, then closes when you select a chapter.' },
];
const params = new URLSearchParams(location.search);
let variant = Math.max(0, variants.findIndex(value => value.key === params.get('variant')));
let chapter = 0, contentsOpen = false, resume = '';
const width = document.querySelector('#width');
const format = document.querySelector('#format');
const workflow = document.querySelector('#workflow');
const app = document.querySelector('#app');
const cover = '../../src/assets/covers/default-book-cover.webp';
const titles = ['The harbour before dawn', 'A letter with no return address', 'Beyond the old lighthouse', 'A road through the winter hills', 'At the edge of the northern sea', 'The long way home'];
const paragraphs = [
  'The harbour was quiet when Mara arrived. A single light burned above the workshop door, and the tide had left a silver line across the steps. She set her bag down and listened to the water moving beneath the boards.',
  'Inside, the map lay open on the table. Someone had marked a route in blue pencil, following the coast before turning inland. There were no instructions, only a small circle around a village she had never visited.',
  '“You can leave in the morning,” the keeper said. He moved the lamp closer so she could see the faded names. “The road is longer than it looks, but you will not have to walk it alone.”',
  'Mara folded the letter once more. Outside the window, the first boats were beginning to move. She had expected an answer at the end of her journey; instead, she had found a place to begin.',
];
function chapters() { return format.value === 'EPUB' ? ['Cover', ...titles] : titles; }
function button(text, action, disabled = false, primary = false) {
  return `<button type="button" class="app-button app-button--${primary ? 'primary' : 'secondary'}" data-action="${action}" ${disabled ? 'disabled' : ''}>${text}</button>`;
}
function pager() {
  return `<div class="pager">${button('Previous', 'previous', chapter === 0)}<span>${chapter + 1} / ${chapters().length}</span>${button('Next', 'next', chapter === chapters().length - 1)}</div>`;
}
function chapterList() {
  return `<ol class="chapter-list">${chapters().map((title, index) => `<li><button type="button" data-chapter="${index}" aria-current="${chapter === index}">${title}<small>${format.value === 'EPUB' && index === 0 ? 'Front matter' : `Chapter ${format.value === 'EPUB' ? index : index + 1}`}</small></button></li>`).join('')}</ol>`;
}
function reading() {
  const isCover = format.value === 'EPUB' && chapter === 0;
  const body = isCover
    ? `<img src="${cover}" alt="Existing placeholder book artwork"><p class="caption">Synthetic publication · opening cover</p>`
    : `<h4>${chapters()[chapter]}</h4>${[...paragraphs, ...paragraphs].map((text, index) => `<p>${text}</p>${format.value === 'EPUB' && index === 1 ? `<img src="${cover}" alt="Placeholder illustration" style="max-height:180px"><p class="caption">Centered illustration sample</p>` : ''}`).join('')}`;
  return `<div class="reading" tabindex="0" aria-label="Selected chapter preview"><article>${body}</article></div>`;
}
function render() {
  const v = variants[variant], reparse = workflow.value === 'reparse';
  params.set('variant', v.key); history.replaceState(null, '', '?' + params);
  document.querySelector('#variant').textContent = `${v.key} · ${v.name}`;
  document.querySelector('#description').textContent = v.description;
  document.querySelector('#state').textContent = `${format.value} · ${reparse ? 're-analysis' : 'import'} · section ${chapter + 1}/${chapters().length} · ${width.value}px requested (capped to window) · resume: ${resume || 'unchanged'}`;
  document.querySelector('#stage').style.width = width.value + 'px';
  const selector = `<label>Contents<select id="chapter">${chapters().map((title, index) => `<option value="${index}" ${chapter === index ? 'selected' : ''}>${title}</option>`).join('')}</select></label>`;
  let preview;
  if (v.key === 'A') preview = `<div class="preview-frame"><div class="toolbar">${selector}${pager()}</div>${reading()}</div>`;
  if (v.key === 'B') preview = `<div class="preview-frame split"><aside class="contents-rail"><h4>Contents · ${chapters().length} sections</h4>${chapterList()}</aside><div class="split-main">${pager()}${reading()}</div></div>`;
  if (v.key === 'C') preview = `<div class="preview-frame"><div class="toolbar"><strong>${chapters()[chapter]}</strong><button class="app-button app-button--secondary" data-action="contents" aria-expanded="${contentsOpen}">Contents</button>${pager()}</div>${contentsOpen ? `<nav class="drawer" aria-label="Chapters">${chapterList()}</nav>` : ''}${reading()}</div>`;
  app.innerHTML = `<header class="app-shell"><strong>NovelReader</strong><span>Local import</span></header>
    <div class="workspace"><div class="page-heading"><h1>${reparse ? 'Re-analyze TXT' : 'Local import'}</h1><span class="muted">${reparse ? 'Back to book details' : 'Back to shelf'}</span></div>
    <details class="context"><summary>${reparse ? 'Current reading settings · UTF-8 · English chapter headings' : 'From this device · 1 file prepared for review'}</summary><p>Simplified surrounding context. Upload and preparation controls are not part of this mock.</p></details>
    <section class="review ${v.key}"><header class="review-heading"><div><h2>The Lantern Keeper</h2><p>${reparse ? 'Prepared interpretation · current book stays unchanged' : 'Check the contents before adding this book.'}</p></div><span class="format-tag">${format.value}</span></header>
    <div class="review-tools">${format.value === 'TXT' ? '<details><summary>Adjust chapters</summary><p>Encoding and chapter detection stay in the existing workflow; no mock preparation is performed.</p></details>' : ''}<details><summary>${reparse ? 'Interpretation details' : 'Book details'}</summary><p>Title: The Lantern Keeper<br>Author: Synthetic example</p></details></div>
    <section class="preview"><div class="preview-heading"><h3>Book preview</h3><span>${chapters().length} sections</span></div>${preview}<p class="preview-note">Preview only — ${reparse ? 'browsing does not choose your resume location or apply changes.' : 'no reading progress is saved.'}</p></section>
    ${reparse ? `<section class="resume"><h3>Resume after applying</h3><p>${resume ? `Chosen: ${resume}` : 'Keep the saved reading location.'}</p>${button('Use previewed chapter', 'resume')}</section>` : ''}
    <footer class="review-footer"><div class="actions">${button(reparse ? 'Review and apply changes' : 'Add book', 'mock', false, true)}${button(reparse ? 'Discard preview' : 'Discard import', 'mock')}</div><p>Prototype actions do not modify data.</p></footer><p id="feedback" role="status"></p></section></div>`;
  const chapterSelect = document.querySelector('#chapter');
  if (chapterSelect) chapterSelect.onchange = event => openChapter(Number(event.target.value));
}
function openChapter(index) { chapter = index; contentsOpen = false; render(); }
function cycle(step) { variant = (variant + step + variants.length) % variants.length; contentsOpen = false; render(); }
app.addEventListener('click', event => {
  const target = event.target.closest('button');
  if (!target) return;
  if (target.dataset.chapter !== undefined) return openChapter(Number(target.dataset.chapter));
  switch (target.dataset.action) {
    case 'previous': return openChapter(chapter - 1);
    case 'next': return openChapter(chapter + 1);
    case 'contents': contentsOpen = !contentsOpen; return render();
    case 'resume': resume = chapters()[chapter]; return render();
    case 'mock': document.querySelector('#feedback').textContent = 'Prototype only — no book or saved reading state was changed.';
  }
});
width.onchange = render;
format.onchange = () => { chapter = 0; resume = ''; render(); };
workflow.onchange = () => {
  if (workflow.value === 'reparse') format.value = 'TXT';
  format.disabled = workflow.value === 'reparse';
  chapter = 0; resume = ''; render();
};
document.querySelector('#previous-variant').onclick = () => cycle(-1);
document.querySelector('#next-variant').onclick = () => cycle(1);
document.addEventListener('keydown', event => {
  if (event.target.closest('input,textarea,select,button,[contenteditable]')) return;
  if (event.key === 'ArrowLeft' || event.key === 'ArrowRight') { event.preventDefault(); cycle(event.key === 'ArrowLeft' ? -1 : 1); }
});
render();
