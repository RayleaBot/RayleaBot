export const builtinFeatures = {
  menuCenter: {
    title: '菜单中心',
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
      noPlugins: '当前没有可预览的启用插件。',
      partial: '预览包含已加载的运行中插件；加载更多可补全总菜单。',
    },
  },
} as const
