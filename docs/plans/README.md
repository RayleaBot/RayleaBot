# 执行计划

执行计划描述待实施的设计、依赖和验收条件。本目录只保存主仓库（宿主、Web、Launcher、SDK 与仓库工程）的计划；已落地并写入现行文档的计划不再保留。当前已提供的接口仍以 [contracts](../../contracts/README.md) 为准。

当前条目：

- [竞品分析（2026-10）](./competitive-analysis-2026-10.md)：对照 AstrBot、Yunzai、NoneBot2、Koishi、LangBot 等同类项目，按优先级列出功能缺口、有意取舍与优势口径，供维护者决定哪些项进入实施。
- [插件管理页按机器人账号分页](./bot-account-scoped-pages.md)：宿主提供机器人账号目录与按账号的插件页面切换，插件管理页按账号划分数据，替代各插件混在一起的列表与逐行机器人选择。

独立插件的业务计划（例如游戏插件的验收与交付）在对应插件仓库的 `docs/` 目录维护，不写入本目录，规则见 [AGENTS.md](./AGENTS.md)。
