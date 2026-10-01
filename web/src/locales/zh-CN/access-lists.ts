export const accessLists = {
  namespace: {
    required: '请输入连接标识与机器人 ID。',
    label: '适用范围', onebotGlobal: 'OneBot · 全部机器人', onebotInstance: 'OneBot · 指定机器人',
    qqOfficial: 'QQ 官方 · 指定机器人', adapter: '连接标识', bot: '机器人 ID',
  },
  title: '黑白名单',
  subtitle: '维护白名单、黑名单和白名单启用状态。',
  actions: {
    openCommands: '查看指令中心',
    addEntry: '添加条目',
    copyTargetId: '复制目标 ID {target}',
  },
  cards: {
    whitelistTitle: '白名单',
    whitelistHelp: '白名单说明',
    whitelistDescription: '命中白名单的用户或群会进入指令分发，指令权限与冷却继续生效。',
    blacklistTitle: '黑名单',
    blacklistHelp: '黑名单说明',
    blacklistDescription: '命中黑名单的用户或群会被拦截；命中白名单时，仍会继续走权限和冷却检查。',
  },
  scopes: {
    user: '用户',
    group: '群',
  },
  entryForm: {
    remove: '移除',
    placeholderTargetId: '输入用户 ID 或群 ID',
    placeholderReason: '填写原因',
    searchPlaceholder: '搜索目标 ID 或原因',
  },
  table: {
    columns: {
      type: '类型',
      targetId: '目标 ID',
      reason: '原因',
      createdAt: '添加时间',
      actions: '操作',
    },
    total: '共 {total} 条',
    matched: '匹配 {total} 条，共 {count} 条',
  },
  filters: {
    all: '全部',
    type: '按类型筛选',
  },
  empty: {
    blacklistTitle: '暂无黑名单',
    blacklistDescription: '当前没有用户或群黑名单记录。',
    whitelistTitle: '暂无白名单',
    whitelistDescription: '当前没有用户或群白名单记录。',
    filteredTitle: '没有匹配的条目',
    filteredDescription: '调整搜索词或类型筛选，或清除全部筛选。',
    clearFilters: '清除筛选',
  },
  errors: {
    whitelistLoadFailed: '白名单读取失败',
    blacklistLoadFailed: '黑名单读取失败',
    refreshFailed: '刷新失败，下方是上次读取的条目',
  },
  whitelist: {
    enableLabel: '启用白名单',
    emptyWarningTitle: '白名单已启用且当前为空',
    emptyWarningDescription: '除超级管理员外，所有指令都会被挡下。请尽快补充条目，或先关闭白名单。',
    enableConfirmTitle: '确认启用空白名单',
    enableConfirmDescription: '当前没有任何白名单条目。启用后，除超级管理员外，所有指令都会被挡下。',
    enableConfirmAction: '确认启用',
  },
  feedback: {
    blacklistSaved: '黑名单已更新。',
    blacklistRemoved: '黑名单条目已移除。',
    whitelistSaved: '白名单已更新。',
    whitelistRemoved: '白名单条目已移除。',
    whitelistEnabled: '白名单已启用。',
    whitelistDisabled: '白名单已关闭。',
    targetIdCopied: '已复制目标 ID',
  },
  modal: {
    save: '保存',
    cancel: '取消',
  },
  confirm: {
    removeTitle: '确认移除',
    removeDescription: '将从{list}移除{type} {target}（{scope}）。',
  },
  validation: {
    entryRequired: '目标 ID 和原因都需要填写。',
  },
} as const
