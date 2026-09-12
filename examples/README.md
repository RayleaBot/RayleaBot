# Examples

本目录只演示 `contracts/` 已经确定的结构，本身不定义任何正式接口。

## 分类索引

- `http/`
  - 配置与治理：配置保存、黑白名单和命令策略请求 / 响应。
  - 日志与协议：当前会话、历史区间和 OneBot11 兼容矩阵响应。
  - 恢复与运行时：恢复确认、恢复复检和 Chromium bootstrap 请求 / 响应。
- `plugins/`：Go SDK、能力参数、Vue 管理页和 artifact 构建示例。
- `deps-manifest.sample.json`：Chromium deps manifest v5 示例。
- `backup-manifest.sample.json`：恢复包 backup manifest 示例。

## 规则

- 示例只能演示已被 `contracts/` 确认的结构。
- HTTP JSON 示例必须在 [`http/index.yaml`](./http/index.yaml) 中登记 `operationId`、请求或响应方向、状态码、媒体类型与包装方式。无响应状态的请求填写 `status: null`；只演示方法和 URL 的请求填写 `representation: request` 与 `media_type: null`。CI 通过同一 OpenAPI 校验器验证 fixtures 与示例，未登记或失效的映射均会失败。
- 示例本身不定义任何正式接口。
- 新字段或消息类型必须先更新对应 contract。
- `plugins/` 下的示例只用于理解 manifest、插件协议、运行时客户端入口和常用 local action，不参与插件发现或正式发布。
