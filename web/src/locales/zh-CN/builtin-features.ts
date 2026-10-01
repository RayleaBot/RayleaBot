export const builtinFeatures = {
  menuCenter: {
    title: '菜单中心',
    save: '保存',
    unsaved: '有未保存更改',
    saved: '保存完成',
    commands: {
      label: '菜单指令',
      placeholder: '输入指令后按 Enter',
    },
    prefixes: {
      label: '菜单前缀',
      placeholder: '输入前缀后按 Enter',
      inherited: '当前沿用插件指令前缀：{prefixes}',
    },
    preview: {
      rootTitle: '总菜单',
      pluginTitle: '单个插件菜单',
      selectedPlugin: '预览插件',
      noPlugins: '当前没有已启用且带指令或帮助说明的插件。',
      partial: '预览包含已加载的插件；加载更多可补全总菜单。',
    },
  },
} as const
