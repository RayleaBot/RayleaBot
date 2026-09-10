export const permissionPolicy = {
  title: '权限策略',
  actions: {
    openAccessLists: '黑白名单',
  },
  sections: {
    settings: '策略配置',
    superAdmins: '超级管理员',
    permission: '默认权限',
    user: '冷却提示',
    group: '群命令',
  },
  fields: {
    superAdmins: '超级管理员',
    defaultLevel: '默认权限级别',
    userCommandRateLimit: '用户命令速率限制',
    groupCommandRateLimit: '群命令速率限制',
    cooldownReply: '冷却提示',
  },
  hints: {
    superAdmins: '输入 OneBot QQ 号后按 Enter 添加。超级管理员可执行最高权限命令，并跳过黑白名单与冷却拦截。此列表不授权 QQ 官方 openid。',
    defaultLevel: '未单独声明权限的命令使用此级别。',
    userCommandRateLimit: '同一用户在一个滑动时间窗口内最多触发多少次命令。',
    groupCommandRateLimit: '同一群在一个滑动时间窗口内合计最多触发多少次命令。',
    cooldownReply: '开启后，命令因冷却被挡下时会自动回复一条提示消息。',
  },
  placeholders: {
    superAdmins: '输入 OneBot QQ 号',
  },
  status: {
    unsaved: '有未保存更改',
    savedHot: '保存完成，已生效',
    savedRestart: '保存完成，重启后生效',
  },
} as const
