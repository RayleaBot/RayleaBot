# 基础组件源码

本目录的初始组件来自 shadcn-vue `2.8.2` CLI 的 `reka-nova` registry，获取日期为 2026-09-07。原始文件摘要记录在 `upstream.json`，许可证见 `LICENSE.shadcn-vue`。

这些文件是项目维护的源码。更新上游时先在隔离目录生成并比较，不直接覆盖本目录。CLI 版本不代表远端 registry 内容不可变，因此同时保留来源摘要。

已进行的适配：

- 类名合并统一使用 `@/lib/ui`，视觉变量映射到项目现有 token。
- 将 registry 状态变体显式映射为 Reka 的 `data-state` 属性。
- 移除 registry 的进入、退出、位移和毛玻璃预设，动画由产品组件和统一配置决定。
- 使用项目自托管字体；没有导入 registry 的全局 CSS 或外部字体请求。

业务页面通过 `components/` 中的产品封装使用交互复杂的浮层、选择器和反馈组件；本目录只承担可组合的基础结构与状态。
