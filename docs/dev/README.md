# Developer Docs

本目录用于整理 RayleaBot 的开发、调试、诊断与贡献说明。

## 当前开发入口

当前仓库的主要实现面集中在以下区域：

- `server/`：Go 服务端主链路、管理面、适配器、runtime、存储与任务系统
- `contracts/`：当前正式接口、schema、错误码与 release metadata
- `fixtures/` 与 `examples/`：契约样例、golden cases、示例插件与示例配置
- `.github/workflows/`：contracts、baseline 与 server smoke 校验

`web/` 与 `launcher/` 目前保留工程基线和默认命令，真实产品实现尚未进入开发主线。

## 调试与验证重点

- 默认命令与版本线以 `docs/engineering/baseline.md` 为准。
- 当前主验证入口是 `go test ./...` 与 `go build ./cmd/raylea-server`。
- 涉及接口、schema、错误码、事件、插件协议或 release metadata 的变更，先同步 `contracts/`，再更新实现、fixtures、示例与文档。

## 协作规则

- 开始业务实现前先确认 baseline、contracts 与 `docs/engineering/implementation-order.md` 的边界。
- 开发说明用于提供工作入口、调试路径和排障上下文，不单独定义对外接口。
- 若当前实现与目录说明存在漂移，优先以 `contracts/`、工程基线文件和已落地主链路为准。
