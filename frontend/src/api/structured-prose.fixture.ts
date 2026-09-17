// Synthetic candidate wire document, not a real publication or enabled API.
export function structuredProseFixture() {
  return { version: 2, contentRevision: 9, document: { kind: 'prose', title: '软件正文', blocks: [
    { kind: 'group', language: 'zh-Hant', direction: 'ltr', id: 'a1', children: [
      { kind: 'heading', level: 2, children: [{ kind: 'text', text: '章节' }] },
      { kind: 'paragraph', children: [
        { kind: 'text', text: '<script>literal</script> 软件' },
        { kind: 'emphasis', children: [{ kind: 'text', text: '强调' }] },
        { kind: 'link', role: 'noteref', target: { chapterIndex: 1, contentRevision: 9, anchor: 'a1' }, children: [{ kind: 'text', text: '注释' }] },
        { kind: 'link', url: 'https://example.invalid/软件', children: [{ kind: 'text', text: '外部' }] },
        { kind: 'link', unavailable: true, children: [{ kind: 'text', text: 'Unavailable' }] },
      ] },
      { kind: 'figure', children: [
        { kind: 'image', resource: { href: '/api/books/proof/chapters/0/resources/r1?contentRevision=9', mediaType: 'image/png' }, width: 640, height: 480, alt: '地图' },
        { kind: 'figureCaption', children: [{ kind: 'text', text: '图片说明' }] },
      ] },
      { kind: 'orderedList', start: 0, children: [{ kind: 'listItem', children: [{ kind: 'text', text: 'First' }] }] },
      { kind: 'ruby', children: [{ kind: 'text', text: '字' }, { kind: 'rubyText', children: [{ kind: 'text', text: 'じ' }] }] },
      { kind: 'table', children: [{ kind: 'row', children: [{ kind: 'cell', colSpan: 2, children: [{ kind: 'text', text: 'Cell' }] }] }] },
    ] },
  ] } };
}
