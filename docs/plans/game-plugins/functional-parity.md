# 功能复刻矩阵

## 范围与状态

本矩阵覆盖本次八个插件参考的全部 106 个顶层应用模块，并关联 151 个应用源码文件、374 个静态命令/事件处理声明。另用 Miao-Yunzai 核对共享账号模型。声明数量含别名、事件入口和通配规则，不等于独立业务功能数。

用户已确定：由宿主通用服务调用连接业务插件；默认所有插件可信，仅加密 CK 并分离显示；保留云功能、优先现有服务；国服先交付、海外保留；采用原生界面与管理。各模块的归属与交付方式见下文；实现状态以[返工计划](./rework-plan.md)为准，机器矩阵中的 `implementation_status` 与 `implementation_evidence` 是审计前的批量标注，不作为完成依据。“复刻”指输入、结果、状态、算法和必要交互的可观察一致性；“分拆”指放入独立游戏或 CK 插件；“原生/安全替代”明确改变旧接入方式；“外部依赖”仍在完整目标内，未获得接口或数据前不能宣布完成。“范围待定”保留补充参考带来的额外业务，未擅自视为用户已排除。

通用字段与调用生命周期见 [通用插件服务计划](../plugin-services.md)。所有账号相关条目经通用 SDK 调用账号插件，业务字段放在 `params` 中；矩阵中的原神、星铁、绝区零、CK 名称属于业务归属，不进入宿主专用分支。

原始表达式、处理函数、权限线索、源码位置见 [命令与入口清单](./command-inventory.md) 与 [机器清单](./data/command-inventory.json)。`accept`、动态别名、模板变体及定时钩子须结合相应函数验收，不以静态条目数证明运行时覆盖率。

## miao-plugin

| 源模块 | 归属 | 交付方式 | 功能与边界 |
| --- | --- | --- | --- |
| [apps/admin.js](https://github.com/yoimiya-kokomi/miao-plugin/blob/7f6f1c84c89102bc6b1c58c8e1b06c61f4642161/apps/admin.js) | 原神、星铁 | 原生替代 | 设置、面板服务额度、资源更新、版本与更新日志，各游戏独立管理 |
| [apps/alias.js](https://github.com/yoimiya-kokomi/miao-plugin/blob/7f6f1c84c89102bc6b1c58c8e1b06c61f4642161/apps/alias.js) | 原神、星铁 | 分拆复刻 | 默认与自定义角色别名、增删、列表、帮助、歧义判定与热更新 |
| [apps/character.js](https://github.com/yoimiya-kokomi/miao-plugin/blob/7f6f1c84c89102bc6b1c58c8e1b06c61f4642161/apps/character.js) | 原神、星铁 | 分拆复刻 | 角色卡片、老婆老公列表、随机照片、图片上传、回复取原图、隐式角色名入口 |
| [apps/gacha.js](https://github.com/yoimiya-kokomi/miao-plugin/blob/7f6f1c84c89102bc6b1c58c8e1b06c61f4642161/apps/gacha.js) | 原神、星铁 | 分拆复刻 | 详细抽卡记录、按池和版本统计、单角色/武器记录、帮助 |
| [apps/help.js](https://github.com/yoimiya-kokomi/miao-plugin/blob/7f6f1c84c89102bc6b1c58c8e1b06c61f4642161/apps/help.js) | 原神、星铁 | 原生替代 | 帮助、版本、主题；按实际安装、开关与权限生成 |
| [apps/index.js](https://github.com/yoimiya-kokomi/miao-plugin/blob/7f6f1c84c89102bc6b1c58c8e1b06c61f4642161/apps/index.js) | 原神、星铁 | 原生替代 | 应用注册和加载顺序转为原生插件入口，保留其业务可达性 |
| [apps/poke.js](https://github.com/yoimiya-kokomi/miao-plugin/blob/7f6f1c84c89102bc6b1c58c8e1b06c61f4642161/apps/poke.js) | 原神、星铁 | 分拆复刻 | 戳一戳角色/老婆互动，独立开关与多插件响应去重 |
| [apps/profile.js](https://github.com/yoimiya-kokomi/miao-plugin/blob/7f6f1c84c89102bc6b1c58c8e1b06c61f4642161/apps/profile.js) | 原神、星铁 | 分拆复刻 | 面板获取、变换、换装、伤害、圣遗物/遗器评分、练度、天赋/行迹、图像、排名、删除与重载 |
| [apps/stat.js](https://github.com/yoimiya-kokomi/miao-plugin/blob/7f6f1c84c89102bc6b1c58c8e1b06c61f4642161/apps/stat.js) | 原神 | 复刻并保留外部依赖 | 深渊/剧诗/幽境、持有率命座分布、出场率、配队、本地挑战报告（固定快照上传方法无调用点）、月谕圣牌收藏与群交换 |
| [apps/wiki.js](https://github.com/yoimiya-kokomi/miao-plugin/blob/7f6f1c84c89102bc6b1c58c8e1b06c61f4642161/apps/wiki.js) | 原神、星铁、绝区零 | 分拆复刻 | 角色天赋/命座资料、材料日历、今日素材、三个游戏活动日历 |

## StarRail-plugin

| 源模块 | 归属 | 交付方式 | 功能与边界 |
| --- | --- | --- | --- |
| [apps/alias.js](https://github.com/TsukinaKasumi/StarRail-plugin/blob/090e411cf9be28b1644721fab54eab0ad283668f/apps/alias.js) | 星铁 | 复刻 | 角色及光锥别名、增删与列表 |
| [apps/challenge.js](https://github.com/TsukinaKasumi/StarRail-plugin/blob/090e411cf9be28b1644721fab54eab0ad283668f/apps/challenge.js) | 星铁 | 复刻 | 忘却之庭、混沌回忆、虚构叙事、末日幻影、异相仲裁，当前/历史和简易视图 |
| [apps/gachasimulation.js](https://github.com/TsukinaKasumi/StarRail-plugin/blob/090e411cf9be28b1644721fab54eab0ad283668f/apps/gachasimulation.js) | 星铁 | 复刻 | 模拟跃迁、池选择、概率、保底和模拟历史，与真实记录分别保存 |
| [apps/gatcha.js](https://github.com/TsukinaKasumi/StarRail-plugin/blob/090e411cf9be28b1644721fab54eab0ad283668f/apps/gatcha.js) | 星铁 | 安全替代 | 抽卡同步、分析、帮助；带 AuthKey 的链接交由 CK 代理消费，提供无凭据记录导入导出 |
| [apps/gridFight.js](https://github.com/TsukinaKasumi/StarRail-plugin/blob/090e411cf9be28b1644721fab54eab0ad283668f/apps/gridFight.js) | 星铁 | 复刻 | 货币战争总览及最近十场战绩/回顾 |
| [apps/help.js](https://github.com/TsukinaKasumi/StarRail-plugin/blob/090e411cf9be28b1644721fab54eab0ad283668f/apps/help.js) | 星铁 | 原生替代 | 独立帮助与主题 |
| [apps/hkrpg.js](https://github.com/TsukinaKasumi/StarRail-plugin/blob/090e411cf9be28b1644721fab54eab0ad283668f/apps/hkrpg.js) | 星铁、CK | 分拆复刻 | UID绑定归 CK；账号卡片、充值记录、在线时长估算归星铁，保留估算数据来源说明 |
| [apps/MiaoOrStarrail.js](https://github.com/TsukinaKasumi/StarRail-plugin/blob/090e411cf9be28b1644721fab54eab0ad283668f/apps/MiaoOrStarrail.js) | 星铁 | 原生替代 | 整合与 miao 重叠命令，标准前缀及可配置别名避免抢占 |
| [apps/month.js](https://github.com/TsukinaKasumi/StarRail-plugin/blob/090e411cf9be28b1644721fab54eab0ad283668f/apps/month.js) | 星铁 | 复刻 | 本月与历史星琼收入、分类明细 |
| [apps/note.js](https://github.com/TsukinaKasumi/StarRail-plugin/blob/090e411cf9be28b1644721fab54eab0ad283668f/apps/note.js) | 星铁 | 复刻 | 开拓力、委托等实时便签 |
| [apps/panel.js](https://github.com/TsukinaKasumi/StarRail-plugin/blob/090e411cf9be28b1644721fab54eab0ad283668f/apps/panel.js) | 星铁 | 复刻并保留外部依赖 | 面板、模板、原图、数据源选择与列表，Enka/Mihomo 等公开来源适配 |
| [apps/rogue.js](https://github.com/TsukinaKasumi/StarRail-plugin/blob/090e411cf9be28b1644721fab54eab0ad283668f/apps/rogue.js) | 星铁 | 复刻 | 模拟宇宙、寰宇蝗灾、黄金与机械、不可知域、差分宇宙及常规/周期演算记录 |
| [apps/srxx.js](https://github.com/TsukinaKasumi/StarRail-plugin/blob/090e411cf9be28b1644721fab54eab0ad283668f/apps/srxx.js) | 星铁 | 复刻并保留外部依赖 | 参考面板、强度榜、收益曲线、攻略、星琼预估、模拟宇宙攻略、商店光锥建议 |
| [apps/strategy.js](https://github.com/TsukinaKasumi/StarRail-plugin/blob/090e411cf9be28b1644721fab54eab0ad283668f/apps/strategy.js) | 星铁 | 复刻并保留外部依赖 | 角色攻略来源、帮助、默认来源设置与图片缓存 |
| [apps/update.js](https://github.com/TsukinaKasumi/StarRail-plugin/blob/090e411cf9be28b1644721fab54eab0ad283668f/apps/update.js) | 星铁 | 原生替代 | 正式 artifact 更新及资源包更新，不执行聊天传入的 Git/Shell |
| [apps/version.js](https://github.com/TsukinaKasumi/StarRail-plugin/blob/090e411cf9be28b1644721fab54eab0ad283668f/apps/version.js) | 星铁 | 复刻 | 插件版本与更新日志 |

## ZZZ-Plugin

| 源模块 | 归属 | 交付方式 | 功能与边界 |
| --- | --- | --- | --- |
| [dist/apps/abyss.js](https://github.com/ZZZure/ZZZ-Plugin/blob/e35d30d52a395eae06d58f41d0ffca1f998b183b/dist/apps/abyss.js) | 绝区零 | 复刻 | 式舆防卫战、历史期数、节点明细 |
| [dist/apps/calendar.js](https://github.com/ZZZure/ZZZ-Plugin/blob/e35d30d52a395eae06d58f41d0ffca1f998b183b/dist/apps/calendar.js) | 绝区零 | 复刻并保留外部依赖 | 活动日历 |
| [dist/apps/card.js](https://github.com/ZZZure/ZZZ-Plugin/blob/e35d30d52a395eae06d58f41d0ffca1f998b183b/dist/apps/card.js) | 绝区零 | 复刻 | 绳匠卡片、代理人与邦布摘要 |
| [dist/apps/climbingTower.js](https://github.com/ZZZure/ZZZ-Plugin/blob/e35d30d52a395eae06d58f41d0ffca1f998b183b/dist/apps/climbingTower.js) | 绝区零 | 复刻 | 拟真鏖战试炼各赛季、层数与战绩 |
| [dist/apps/code.js](https://github.com/ZZZure/ZZZ-Plugin/blob/e35d30d52a395eae06d58f41d0ffca1f998b183b/dist/apps/code.js) | 绝区零 | 复刻并保留外部依赖 | 兑换码查询 |
| [dist/apps/damage.js](https://github.com/ZZZure/ZZZ-Plugin/blob/e35d30d52a395eae06d58f41d0ffca1f998b183b/dist/apps/damage.js) | 绝区零 | 复刻 | 代理人技能伤害，音擎、驱动盘、队伍和条件参数 |
| [dist/apps/deadly.js](https://github.com/ZZZure/ZZZ-Plugin/blob/e35d30d52a395eae06d58f41d0ffca1f998b183b/dist/apps/deadly.js) | 绝区零 | 复刻 | 危局强袭战、历史、常规/高难成绩 |
| [dist/apps/explorationDetail.js](https://github.com/ZZZure/ZZZ-Plugin/blob/e35d30d52a395eae06d58f41d0ffca1f998b183b/dist/apps/explorationDetail.js) | 绝区零 | 复刻 | 区域探索与收集进度 |
| [dist/apps/gachalog.js](https://github.com/ZZZure/ZZZ-Plugin/blob/e35d30d52a395eae06d58f41d0ffca1f998b183b/dist/apps/gachalog.js) | 绝区零、CK | 安全替代 | 调频记录刷新、分析、同步帮助；AuthKey 和原始抽卡链接留在 CK 代理 |
| [dist/apps/guide.js](https://github.com/ZZZure/ZZZ-Plugin/blob/e35d30d52a395eae06d58f41d0ffca1f998b183b/dist/apps/guide.js) | 绝区零 | 复刻并保留外部依赖 | 代理人攻略、来源与帮助 |
| [dist/apps/help.js](https://github.com/ZZZure/ZZZ-Plugin/blob/e35d30d52a395eae06d58f41d0ffca1f998b183b/dist/apps/help.js) | 绝区零 | 原生替代 | 独立帮助、实际权限及功能开关 |
| [dist/apps/hollowZero.js](https://github.com/ZZZure/ZZZ-Plugin/blob/e35d30d52a395eae06d58f41d0ffca1f998b183b/dist/apps/hollowZero.js) | 绝区零 | 复刻 | 零号空洞、枯萎苗圃、迷失之地等源码分支与周期记录 |
| [dist/apps/holoBoss.js](https://github.com/ZZZure/ZZZ-Plugin/blob/e35d30d52a395eae06d58f41d0ffca1f998b183b/dist/apps/holoBoss.js) | 绝区零 | 复刻 | 拟境湮灭战及历史记录 |
| [dist/apps/manage.js](https://github.com/ZZZure/ZZZ-Plugin/blob/e35d30d52a395eae06d58f41d0ffca1f998b183b/dist/apps/manage.js) | 绝区零 | 原生替代 | 资源下载/清理、别名、面板图、精度、冷却、攻略来源、设备入口、群榜、更新通知和版本设置 |
| [dist/apps/monthly.js](https://github.com/ZZZure/ZZZ-Plugin/blob/e35d30d52a395eae06d58f41d0ffca1f998b183b/dist/apps/monthly.js) | 绝区零 | 复刻 | 月报、菲林/母带/邦布券明细及累计统计 |
| [dist/apps/note.js](https://github.com/ZZZure/ZZZ-Plugin/blob/e35d30d52a395eae06d58f41d0ffca1f998b183b/dist/apps/note.js) | 绝区零 | 复刻 | 电量与实时便签 |
| [dist/apps/panel.js](https://github.com/ZZZure/ZZZ-Plugin/blob/e35d30d52a395eae06d58f41d0ffca1f998b183b/dist/apps/panel.js) | 绝区零 | 复刻并保留外部依赖 | 面板列表/详情/刷新、展柜、驱动盘评分、练度、原图，动态后缀路由逐项验收 |
| [dist/apps/poolHistory.js](https://github.com/ZZZure/ZZZ-Plugin/blob/e35d30d52a395eae06d58f41d0ffca1f998b183b/dist/apps/poolHistory.js) | 绝区零 | 复刻 | 当前/全部/版本卡池，代理人及音擎复刻记录 |
| [dist/apps/rank.js](https://github.com/ZZZure/ZZZ-Plugin/blob/e35d30d52a395eae06d58f41d0ffca1f998b183b/dist/apps/rank.js) | 绝区零 | 复刻 | 防卫战/危局/拟境/临界/试炼各赛季群榜，个人显示隐藏与授权 |
| [dist/apps/remind.js](https://github.com/ZZZure/ZZZ-Plugin/blob/e35d30d52a395eae06d58f41d0ffca1f998b183b/dist/apps/remind.js) | 绝区零 | 复刻 | 个人/全局挑战提醒、阈值、每日/每周时间、即时检查和退订 |
| [dist/apps/update.js](https://github.com/ZZZure/ZZZ-Plugin/blob/e35d30d52a395eae06d58f41d0ffca1f998b183b/dist/apps/update.js) | 绝区零 | 原生替代 | 商店/artifact 更新与资源更新 |
| [dist/apps/user.js](https://github.com/ZZZure/ZZZ-Plugin/blob/e35d30d52a395eae06d58f41d0ffca1f998b183b/dist/apps/user.js) | 绝区零、CK | 分拆复刻 | 设备绑定/解绑/帮助由绝区零提供入口，凭据关联设备参数由 CK 管理 |
| [dist/apps/voidFrontBattle.js](https://github.com/ZZZure/ZZZ-Plugin/blob/e35d30d52a395eae06d58f41d0ffca1f998b183b/dist/apps/voidFrontBattle.js) | 绝区零 | 复刻 | 临界推演、历史期数与详情 |
| [dist/apps/wiki.js](https://github.com/ZZZure/ZZZ-Plugin/blob/e35d30d52a395eae06d58f41d0ffca1f998b183b/dist/apps/wiki.js) | 绝区零 | 复刻 | 技能与意象影画资料 |
| [dist/apps/zenkov.js](https://github.com/ZZZure/ZZZ-Plugin/blob/e35d30d52a395eae06d58f41d0ffca1f998b183b/dist/apps/zenkov.js) | 绝区零 | 复刻 | 迷宫诡域、总览和战绩/回顾 |

## Yunzai-genshin

| 源模块 | 归属 | 交付方式 | 功能与边界 |
| --- | --- | --- | --- |
| [apps/abbrSet.js](https://github.com/TimeRainStarSky/Yunzai-genshin/blob/4a2e1fb8f094b2a8f039ce0768773701718b60b9/apps/abbrSet.js) | 原神、星铁 | 分拆复刻 | 游戏角色别名，按源码游戏分支归属 |
| [apps/buddy.js](https://github.com/TimeRainStarSky/Yunzai-genshin/blob/4a2e1fb8f094b2a8f039ce0768773701718b60b9/apps/buddy.js) | 绝区零 | 复刻 | 邦布信息查询 |
| [apps/calculator.js](https://github.com/TimeRainStarSky/Yunzai-genshin/blob/4a2e1fb8f094b2a8f039ce0768773701718b60b9/apps/calculator.js) | 原神、星铁 | 分拆复刻 | 角色养成材料、等级/天赋计算，尘歌壶摹本相关功能归原神 |
| [apps/dailyNote.js](https://github.com/TimeRainStarSky/Yunzai-genshin/blob/4a2e1fb8f094b2a8f039ce0768773701718b60b9/apps/dailyNote.js) | 原神 | 复刻 | 树脂、派遣、洞天等便签 |
| [apps/exchange.js](https://github.com/TimeRainStarSky/Yunzai-genshin/blob/4a2e1fb8f094b2a8f039ce0768773701718b60b9/apps/exchange.js) | 原神、星铁 | 复刻并保留外部依赖 | 前瞻兑换码、兑换操作与逐账号结果，写操作单独授权 |
| [apps/gacha.js](https://github.com/TimeRainStarSky/Yunzai-genshin/blob/4a2e1fb8f094b2a8f039ce0768773701718b60b9/apps/gacha.js) | 原神 | 复刻 | 模拟抽卡、武器定轨、每日额度、卡池与保底 |
| [apps/gcLog.js](https://github.com/TimeRainStarSky/Yunzai-genshin/blob/4a2e1fb8f094b2a8f039ce0768773701718b60b9/apps/gcLog.js) | 原神、星铁 | 安全替代 | 抽卡导入/导出/同步/计数/全量拉取设置，敏感链接转安全导入流程 |
| [apps/ledger.js](https://github.com/TimeRainStarSky/Yunzai-genshin/blob/4a2e1fb8f094b2a8f039ce0768773701718b60b9/apps/ledger.js) | 原神、星铁 | 分拆复刻 | 原石札记、开拓月历、定时获取、月度和历史汇总 |
| [apps/material.js](https://github.com/TimeRainStarSky/Yunzai-genshin/blob/4a2e1fb8f094b2a8f039ce0768773701718b60b9/apps/material.js) | 原神、星铁 | 分拆复刻 | 角色培养素材及按源码支持的游戏分支查询 |
| [apps/mysNews.js](https://github.com/TimeRainStarSky/Yunzai-genshin/blob/4a2e1fb8f094b2a8f039ce0768773701718b60b9/apps/mysNews.js) | 原神、星铁、绝区零 | 分拆复刻 | 公告资讯活动、帖子与链接解析、搜索、版本预估、群订阅及活动到期推送 |
| [apps/noteZzz.js](https://github.com/TimeRainStarSky/Yunzai-genshin/blob/4a2e1fb8f094b2a8f039ce0768773701718b60b9/apps/noteZzz.js) | 绝区零 | 复刻 | 绝区零便签，合并 ZZZ 同类功能 |
| [apps/payLog.js](https://github.com/TimeRainStarSky/Yunzai-genshin/blob/4a2e1fb8f094b2a8f039ce0768773701718b60b9/apps/payLog.js) | 原神、星铁 | 安全替代 | 充值/消费记录、统计、刷新；AuthKey 由 CK 代理保管，不回传原始凭据 |
| [apps/role.js](https://github.com/TimeRainStarSky/Yunzai-genshin/blob/4a2e1fb8f094b2a8f039ce0768773701718b60b9/apps/role.js) | 原神 | 复刻 | 角色卡片、武器、探索、深渊总览/分层、幻想真境剧诗 |
| [apps/setPubCk.js](https://github.com/TimeRainStarSky/Yunzai-genshin/blob/4a2e1fb8f094b2a8f039ce0768773701718b60b9/apps/setPubCk.js) | CK | 安全替代 | 公共查询账号池、启停与预算，需账号所有者主动授权，禁止默认共享私人 CK |
| [apps/sevenSaints.js](https://github.com/TimeRainStarSky/Yunzai-genshin/blob/4a2e1fb8f094b2a8f039ce0768773701718b60b9/apps/sevenSaints.js) | 原神 | 复刻 | 七圣召唤牌组和卡牌信息 |
| [apps/strategy.js](https://github.com/TimeRainStarSky/Yunzai-genshin/blob/4a2e1fb8f094b2a8f039ce0768773701718b60b9/apps/strategy.js) | 原神、星铁 | 分拆复刻 | 攻略查询、来源设置与帮助 |
| [apps/takeBirthdayPhoto.js](https://github.com/TimeRainStarSky/Yunzai-genshin/blob/4a2e1fb8f094b2a8f039ce0768773701718b60b9/apps/takeBirthdayPhoto.js) | 原神 | 复刻 | 留影叙佳期、生日内容 |
| [apps/user.js](https://github.com/TimeRainStarSky/Yunzai-genshin/blob/4a2e1fb8f094b2a8f039ce0768773701718b60b9/apps/user.js) | CK | 安全替代 | 多账号/UID、默认角色、主子用户关联、状态、解绑；我的CK改摘要，手工明文绑定改扫码/加密导入 |
| [apps/userAdmin.js](https://github.com/TimeRainStarSky/Yunzai-genshin/blob/4a2e1fb8f094b2a8f039ce0768773701718b60b9/apps/userAdmin.js) | CK | 复刻 | 用户统计、刷新缓存、检查失效；与 SToken 换取新 CK 明确区分 |

## ark-plugin

| 源模块 | 归属 | 交付方式 | 功能与边界 |
| --- | --- | --- | --- |
| [apps/admin.js](https://github.com/NotIvny/ark-plugin/blob/b7d8231ddcf606ce4f8d20e9bd743970e4f18cea/apps/admin.js) | 原神、星铁 | 分拆复刻 | 增强项开关及配置，各游戏独立管理 |
| [apps/customRank.js](https://github.com/NotIvny/ark-plugin/blob/b7d8231ddcf606ce4f8d20e9bd743970e4f18cea/apps/customRank.js) | 原神、星铁 | 复刻并保留外部依赖 | 自定义伤害/圣遗物排行、筛选语法、数值单位、条件组合与配额 |
| [apps/customRankPanel.js](https://github.com/NotIvny/ark-plugin/blob/b7d8231ddcf606ce4f8d20e9bd743970e4f18cea/apps/customRankPanel.js) | 原神、星铁 | 复刻并保留外部依赖 | 自定义榜单前二十名面板展示，承接筛选会话 |
| [apps/help.js](https://github.com/NotIvny/ark-plugin/blob/b7d8231ddcf606ce4f8d20e9bd743970e4f18cea/apps/help.js) | 原神、星铁 | 原生替代 | 增强帮助与版本并入游戏帮助 |
| [apps/HelpTheme.js](https://github.com/NotIvny/ark-plugin/blob/b7d8231ddcf606ce4f8d20e9bd743970e4f18cea/apps/HelpTheme.js) | 原神、星铁 | 原生替代 | 帮助主题、资源、布局设置 |
| [apps/miaoGroupConfig.js](https://github.com/NotIvny/ark-plugin/blob/b7d8231ddcf606ce4f8d20e9bd743970e4f18cea/apps/miaoGroupConfig.js) | 原神、星铁 | 分拆复刻 | 独立群设置、覆盖优先级与群权限 |
| [apps/miaoHelp.js](https://github.com/NotIvny/ark-plugin/blob/b7d8231ddcf606ce4f8d20e9bd743970e4f18cea/apps/miaoHelp.js) | 原神、星铁 | 原生替代 | 面板帮助扩展内建，保留信息与使用指引 |
| [apps/priority.js](https://github.com/NotIvny/ark-plugin/blob/b7d8231ddcf606ce4f8d20e9bd743970e4f18cea/apps/priority.js) | 原神、星铁 | 原生替代（已确认） | 冲突诊断和兼容别名开关，不改其他插件文件/加载器；原全局优先级功能映射为游戏内命令路由与冲突设置 |
| [apps/replaceFile.js](https://github.com/NotIvny/ark-plugin/blob/b7d8231ddcf606ce4f8d20e9bd743970e4f18cea/apps/replaceFile.js) | 原神、星铁 | 原生替代（已确认） | 游戏内增强开关、配置/非敏感业务备份导入导出；原任意文件替换恢复不属于四插件原生接口 |
| [apps/token.js](https://github.com/NotIvny/ark-plugin/blob/b7d8231ddcf606ce4f8d20e9bd743970e4f18cea/apps/token.js) | 原神、星铁 | 安全输入替代 | 外部 ark API 凭据配置，不通过群命令携带；不把该 token 当米游社 CK |
| [apps/update.js](https://github.com/NotIvny/ark-plugin/blob/b7d8231ddcf606ce4f8d20e9bd743970e4f18cea/apps/update.js) | 原神、星铁 | 原生替代（已确认） | 独立发行版本/渠道，替代上游 Git 分支列表与切换 |
| [apps/usage.js](https://github.com/NotIvny/ark-plugin/blob/b7d8231ddcf606ce4f8d20e9bd743970e4f18cea/apps/usage.js) | 原神、星铁 | 复刻并保留外部依赖 | API token 配额与用量 |
| [apps/user.js](https://github.com/NotIvny/ark-plugin/blob/b7d8231ddcf606ce4f8d20e9bd743970e4f18cea/apps/user.js) | 原神、星铁 | 复刻并保留外部依赖 | 全服/百分位排名、幽境榜、UID签名验证、云面板十分钟交换、历史对比、OCR重塑重投识别、面板导出 |

## Axiu-Plugin

| 源模块 | 归属 | 交付方式 | 功能与边界 |
| --- | --- | --- | --- |
| [apps/autoGroupApprove.js](https://github.com/AxiuCN/Axiu-Plugin/blob/6f336229dce0a22344e37107a738717d0860f6b7/apps/autoGroupApprove.js) | 范围待定 | 补充参考的非游戏功能 | 入群审批与三游戏/CK目标无直接关系，保留清单供范围确认 |
| [apps/captchaHandler.js](https://github.com/AxiuCN/Axiu-Plugin/blob/6f336229dce0a22344e37107a738717d0860f6b7/apps/captchaHandler.js) | 游戏入口、CK会话 | 复刻并保留外部依赖 | 验证码状态、官方人工完成流程；参考自动服务依赖、隐私与不可用回退单独验收 |
| [apps/challenge.js](https://github.com/AxiuCN/Axiu-Plugin/blob/6f336229dce0a22344e37107a738717d0860f6b7/apps/challenge.js) | 原神、星铁 | 分拆复刻 | 终局查询与群排行，榜单重建/重置/管理和历史数据 |
| [apps/gsGachaLog.js](https://github.com/AxiuCN/Axiu-Plugin/blob/6f336229dce0a22344e37107a738717d0860f6b7/apps/gsGachaLog.js) | 原神、CK | 复刻并保留外部依赖 | 官方抽卡同步、小助手全历史数据接入；凭据链接走代理，第三方不得获得米游社 CK |
| [apps/help.js](https://github.com/AxiuCN/Axiu-Plugin/blob/6f336229dce0a22344e37107a738717d0860f6b7/apps/help.js) | 原神、星铁、CK | 原生替代 | 相关帮助分别内建 |
| [apps/mysSignin.js](https://github.com/AxiuCN/Axiu-Plugin/blob/6f336229dce0a22344e37107a738717d0860f6b7/apps/mysSignin.js) | 原神、星铁、绝区零、CK | 分拆复刻 | 游戏/社区/云游戏签到及名单、开关、周期、批量报告；凭据生命周期归CK，签到策略归各游戏 |
| [apps/proxySpeak.js](https://github.com/AxiuCN/Axiu-Plugin/blob/6f336229dce0a22344e37107a738717d0860f6b7/apps/proxySpeak.js) | 范围待定 | 补充参考的非游戏功能 | 代发言会涉及跨用户身份，不能用于绕过 CK 所有者校验 |
| [apps/qrLogin.js](https://github.com/AxiuCN/Axiu-Plugin/blob/6f336229dce0a22344e37107a738717d0860f6b7/apps/qrLogin.js) | CK | 安全重写 | 米游社扫码、SToken换取CK、刷新，多游戏发现与原子保存 |
| [apps/srGachaLog.js](https://github.com/AxiuCN/Axiu-Plugin/blob/6f336229dce0a22344e37107a738717d0860f6b7/apps/srGachaLog.js) | 星铁、CK | 安全替代 | 星铁抽卡同步与数据导出，原始 AuthKey 链接不回传 |

## xiaoyao-cvs-plugin

| 源模块 | 归属 | 交付方式 | 功能与边界 |
| --- | --- | --- | --- |
| [apps/admin.js](https://github.com/ctrlcvs/xiaoyao-cvs-plugin/blob/e7ab3e8b276a11680beb47d0c5517f6e0a4c2022/apps/admin.js) | 原神、星铁 | 原生替代 | 图鉴和模板资源更新、游戏内配置 |
| [apps/help.js](https://github.com/ctrlcvs/xiaoyao-cvs-plugin/blob/e7ab3e8b276a11680beb47d0c5517f6e0a4c2022/apps/help.js) | 原神、星铁、CK | 原生替代 | 图鉴、账号功能帮助及版本 |
| [apps/index.js](https://github.com/ctrlcvs/xiaoyao-cvs-plugin/blob/e7ab3e8b276a11680beb47d0c5517f6e0a4c2022/apps/index.js) | 原神、星铁、CK | 分拆复刻 | 动态规则整合、原神/星铁图鉴、体力模板、戳一戳、动态内容及定时任务分别落入所属游戏 |
| [apps/map.js](https://github.com/ctrlcvs/xiaoyao-cvs-plugin/blob/e7ab3e8b276a11680beb47d0c5517f6e0a4c2022/apps/map.js) | 原神 | 复刻并保留外部依赖 | 地图物品定位、地图缓存刷新/清理 |
| [apps/mhyTopUpLogin.js](https://github.com/ctrlcvs/xiaoyao-cvs-plugin/blob/e7ab3e8b276a11680beb47d0c5517f6e0a4c2022/apps/mhyTopUpLogin.js) | CK、原神 | 分拆与范围待定 | 扫码归CK；聊天密码登录与明文凭据不保留；充值商品、订单、支付流程列为待确认的额外业务 |
| [apps/Note.js](https://github.com/ctrlcvs/xiaoyao-cvs-plugin/blob/e7ab3e8b276a11680beb47d0c5517f6e0a4c2022/apps/Note.js) | 原神 | 复刻 | 多账号树脂便签、推送阈值、群设置、模板与提醒去重 |
| [apps/sign.js](https://github.com/ctrlcvs/xiaoyao-cvs-plugin/blob/e7ab3e8b276a11680beb47d0c5517f6e0a4c2022/apps/sign.js) | 三游戏 | 分拆复刻 | 原神/星铁/绝区零社区与游戏签到、米游币查询，云原神签到归原神；其他游戏分支范围待定 |
| [apps/srGallery.js](https://github.com/ctrlcvs/xiaoyao-cvs-plugin/blob/e7ab3e8b276a11680beb47d0c5517f6e0a4c2022/apps/srGallery.js) | 星铁 | 复刻并保留资源依赖 | 角色/光锥等图鉴、别名和素材 |
| [apps/user.js](https://github.com/ctrlcvs/xiaoyao-cvs-plugin/blob/e7ab3e8b276a11680beb47d0c5517f6e0a4c2022/apps/user.js) | CK、原神、星铁 | 分拆与安全替代 | 账号状态、刷新、删除归CK；抽卡/充值记录归游戏；云原神凭据由CK保管 |
| [apps/xiaoyao_image.js](https://github.com/ctrlcvs/xiaoyao-cvs-plugin/blob/e7ab3e8b276a11680beb47d0c5517f6e0a4c2022/apps/xiaoyao_image.js) | 原神 | 复刻并保留资源依赖 | 角色/武器/食物/敌人/七圣召唤图鉴、别名、动态内容 |

## Atlas

| 源模块 | 归属 | 交付方式 | 功能与边界 |
| --- | --- | --- | --- |
| [apps/admin.js](https://github.com/Nwflower/Atlas/blob/016e49357666e0823791abdf28fbb3b2efe68225/apps/admin.js) | 原神、星铁、绝区零 | 原生替代 | 图鉴库安装/更新、版本与资源索引，不执行远程代码更新 |
| [apps/atlas.js](https://github.com/Nwflower/Atlas/blob/016e49357666e0823791abdf28fbb3b2efe68225/apps/atlas.js) | 原神、星铁、绝区零 | 分拆复刻 | 动态图库规则、别名、分页与多结果按游戏拆分，依赖图库资源包 |
| [apps/atlasHelp.js](https://github.com/Nwflower/Atlas/blob/016e49357666e0823791abdf28fbb3b2efe68225/apps/atlasHelp.js) | 三游戏 | 原生替代 | 实际可用图库的帮助 |
| [apps/EnemyValue.js](https://github.com/Nwflower/Atlas/blob/016e49357666e0823791abdf28fbb3b2efe68225/apps/EnemyValue.js) | 原神 | 复刻 | 原魔生命值与攻击力计算 |

## 应用目录之外的必查面

| 范围 | 实施与验收要求 |
| --- | --- |
| miao `models/`、`resources/meta-*`、面板转换器 | 角色、武器/光锥、天赋/行迹、命座/星魂、套装、增益、评分权重、公式、队伍与敌人参数逐数据版本建立向量。不得只实现几个示例角色。 |
| miao `components/App.js`、角色 `check`、`yzRule` | 隐式角色名、原神/星铁前缀预处理、戳一戳、旧 Yunzai 命令的条件注册均有验收案例。 |
| StarRail `utils/`、`runtime/`、`common/` | 数据源转换、请求签名、角色模型、伤害细节、日期和缓存行为；不搬运运行时全局对象。 |
| ZZZ `dist/lib/`、`dist/utils/`、`resources/map/`、开发分支源码 | `panel.handleRule` 动态命令、评分/伤害、试炼赛季、资源与 API 适配；main 的编译代码为本次功能快照，dev 作为源码补充单独固定提交。 |
| Yunzai `model/mys/`、Miao-Yunzai `plugins/genshin/model/mys/` | NoteUser → 多 MysUser → 多游戏角色及默认 UID 的关联、公共查询池、`onlySelfCk` 与查询限额；目标改为无 CK 输出的代理。 |
| ark `model/`、`defSet/`、`resources/` | miao 文件替换注入、全服榜、OCR、云端 API、群配置、备份清单、幽境初始化、排名和面板模板。 |
| 逍遥 `adapter/`、`apps/index.js`、`model/` | 导出函数名驱动的动态命令、定时签到/便签、密码/扫码登录分支、地图与图鉴规则。 |
| Axiu `model/`、`tool/`、`.gitmodules` | QR 与 Passport 端点、周期刷新、MihoyoBBSTools 签到引擎及验证码子模块；不得让 Python/JS 辅助程序接收明文 CK 文件或命令行参数。 |
| Atlas `model/moreLib.js`、`resources/Forlibrary/` | 图库是外部资源库，主仓库下载完成不等于全部图鉴素材已下载。 |
| 各仓库 config、defSet、模板、README、CHANGELOG | 每个可见开关、定时任务、分页、图片/表格输出、数据来源、权限与失败状态纳入测试台账；帮助提到宿主或其他插件的功能，先追踪真实入口。 |

## 每项功能的完成标准

后续把每个模块拆到可观察功能用例，台账至少记录：固定来源提交和函数、旧命令与别名、目标命令、用户/群/管理员权限、账号与角色范围、公开/私有数据来源、代理 operation、分页、缓存、调度、数据与模板版本、错误语义、差异说明、验收证据。

所有必选项都需有结果，外部依赖未满足的项保持阻塞状态。原生和安全替代采用已确认的 RayleaBot 原生方式；补充参考的额外业务仍单独登记。机器矩阵的 native_implementation_available 表示已有原生实现，不等同于全部账号、外部服务和系统平台都已验收；具体范围以交付验收和依赖登记为准。
