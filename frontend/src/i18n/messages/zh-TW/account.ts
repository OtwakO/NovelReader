export default {
  login: { title: '歡迎回來', intro: '登入後繼續存取你的書架、書源、閱讀進度與書籤。', username: '使用者名稱', password: '密碼', submit: '登入', submitting: '正在登入…', failed: '登入失敗。', help: '帳戶協助', register: '建立帳戶', reset: '使用重設權杖', recovery: '管理員復原', expired: '登入已過期，請重新登入後繼續。' },
  register: { title: '建立閱讀帳戶', intro: '你的書架、書源、進度與檔案會保存在獨立的私人閱讀空間中。', invite: '邀請碼', submit: '建立帳戶', submitting: '正在建立…', failed: '建立帳戶失敗。' },
  reset: { title: '重設閱讀帳戶密碼', intro: '輸入管理員提供的一次性權杖並設定新密碼。權杖將在 30 分鐘後失效。', token: '重設權杖', newPassword: '新密碼', confirm: '確認新密碼', mismatch: '兩次輸入的密碼不一致。', submit: '重設密碼', submitting: '正在重設…', failed: '密碼重設失敗。', complete: '密碼重設完成，請使用新密碼登入。' },
  setup: { title: '設定 NovelReader', intro: '為此安裝建立第一個管理員帳戶。', token: '初始化權杖', tokenHint: '貼上伺服器設定中的 ADMIN_BOOTSTRAP_TOKEN 暫時值。', confirm: '確認密碼', mismatch: '兩次輸入的密碼不一致。', submit: '建立管理員', submitting: '正在建立…', failed: '初始化失敗。', aftercare: '設定成功後，請從伺服器環境中移除 ADMIN_BOOTSTRAP_TOKEN。', unavailableTitle: '伺服器尚未開放初始化', unavailableIntro: '請設定 ADMIN_BOOTSTRAP_TOKEN，重新啟動 NovelReader，然後重新整理此頁面。' },
  recovery: { title: '復原管理員存取權', intro: '使用伺服器暫時設定的復原權杖。完成後請立即移除該環境變數。', checking: '正在檢查復原設定…', action: '復原方式', resetExisting: '重設現有管理員', createReplacement: '建立替代管理員', token: '復原權杖', username: '管理員使用者名稱', newPassword: '新密碼', confirm: '確認新密碼', mismatch: '兩次輸入的密碼不一致。', submit: '復原並登入', submitting: '正在復原…', failed: '管理員復原失敗。', statusFailed: '無法讀取復原狀態。', unavailable: '伺服器未啟用管理員復原。請暫時設定 ADMIN_RECOVERY_TOKEN，重新啟動後再返回此頁面。' },
  common: { backToLogin: '返回登入' },
};
