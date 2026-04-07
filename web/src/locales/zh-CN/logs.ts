export const logs = {
  title: '日志',
  refresh: '刷新日志',
  filters: {
    level: '级别',
    source: '来源',
    plugin: '插件',
    requestId: '请求 ID',
    apply: '应用筛选',
    all: '全部',
    sourcePlaceholder: '例如 runtime / adapter.onebot11',
    pluginPlaceholder: '例如 weather',
    requestPlaceholder: '例如 req_*',
  },
  fields: {
    timestamp: '时间',
    level: '级别',
    source: '来源',
    plugin: '插件',
    requestId: '请求 ID',
    message: '内容',
  },
} as const
