export const commands = {
  title: '指令中心',
  subtitle: '集中查看当前插件声明的全部指令与可用状态。',
  refresh: '刷新列表',
  filters: {
    plugins: '按插件筛选',
    allPlugins: '全部插件',
  },
  empty: '当前没有可展示的指令。',
  fields: {
    command: '命令',
    aliases: '别名',
    description: '说明',
    usage: '用法',
    permission: '权限',
    plugin: '所属插件',
    status: '当前状态',
  },
  status: {
    available: '当前可用',
    starting: '启动中',
    switching: '切换中',
    not_ready: '未就绪',
    disabled: '已停用',
  },
} as const
