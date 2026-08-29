export default {
  title: '備份與還原',
  description: '保存完整閱讀資料的可攜副本，或從可信的 NovelReader 備份取代目前資料。',
  export: { title: '匯出閱讀資料', description: '下載目前閱讀資料的一致時間點快照。不包含帳戶憑證。', included: '包含內容', items: ['書架', '進度', '書籤', '書源', '偏好與字型'], action: '下載備份', busy: '正在準備備份…', failed: '無法下載備份。' },
  restore: { title: '還原閱讀資料', description: '先上傳並驗證。只有明確確認後才會變更資料。', choose: '選擇 .tar.gz 備份', selected: '已選檔案', prepare: '上傳並驗證', preparing: '正在驗證備份…', failed: '無法準備此備份。', ready: '資料驗證成功，準備還原', source: '匯出自 {username}', created: '建立於 {date}', schema: '閱讀資料結構 {version}', expires: '請在 {date} 前確認', warning: '確認後將完整取代目前閱讀資料。此備份建立之後的變更會被捨棄。', confirmLabel: '輸入 RESTORE 以確認', confirmWord: 'RESTORE', commit: '取代閱讀資料', committing: '正在取代閱讀資料…', cancel: '取消待還原備份', complete: '閱讀資料已還原，正在重新載入 NovelReader…' },
  tokens: { title: '自動化權杖', description: '為備份指令稿建立可撤銷權杖。每個權杖只顯示一次。', name: '權杖名稱', exportScope: '允許匯出備份', restoreScope: '允許準備及確認還原', password: '目前密碼', passwordHint: '選擇還原權限時必須填寫。', expiry: '到期時間（選填）', expiryHint: '留空則權杖不會自動到期。', create: '建立權杖', creating: '正在建立權杖…', loading: '正在載入權杖…', empty: '尚無備份自動化權杖。', secretTitle: '請立即複製此權杖', secretDescription: 'NovelReader 只儲存雜湊，之後無法再次顯示。', copy: '複製權杖', copied: '已複製', revoke: '撤銷', created: '建立於 {date}', lastUsed: '上次使用 {date}', neverUsed: '從未使用', expires: '到期於 {date}', noExpiry: '永不過期', export: '匯出', restore: '還原' },
};
