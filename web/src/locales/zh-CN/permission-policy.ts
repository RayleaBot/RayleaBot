export const permissionPolicy = {
  title: '权限策略',
  actions: {
    openAccessLists: '黑白名单',
  },
  sections: {
    summary: '策略总览',
    settings: '策略配置',
    superAdmins: '超级管理员',
    permission: '默认权限',
    user: '用户命令',
    group: '群命令',
  },
  summary: {
    superAdmins: '超级管理员',
    superAdminsMeta: '拥有最高权限并跳过名单与冷却裁决的用户数量。',
    defaultPermission: '默认权限',
    defaultPermissionMeta: '未声明权限的命令使用该级别。',
    userCooldown: '用户冷却',
    groupCooldown: '群冷却',
    cooldownReplyEnabled: '会发送提示',
    cooldownReplyDisabled: '不发送提示',
    cooldownReply: '冷却提示',
    cooldownReplyDescription: '命令被冷却挡下时的自动回复开关。',
    userCooldownMeta: '按同一用户累计 · 当前值 {value}',
    groupCooldownMeta: '按同一群累计 · 当前值 {value}',
  },
  fields: {
    superAdmins: '超级管理员',
    defaultLevel: '默认权限级别',
    userCommandRateLimit: '用户命令速率限制',
    groupCommandRateLimit: '群命令速率限制',
    cooldownReply: '冷却提示',
  },
  hints: {
    superAdmins: '输入用户 ID 后按 Enter 添加。超级管理员可执行最高权限命令。',
    userCommandRateLimit: '同一用户在一个滑动时间窗口内最多触发多少次命令。',
    groupCommandRateLimit: '同一群在一个滑动时间窗口内合计最多触发多少次命令。',
    cooldownReply: '开启后，命令因冷却被挡下时会自动回复一条提示消息。',
  },
  placeholders: {
    superAdmins: '输入用户 ID',
  },
  status: {
    unsaved: '有未保存更改',
    savedHot: '保存完成，已生效',
    savedRestart: '保存完成，重启后生效',
  },
  empty: {
    summaryTitle: '暂无权限策略快照',
    summaryDescription: '当前未读取到权限策略摘要。',
  },
} as const
