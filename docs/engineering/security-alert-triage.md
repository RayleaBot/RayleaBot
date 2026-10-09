# CodeQL 告警核查

本页记录需要人工复核的数据流告警，不替代契约，也不屏蔽扫描规则。2026-10-09 的本地复扫使用 CodeQL CLI 2.27.2、Go queries 1.6.12；以下两条仍出现在结果中。**两条 GitHub 告警均保持开启，须在修复合入默认分支、远端复扫后再按下述依据关闭。**

## 本地插件包路径：#63

[告警 #63](https://github.com/RayleaBot/RayleaBot/security/code-scanning/63) 指向 [`SHA256File`](../../server/internal/platform/fsguard/digest.go) 的文件打开操作，规则为 `go/path-injection`。

[`PluginInstallRequest`](../../contracts/web-api.openapi.yaml) 明确允许管理员指定本地插件目录或 ZIP 文件。[管理安装入口](../../server/internal/management/plugins.go) 位于受鉴权的管理路由，严格解码请求并要求 `trusted_code_confirmed: true`；任意本地包选择是这一受信任操作的既有能力。

此次报告的数据流把 HTTP 请求中的 `source` 连到了[来源摘要校验](../../server/internal/plugins/lifecycle/install_sources.go)。但该校验在预期摘要为空时立即返回；HTTP 请求 schema 不暴露 `ExpectedArchiveSHA256`，严格解码也不允许附加该字段。商店摘要由 [market 安装流程](../../server/internal/plugins/market/service.go) 从所选发布资产中填写，下载后校验的是安装临时目录中的文件。

处理结论：保留契约允许的本地安装能力。默认分支复扫仍报告同一条调用链时，可按 `false positive` 关闭 #63。若未来管理请求允许提供预期摘要，或本地文件能力向非管理调用方开放，须重新核查该结论。

## 商店请求 URL：#71

[告警 #71](https://github.com/RayleaBot/RayleaBot/security/code-scanning/71) 指向 [`Service.fetch`](../../server/internal/plugins/market/service.go) 的 HTTP 请求，规则为 `go/request-forgery`。

自定义 HTTPS 来源是既有功能，因此 URL 的主机名会来自管理请求。修复前仅检查 URL 字面值，域名解析后的私网地址仍可绕过该检查；**不能以旧实现中的 URL 检查为由关闭此告警**。

本次修复在[默认商店 HTTP client](../../server/internal/plugins/market/http_client.go) 的拨号阶段检查完整 DNS 地址集合，拒绝 loopback、私网、链路本地、组播、未指定地址、共享地址空间及保留的 IPv4 空间，然后只连接已检查的数字 IP，不再次解析原始域名。请求保留原始主机名用于 Host 和 TLS 证书验证。重定向继续检查 HTTPS、userinfo 和主机地址，并受同一拨号保护。商店目录请求直连，避免代理自行解析目标而绕过检查。

[生产装配](../../server/internal/app/plugin_stack.go) 使用这一默认 client；`HTTPClient` 构造参数供受信任的代码注入，不能由管理请求配置。CodeQL 的 URL 数据流规则没有识别这层自定义拨号保护，因此仍保留该提示。

处理结论：合入 DNS 防护后，确认远端复扫指向同一受保护调用链，再按 `false positive` 关闭 #71。依据包括 [DNS、数字 IP 拨号、重定向和默认 client 回归测试](../../server/internal/plugins/market/http_client_test.go)。替换 transport、增加代理或开放 client 配置时，须重新核查这一结论。
