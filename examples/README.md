# Examples

本目录只演示 `contracts/` 已经确定的结构，本身不定义任何正式接口。

## 分类索引

- `http/`
  - 配置与治理：配置保存、黑白名单和命令策略请求 / 响应。
  - 日志与协议：当前会话、历史区间和 OneBot11 兼容矩阵响应。
  - 恢复与运行时：恢复确认、恢复复检和 Chromium bootstrap 请求 / 响应。
  - 三方账号：账号列表、保存、校验和扫码登录 JSON payload。
- `plugins/`：Go SDK、能力参数、Vue 管理页和 artifact 构建示例。
- `deps-manifest.sample.json`：Chromium deps manifest v5 示例。
- `backup-manifest.sample.json`：恢复包 backup manifest 示例。

## 三方账号 HTTP 面

下列 JSON 文件均使用脱敏测试值；Cookie 示例 `fixture-only-secret` 不是可用凭据。

| 操作 | 端点 | 示例或响应 |
| --- | --- | --- |
| 列出账号 | `GET /api/third-party/accounts` | `http/third-party-accounts.response.json` |
| 保存账号 | `PUT /api/third-party/accounts/{platform}/{account_id}` | `http/third-party-account-upsert.request.json`、`http/third-party-account-upsert.response.json` |
| 删除账号 | `DELETE /api/third-party/accounts/{platform}/{account_id}` | 成功返回 `204`，无响应体 |
| 校验凭据 | `POST /api/third-party/accounts/{platform}/{account_id}/validate` | `http/third-party-account-validation.response.json` |
| 读取头像 | `GET /api/third-party/accounts/{platform}/{account_id}/avatar` | 成功返回受控代理的图片二进制 |
| 创建扫码会话 | `POST /api/third-party/accounts/{platform}/login/qrcode` | `http/third-party-login-qrcode-create.response.json` |
| 轮询扫码会话 | `GET /api/third-party/accounts/{platform}/login/qrcode/{login_id}` | `http/third-party-login-qrcode-poll.response.json` |
| 取消扫码会话 | `DELETE /api/third-party/accounts/{platform}/login/qrcode/{login_id}` | 成功返回 `204`，无响应体 |

## 规则

- 示例只能演示已被 `contracts/` 确认的结构。
- 示例本身不定义任何正式接口。
- 新字段或消息类型必须先更新对应 contract。
- `plugins/` 下的示例只用于理解 manifest、插件协议、运行时客户端入口和常用 local action，不参与插件发现或正式发布。
