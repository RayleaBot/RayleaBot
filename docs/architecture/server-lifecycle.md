# Server Lifecycle

本页说明服务端启动、运行和关闭时的主要代码路径。

## 启动链路

```mermaid
flowchart TD
  cmd["cmd/raylea-server"] --> run["internal/app.Run"]
  run --> platform["internal/app platform wiring"]
  platform --> pluginstate["internal/app plugin stack"]
  pluginstate --> render["internal/app render wiring"]
  render --> services["internal/app service wiring"]
  services --> httpwire["internal/app HTTP wiring"]
  httpwire --> server["net/http.Server"]
```

`internal/app` 是组合根。它负责把配置、存储、日志、插件、渲染、事件管线和管理 HTTP 入口组装起来。业务规则留在各自领域包内，组合根只持有模块对外接口。

`App.New` 在构建任何运行期服务前获取 `<config-path>.runtime.lock`。锁已被同配置的另一实例持有时启动立即失败；构建中途失败或 `App.Close` 完成时释放该锁。

## 运行态资源

| 资源 | 主要路径 | 职责 |
| --- | --- | --- |
| 配置 | `internal/configruntime` | 读取、更新、脱敏和 apply policy |
| 存储 | `internal/storage` | SQLite schema、迁移和仓储 |
| 插件 | `internal/plugins/lifecycle`、`internal/plugins/runtime` | 插件启停、重载、进程协议和状态 |
| 事件 | `internal/eventpipeline` | 入站、桥接、分发和出站 |
| 渲染 | `internal/render` | 模板、队列、浏览器和 artifact |
| 管理入口 | `internal/management` | HTTP handlers 和 WebSocket events |

## 关闭链路

```mermaid
flowchart TD
  signal["context cancelled / shutdown request"] --> scheduler["stop scheduler"]
  scheduler --> plugins["stop runtime managers"]
  plugins --> adapter["stop adapter"]
  adapter --> http["shutdown HTTP server"]
  http --> close["App.Close"]
  close --> services["events · installer · QR sessions · tasks · render · logs"]
  services --> storage["close storage"]
  storage --> lock["release config lifecycle lock"]
```

`App.Close` 还会关闭事件栈、插件安装与卸载服务、剩余三方扫码会话、任务 executor 与 registry、渲染和日志。持久化资源和配置生命周期锁最后释放。新的长期运行资源需要接入同一关闭链路，避免关闭完成后继续无界运行。
