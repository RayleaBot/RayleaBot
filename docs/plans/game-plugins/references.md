# 参考来源与验证范围

参考快照固定于 2026-09-15，计划更新于 2026-09-18。所有上游源码引用使用完整提交 ID；本地源码仍在 `external/参考项目/2026-09-15/`，ZIP 在其 `archives/` 下。docs 只保存计划与证据清单。

## 固定来源

| 名称 | 提交 | 许可标识 | 角色 |
| --- | --- | --- | --- |
| [miao-plugin](https://github.com/yoimiya-kokomi/miao-plugin/tree/7f6f1c84c89102bc6b1c58c8e1b06c61f4642161) | `7f6f1c84c89102bc6b1c58c8e1b06c61f4642161` | MIT | 主参考 |
| [StarRail-plugin](https://github.com/TsukinaKasumi/StarRail-plugin/tree/090e411cf9be28b1644721fab54eab0ad283668f) | `090e411cf9be28b1644721fab54eab0ad283668f` | Apache-2.0 | 主参考 |
| [ZZZ-Plugin](https://github.com/ZZZure/ZZZ-Plugin/tree/e35d30d52a395eae06d58f41d0ffca1f998b183b) | `e35d30d52a395eae06d58f41d0ffca1f998b183b` | AGPL-3.0 | 主参考 |
| [Yunzai-genshin](https://github.com/TimeRainStarSky/Yunzai-genshin/tree/4a2e1fb8f094b2a8f039ce0768773701718b60b9) | `4a2e1fb8f094b2a8f039ce0768773701718b60b9` | 未发现许可证标识 | 主参考 |
| [ark-plugin](https://github.com/NotIvny/ark-plugin/tree/b7d8231ddcf606ce4f8d20e9bd743970e4f18cea) | `b7d8231ddcf606ce4f8d20e9bd743970e4f18cea` | MIT | 主参考 |
| [Axiu-Plugin](https://github.com/AxiuCN/Axiu-Plugin/tree/6f336229dce0a22344e37107a738717d0860f6b7) | `6f336229dce0a22344e37107a738717d0860f6b7` | GPL-3.0 | 主参考 |
| [xiaoyao-cvs-plugin](https://github.com/ctrlcvs/xiaoyao-cvs-plugin/tree/e7ab3e8b276a11680beb47d0c5517f6e0a4c2022) | `e7ab3e8b276a11680beb47d0c5517f6e0a4c2022` | GPL-3.0 | 主参考 |
| [Atlas](https://github.com/Nwflower/Atlas/tree/016e49357666e0823791abdf28fbb3b2efe68225) | `016e49357666e0823791abdf28fbb3b2efe68225` | GPL-3.0 | 主参考 |
| [Miao-Yunzai](https://github.com/yoimiya-kokomi/Miao-Yunzai/tree/40cc2103efba1fbb279b768e3f5345d45372d357) | `40cc2103efba1fbb279b768e3f5345d45372d357` | GPL-3.0 | 共享账号模型参考 |
| [ZZZ-Plugin-dev](https://github.com/ZZZure/ZZZ-Plugin/tree/fb66219cec0294e1834bacdf0033b2d43a9ccaf4) | `fb66219cec0294e1834bacdf0033b2d43a9ccaf4` | AGPL-3.0 | TypeScript 源码；与 main 分别固定提交 |
| [MihoyoBBSTools](https://github.com/Womsxd/MihoyoBBSTools/tree/f062d1fda8fab88fd312a5ca3a89537f6351943b) | `f062d1fda8fab88fd312a5ca3a89537f6351943b` | MIT | Axiu 固定引用的子模块，单独保存 |
| [test_nine](https://github.com/luguoyixiazi/test_nine/tree/a6a53bb46bfd33c419fa503f5a31708462893775) | `a6a53bb46bfd33c419fa503f5a31708462893775` | 未发现许可证标识 | Axiu 固定引用的子模块，单独保存 |

2026-09-19 追加的两份补充参考，未纳入上表的 12 份归档统计：

| 名称 | 提交 | 许可标识 | 角色 | 本地位置 |
| --- | --- | --- | --- | --- |
| [GachaClock](https://github.com/iaoongin/GachaClock/tree/99d16c10bfeeb5f885e9cb42861c50f993f3a746) | `99d16c10bfeeb5f885e9cb42861c50f993f3a746` | MIT | 卡池历史数据；只保存历史元数据，未下载图片 | `external/参考项目/2026-09-15/GachaClock-data/` |
| [genshin.py](https://github.com/seriaati/genshin.py/tree/075f0e1e332f9452e544aa7dfdf265678889d6ab) | `075f0e1e332f9452e544aa7dfdf265678889d6ab` | MIT | 米游社与 HoYoLab 接口、二维码登录的实现参考 | `external/参考项目/2026-09-19/genshin.py/` |

## 完整性与限制

共 12 份快照、14,358 个归档文件，展开约 1.63 GiB，ZIP 约 1.53 GiB。下载时记录归档 SHA-256，验证 ZIP 条目 CRC、展开文件大小与 CRC、路径和文件集合。

- [来源元数据](./data/sources.json) 保存获取时间、完整提交、下载 URL、摘要与文件数。
- [历史核验记录](./data/reference-verification.json) 对应原始本地参考。其旧 Markdown 链接检查记录不代表迁移后的 docs，当前文档使用仓库文档门禁。
- [静态入口](./data/command-inventory.json) 涵盖 151 个应用源码文件与 374 个声明，解析无语法诊断；[模块矩阵](./data/parity-matrix.json) 覆盖 106 个顶层应用模块。

静态条目含别名、事件入口和通配匹配，不能据此宣称全部业务已运行通过。模板、动态注册、公式数据和外部服务仍按实施方案验收。

未执行上游插件、安装依赖或使用真实账号登录。额外图库、字体、模型权重和第三方数据库不包含在普通源码归档中；其下载与再分发条件分别核对。ark 的 LICENSE 与 README 使用限制、无许可证标识项目及图片资源的具体处理见 [完整方案](./implementation.md)。

## 研究方法

研究使用 Exa 在插件来源、扫码/刷新、云功能依赖三个方向检索 20 条结果（含重复），随后核对 GitHub 固定提交源码与 RayleaBot 当前契约。应用入口通过工作区 TypeScript 解析器静态提取，不导入上游模块。

重新研究上游时先保存新的提交快照，再更新来源、命令清单、模块矩阵及行为用例。现行计划在 docs 维护；外部目录中的采集脚本与历史记录已于 2026-09-20 移出，命令清单与模块矩阵以本目录 `data/` 下的文件为准。
