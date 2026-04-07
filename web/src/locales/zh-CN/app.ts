export const app = {
  brand: 'RayleaBot',
  consoleName: '管理控制台',
  skipToMain: '跳到主内容',
} as const

export const routes = {
  status: '系统状态',
  plugins: '插件',
  pluginDetail: '插件详情',
  tasks: '任务',
  logs: '日志',
  config: '配置',
  login: '登录',
  setup: '创建管理员账号',
} as const

export const shell = {
  systemStatus: '系统状态',
  readyStatus: '就绪状态',
  reconnectAll: '全部重连',
  logout: '退出登录',
  shutdown: '关闭服务',
  shutdownConfirmTitle: '确认关闭服务',
  shutdownConfirmBody: '关闭服务后，管理界面连接会中断。',
  shutdownConfirmAction: '确认关闭',
  shutdownAccepted: '停机请求已发送',
  shutdownRequestedTitle: '服务正在停止',
  shutdownRequestedDescription: '服务正在停止，管理界面连接断开属于预期行为。',
  shutdownRequestedLive: '服务已收到关闭请求，连接断开属于预期行为。',
} as const
