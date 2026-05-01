export const commands = {
  title: '指令中心',
  subtitle: '查看当前生效的命令策略，并核对插件声明指令。',
  refresh: '刷新列表',
  actions: {
    openPermissionPolicy: '权限策略',
  },
  filters: {
    plugins: '按插件筛选',
    allPlugins: '全部插件',
  },
  empty: {
    title: '暂无指令',
    description: '当前没有插件声明可用指令',
    effectiveTitle: '暂无生效策略',
    effectiveDescription: '当前没有参与命令裁决的插件指令。',
  },
  fields: {
    command: '命令',
    aliases: '别名',
    description: '说明',
    usage: '用法',
    permission: '权限',
    declaredPermission: '声明权限',
    effectivePermission: '生效权限',
    permissionSource: '权限来源',
    plugin: '所属插件',
    status: '当前状态',
  },
  sections: {
    effectivePolicies: '生效命令策略',
    declaredCommands: '全部声明命令',
  },
  status: {
    available: '当前可用',
    starting: '启动中',
    switching: '切换中',
    not_ready: '未就绪',
    disabled: '已停用',
  },
  permissions: {
    everyone: '所有成员',
    groupAdmin: '群管理员',
    superAdmin: '超级管理员',
  },
  permissionDefault: '跟随默认权限',
  permissionSource: {
    declared: '命令声明',
    default_level: '默认权限',
  },
} as const
