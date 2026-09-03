# Text Resources

本页说明 RayleaBot 当前文本资源与国际化的边界。

## 当前落点

- Web 使用 `vue-i18n` 和 `zh-CN` 资源；当前没有已交付的 `en-US` 资源集。
- Server 的 HTTP 错误 envelope 提供稳定 `code` 与 `message_key`，Web 可据此选择本地化文案；程序分支不能依赖可读 `message`。
- Launcher renderer 当前直接维护中文界面文案，没有共享 i18n 资源层；生成的 Web API 类型中出现 `message_key` 不代表 Launcher 已接入统一资源。
- CLI 当前输出中文摘要和稳定结构化 issue code，没有 `message_key` 字段，也不与 Web 共用资源文件。
- 模板固定文案由各模板受控维护；多语言扩展不能改变正式错误码、字段、状态名或模板输入 contract。

## 维护原则

- 只在已经接入资源层的表面复用资源键，不把未来国际化设计描述成当前能力。
- 新增语言前先明确该客户端的资源归属方、回退语言和缺键行为，再同步测试与用户文档。
- 文案一致性不改变 contract 的稳定英文 wire value。
