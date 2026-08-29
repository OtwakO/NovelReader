export default {
  title: '备份与恢复',
  description: '保存完整阅读数据的便携副本，或从可信的 NovelReader 备份替换当前数据。',
  export: { title: '导出阅读数据', description: '下载当前阅读数据的一致时间点快照。不包含账户凭据。', included: '包含内容', items: ['书架', '进度', '书签', '书源', '偏好与字体'], action: '下载备份', busy: '正在准备备份…', failed: '无法下载备份。' },
  restore: { title: '恢复阅读数据', description: '先上传并验证。只有明确确认后才会更改数据。', choose: '选择 .tar.gz 备份', selected: '已选文件', prepare: '上传并验证', preparing: '正在验证备份…', failed: '无法准备此备份。', ready: '数据验证成功，准备恢复', source: '导出自 {username}', created: '创建于 {date}', schema: '阅读数据架构 {version}', expires: '请在 {date} 前确认', warning: '确认后将完整替换当前阅读数据。此备份创建之后的更改会被丢弃。', confirmLabel: '输入 RESTORE 以确认', confirmWord: 'RESTORE', commit: '替换阅读数据', committing: '正在替换阅读数据…', cancel: '取消待恢复备份', complete: '阅读数据已恢复，正在重新载入 NovelReader…' },
  tokens: { title: '自动化令牌', description: '为备份脚本创建可撤销令牌。每个令牌只显示一次。', name: '令牌名称', exportScope: '允许导出备份', restoreScope: '允许准备和确认恢复', password: '当前密码', passwordHint: '选择恢复权限时必须填写。', expiry: '到期时间（可选）', expiryHint: '留空则令牌不会自动到期。', create: '创建令牌', creating: '正在创建令牌…', loading: '正在载入令牌…', empty: '尚无备份自动化令牌。', secretTitle: '请立即复制此令牌', secretDescription: 'NovelReader 只保存哈希，之后无法再次显示。', copy: '复制令牌', copied: '已复制', revoke: '撤销', created: '创建于 {date}', lastUsed: '上次使用 {date}', neverUsed: '从未使用', expires: '到期于 {date}', noExpiry: '永不过期', export: '导出', restore: '恢复' },
};
