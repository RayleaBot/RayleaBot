export const commands = {
  title: '指令中心',
  actions: {
    openPermissionPolicy: '权限策略',
  },
  filters: {
    plugins: '按插件筛选',
    allPlugins: '全部插件',
  },
  empty: {
    title: '暂无指令',
    partialTitle: '已加载的插件没有指令',
    partialDescription: '继续加载或按插件搜索，查看其他插件的指令。',
    description: '当前没有可展示的插件指令。',
  },
  fields: {
    command: '指令',
    aliases: '别名',
    description: '说明',
    usage: '用法',
    permission: '权限',
    declaredPermission: '声明权限',
    permissionSource: '权限来源',
    source: '触发方式',
    plugin: '所属插件',
    status: '当前状态',
  },
  sections: {
    commandList: '指令列表',
  },
  aria: {
    command: '指令：{name}',
    availability: '可用性：{status}',
  },
  status: {
    available: '当前可用',
    starting: '启动中',
    not_running: '未运行',
    switching: '停止中',
    not_ready: '未就绪',
    disabled: '已停用',
  },
  permissions: {
    everyone: '所有成员',
    groupAdmin: '群管理员',
    superAdmin: '超级管理员',
  },
  permissionDefault: '跟随默认权限',
  permissionInactive: '尚未生效',
  permissionSource: {
    declared: '指令声明',
    default_level: '默认权限',
  },
  commandSource: {
    exact: '固定指令',
    setting: '设置指令',
    pattern: '规则指令',
  },
} as const
