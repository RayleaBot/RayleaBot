# Fixtures

本目录承载由 `contracts/` 派生的 golden inputs / outputs 与长期回归样例。

规则：

- fixture 不能先于 contract 发明字段、状态名、错误码或消息类型。
- 后续应按 contract 分类建立子目录，例如 config、web-api、websocket、plugin-info、plugin-protocol。
- fixture 只演示已经被正式契约确认的结构。
