export const permissionPolicy = {
  title: '权限策略',
  subtitle: '指定超级管理员，并设置未单独声明权限的指令默认允许谁使用。',
  actions: {
    openAccessLists: '黑白名单',
  },
  sections: {
    superAdmins: '超级管理员',
    permission: '默认权限',
  },
  fields: {
    superAdmins: '超级管理员',
    defaultLevel: '默认权限级别',
  },
  hints: {
    superAdmins: '输入 OneBot QQ 号后按 Enter 添加。超级管理员可执行最高权限指令，并跳过黑白名单与冷却拦截。此列表不授权 QQ 官方 openid。',
    defaultLevel: '未单独声明权限的指令使用此级别。',
  },
  placeholders: {
    superAdmins: '输入 OneBot QQ 号',
  },
} as const
