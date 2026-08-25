export default {
  title: '搜尋', description: '依書名或作者搜尋已啟用書源。結果會逐步出現，同一本書會合併顯示，但不會隱藏可用書源。',
  form: { label: '書名或作者', placeholder: '搜尋書名或作者…' },
  actions: { search: '搜尋', stop: '停止', retry: '重試目前批次', restart: '重新搜尋', more: '繼續掃描 {count} 個書源' },
  controls: { title: '搜尋範圍與強度', batchSize: '每批書源數', intensity: '搜尋強度', gentle: '溫和 · 4 並行', balanced: '均衡 · 8 並行', fast: '快速 · 16 並行', advanced: '進階', concurrency: '並行數' },
  status: { checkedOf: '已檢查 {checked} / {eligible} 個可搜尋文字書源', checked: '已檢查 {checked} 個可搜尋文字書源', results: '找到 {count} 本書', concurrency: '並行 {count}', failures: '已完成批次中有 {count} 個書源失敗', disconnected: '搜尋連線已中斷。重試目前批次可保留既有結果並繼續。', stale: '搜尋期間書源清單發生變化，請重新搜尋。', storage: '無法在目前分頁儲存搜尋狀態。' },
  results: { label: '搜尋結果', summary: '合併後共 {count} 本書', multiple: '其中 {count} 本有多個書源', detailsFor: '預覽《{name}》', preview: '預覽書籍', sources: '{count} 個書源', shelve: '加入書架', retryAdd: '重試書源', retryCommit: '重新加入書架', disconnected: '進度連線已中斷，正在重新連線…', cancelShelving: '取消', cancelled: '已取消加入。', added: '已加入書架。', addFailed: '已知書源皆未能提供可閱讀的正文。' },
  empty: { title: '沒有找到書籍', description: '請嘗試其他書名或作者；若仍有書源可掃描，也可以繼續搜尋。' },
};
