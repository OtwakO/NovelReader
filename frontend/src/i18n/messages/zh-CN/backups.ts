export default {
  title: '备份与恢复',
  description: '保存完整阅读数据的便携副本，或从可信的 NovelReader 备份替换当前数据。',
  export: { title: '导出阅读数据', description: '下载当前阅读数据的一致时间点快照。不包含账户凭据。', included: '包含内容', items: ['书架', '进度', '书签', '书源', '偏好与字体'], action: '下载备份', busy: '正在准备备份…', failed: '无法下载备份。' },
  restore: {
    checkOutcome: '检查恢复状态', cancelUnstarted: '取消尚未开始的恢复', acknowledgeUnknown: '使用全新阅读状态继续',
    commitFailed: '恢复未完成，此操作不能重试。请先检查当前书库，再准备新的恢复操作。',
    unknown: '恢复结果已无法查询。请检查当前书库，不要假定恢复成功或失败。',
    outcome: { committing: '正在恢复，旧的阅读活动已停止。请检查状态后再继续。', unavailable: '无法确认恢复响应。阅读活动已停止，请在连接恢复后检查状态。系统不会自动重试替换操作。', prepared: '服务器尚未开始恢复。请取消此准备，以阻止结果不明的请求随后执行，再继续操作。', unknown: '恢复记录已过期、被新操作替代或因服务器重启而丢失，结果未知。请使用全新阅读状态继续，并检查书库。' },
    title: '恢复阅读数据', description: '先上传并验证。只有明确确认后才会更改数据。', choose: '选择 .tar.gz 备份', selected: '已选文件', prepare: '上传并验证', preparing: '正在验证备份…', failed: '无法准备此备份。', ready: '数据验证成功，准备恢复', source: '导出自 {username}', created: '创建于 {date}', schema: '阅读数据架构 {version}', expires: '请在 {date} 前确认', warning: '确认后将完整替换当前阅读数据。此备份创建之后的更改会被丢弃。', confirmLabel: '输入 RESTORE 以确认', confirmWord: 'RESTORE', commit: '替换阅读数据', committing: '正在替换阅读数据…', cancel: '取消待恢复备份', complete: '阅读数据已恢复，正在重新载入 NovelReader…', recoveryWarning: '数据已恢复。部分恢复后处理或旧文件清理未完成，相关记录或文件已保留以供处理。请联系服务器管理员检查日志。你可以继续使用书库。' },
  tokens: { title: '自动化令牌', description: '为备份脚本创建可撤销令牌。每个令牌只显示一次。', name: '令牌名称', exportScope: '允许导出备份', restoreScope: '允许准备和确认恢复', password: '当前密码', passwordHint: '选择恢复权限时必须填写。', expiry: '到期时间（可选）', expiryHint: '留空则令牌不会自动到期。', create: '创建令牌', creating: '正在创建令牌…', loading: '正在载入令牌…', empty: '尚无备份自动化令牌。', secretTitle: '请立即复制此令牌', secretDescription: 'NovelReader 只保存哈希，之后无法再次显示。', copy: '复制令牌', copied: '已复制', revoke: '撤销', created: '创建于 {date}', lastUsed: '上次使用 {date}', neverUsed: '从未使用', expires: '到期于 {date}', noExpiry: '永不过期', export: '导出', restore: '恢复' },
  api: { title: '自动化 API', description: '用于定时备份和读者间数据替换的端点说明。', show: '显示端点文档', auth: '使用 Authorization: Bearer <token> 发送自动化令牌。令牌所属读者始终是导出或被替换数据的读者；请求不能指定其他读者 ID。', export: '以 application/gzip 下载令牌所属读者的阅读数据。', prepare: '上传 application/gzip，验证后返回 operationId；当前数据不会改变。', status: '读取准备中、执行中、已完成或失败的状态。响应不明时应检查状态，不要重复提交恢复。', commit: '使用已准备的备份原子替换令牌所属读者的阅读数据。', cancel: '丢弃待恢复备份，不更改当前数据。', exampleTitle: '从一名读者导出并恢复到另一名读者', exampleTabs: 'API 示例语言', exampleDescription: '源读者使用自己的导出令牌，目标读者使用自己的恢复令牌。', note: '待恢复备份会在 30 分钟后到期。确认恢复是单独请求，使上传和验证在破坏性切换前完成。请使用 HTTPS，并将备份文件视为敏感数据。' },
};
