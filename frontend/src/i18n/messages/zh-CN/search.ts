export default {
  title: '搜索', description: '按书名或作者搜索已启用书源。结果会逐步出现，同一本书会合并展示，但不会隐藏可用书源。',
  form: { label: '书名或作者', placeholder: '搜索书名或作者…' },
  actions: { search: '搜索', stop: '停止', retry: '重试当前批次', restart: '重新搜索', more: '继续扫描 {count} 个书源' },
  controls: { title: '搜索范围与强度', batchSize: '每批书源数', intensity: '搜索强度', gentle: '温和 · 4 并发', balanced: '均衡 · 8 并发', fast: '快速 · 16 并发', advanced: '高级', concurrency: '并发数' },
  status: { checkedOf: '已检查 {checked} / {eligible} 个书源', checked: '已检查 {checked} 个书源', results: '找到 {count} 本书', concurrency: '并发 {count}', failures: '已完成批次中有 {count} 个书源失败', disconnected: '搜索连接已中断。重试当前批次可以保留已有结果并继续。', stale: '搜索期间书源列表发生变化，请重新搜索。', storage: '无法在当前标签页保存搜索状态。' },
  results: { label: '搜索结果', summary: '合并后共 {count} 本书', multiple: '其中 {count} 本有多个书源', detailsFor: '查看《{name}》详情', sources: '{count} 个书源', details: '详情', shelve: '加入书架', shelving: '正在加入…', added: '已加入书架。', addFailed: '无法将此书加入书架。' },
  empty: { title: '没有找到书籍', description: '请尝试其他书名或作者；如果还有书源可扫描，也可以继续搜索。' },
};
