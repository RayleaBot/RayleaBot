export const auth = {
  surface: '管理工作台',
  loginTitle: '登录',
  loginBody: '使用管理员账号和密钥进入管理工作区。',
  setupTitle: '创建管理员账号',
  setupBody: '首次使用时创建管理员账号并进入管理工作区。',
  identifier: '管理员账号',
  secret: '管理员密钥',
  showSecret: '显示密钥',
  hideSecret: '隐藏密钥',
  loginSubmit: '登录',
  setupSubmit: '创建并进入管理界面',
  alerts: {
    bootstrapUnavailable: '暂时无法进入管理界面',
    launcherManualLogin: '请手动登录',
    loginIncomplete: '登录未完成',
    setupIncomplete: '创建管理员账号未完成',
  },
  feedback: {
    loginSuccess: '已登录',
    setupSuccess: '管理员账号已创建',
  },
  validation: {
    identifierRequired: '请输入管理员账号',
    secretRequired: '请输入管理员密钥',
  },
} as const
