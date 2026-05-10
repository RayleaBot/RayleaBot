export const builtinFeatures = {
  menuCenter: {
    title: '菜单中心',
    subtitle: '配置内置菜单命令，并预览聊天菜单内容。',
    refresh: '刷新',
    save: '保存',
    unsaved: '有未保存更改',
    saved: '保存完成',
    commands: {
      label: '菜单命令',
      placeholder: '输入命令后按 Enter',
    },
    prefixes: {
      label: '菜单前缀',
      placeholder: '输入前缀后按 Enter',
      inherited: '当前沿用插件命令前缀：{prefixes}',
    },
    preview: {
      rootTitle: '总菜单预览',
      pluginTitle: '插件菜单预览',
      selectedPlugin: '预览插件',
      allPlugins: '全部插件',
      commandLine: '触发命令',
      noPlugins: '当前没有可预览的启用插件。',
      noPluginHelp: '当前插件没有菜单项。',
      permission: {
        everyone: '所有成员',
        group_admin: '群管理员',
        super_admin: '超级管理员',
      },
    },
  },
} as const
