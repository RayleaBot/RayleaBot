# 命令与入口静态清单

参考快照：2026-09-15。文档更新：2026-09-18。

目标架构采用宿主通用插件服务调用；本清单记录上游命令基线，不定义宿主协议。目标归属与替代方式见 [功能矩阵](./functional-parity.md)。

从固定提交源码提取 374 个静态命令或事件处理声明，本表保存探索时的源码入口，不作为当前实现状态；当前完成范围见执行进度与功能矩阵。动态命令、通配入口、accept/init/task 钩子与模板变体需结合功能矩阵验收。原始表达式、兼容规则和函数位置见 [机器清单](./data/command-inventory.json)。表内表达式按代码显示，源码链接定位到固定提交的对应行。

## miao-plugin

固定提交：`7f6f1c84c89102bc6b1c58c8e1b06c61f4642161`。静态声明：63。

| 源文件与行 | 标签或处理函数 | 规则表达式 | 参考权限 |
| --- | --- | --- | --- |
| [apps/admin.js:20](https://github.com/yoimiya-kokomi/miao-plugin/blob/7f6f1c84c89102bc6b1c58c8e1b06c61f4642161/apps/admin.js#L20) | ` '【#管理】更新素材' ` | ` /^#喵喵(强制)?(更新图像\|图像更新)$/ ` | 由上层或处理函数决定 |
| [apps/admin.js:25](https://github.com/yoimiya-kokomi/miao-plugin/blob/7f6f1c84c89102bc6b1c58c8e1b06c61f4642161/apps/admin.js#L25) | ` '【#管理】喵喵更新' ` | ` /^#喵喵(强制)?更新$/ ` | 由上层或处理函数决定 |
| [apps/admin.js:30](https://github.com/yoimiya-kokomi/miao-plugin/blob/7f6f1c84c89102bc6b1c58c8e1b06c61f4642161/apps/admin.js#L30) | ` '【#管理】喵喵更新' ` | ` /^#?喵喵更新日志$/ ` | 由上层或处理函数决定 |
| [apps/admin.js:35](https://github.com/yoimiya-kokomi/miao-plugin/blob/7f6f1c84c89102bc6b1c58c8e1b06c61f4642161/apps/admin.js#L35) | ` '【#管理】系统设置' ` | ` sysCfgReg ` | 由上层或处理函数决定 |
| [apps/admin.js:40](https://github.com/yoimiya-kokomi/miao-plugin/blob/7f6f1c84c89102bc6b1c58c8e1b06c61f4642161/apps/admin.js#L40) | ` '【#管理】喵喵Api' ` | ` /^#喵喵api$/ ` | 由上层或处理函数决定 |
| [apps/alias.js:25](https://github.com/yoimiya-kokomi/miao-plugin/blob/7f6f1c84c89102bc6b1c58c8e1b06c61f4642161/apps/alias.js#L25) | ` '【#别名】 喵喵别名帮助' ` | ` /^[#*](?:星铁)?喵喵别名(帮助)?$/ ` | 由上层或处理函数决定 |
| [apps/alias.js:30](https://github.com/yoimiya-kokomi/miao-plugin/blob/7f6f1c84c89102bc6b1c58c8e1b06c61f4642161/apps/alias.js#L30) | ` '【#别名】 #喵喵别名原神设置 角色名 别名' ` | ` /^[#*](?:星铁)?喵喵别名原神设置\s+\S+\s+\S+$/ ` | 由上层或处理函数决定 |
| [apps/alias.js:35](https://github.com/yoimiya-kokomi/miao-plugin/blob/7f6f1c84c89102bc6b1c58c8e1b06c61f4642161/apps/alias.js#L35) | ` '【#别名】 #喵喵别名星铁设置 角色名 别名' ` | ` /^[#*](?:星铁)?喵喵别名[崩星]铁设置\s+\S+\s+\S+$/ ` | 由上层或处理函数决定 |
| [apps/alias.js:40](https://github.com/yoimiya-kokomi/miao-plugin/blob/7f6f1c84c89102bc6b1c58c8e1b06c61f4642161/apps/alias.js#L40) | ` '【#别名】 #喵喵别名设置 角色名 别名' ` | ` /^[#*](?:星铁)?喵喵别名设置\s+\S+\s+\S+$/ ` | 由上层或处理函数决定 |
| [apps/alias.js:45](https://github.com/yoimiya-kokomi/miao-plugin/blob/7f6f1c84c89102bc6b1c58c8e1b06c61f4642161/apps/alias.js#L45) | ` '【#别名】 #喵喵别名原神删除 别名' ` | ` /^[#*](?:星铁)?喵喵别名原神删除\s+\S+$/ ` | 由上层或处理函数决定 |
| [apps/alias.js:50](https://github.com/yoimiya-kokomi/miao-plugin/blob/7f6f1c84c89102bc6b1c58c8e1b06c61f4642161/apps/alias.js#L50) | ` '【#别名】 #喵喵别名星铁删除 别名' ` | ` /^[#*](?:星铁)?喵喵别名[崩星]铁删除\s+\S+$/ ` | 由上层或处理函数决定 |
| [apps/alias.js:55](https://github.com/yoimiya-kokomi/miao-plugin/blob/7f6f1c84c89102bc6b1c58c8e1b06c61f4642161/apps/alias.js#L55) | ` '【#别名】 #喵喵别名删除 别名' ` | ` /^[#*](?:星铁)?喵喵别名删除\s+\S+$/ ` | 由上层或处理函数决定 |
| [apps/alias.js:60](https://github.com/yoimiya-kokomi/miao-plugin/blob/7f6f1c84c89102bc6b1c58c8e1b06c61f4642161/apps/alias.js#L60) | ` '【#别名】 #喵喵别名列表 查看全部自定义别名' ` | ` /^[#*](?:星铁)?喵喵别名列表$/ ` | 由上层或处理函数决定 |
| [apps/alias.js:65](https://github.com/yoimiya-kokomi/miao-plugin/blob/7f6f1c84c89102bc6b1c58c8e1b06c61f4642161/apps/alias.js#L65) | ` '【#别名】 #喵喵别名原神列表 查看原神自定义别名' ` | ` /^[#*](?:星铁)?喵喵别名原神列表$/ ` | 由上层或处理函数决定 |
| [apps/alias.js:70](https://github.com/yoimiya-kokomi/miao-plugin/blob/7f6f1c84c89102bc6b1c58c8e1b06c61f4642161/apps/alias.js#L70) | ` '【#别名】 #喵喵别名星铁列表 查看星铁自定义别名' ` | ` /^[#*](?:星铁)?喵喵别名[崩星]铁列表$/ ` | 由上层或处理函数决定 |
| [apps/character.js:13](https://github.com/yoimiya-kokomi/miao-plugin/blob/7f6f1c84c89102bc6b1c58c8e1b06c61f4642161/apps/character.js#L13) | ` '角色卡片' ` | ` /^#喵喵角色卡片$/ ` | 由上层或处理函数决定 |
| [apps/character.js:19](https://github.com/yoimiya-kokomi/miao-plugin/blob/7f6f1c84c89102bc6b1c58c8e1b06c61f4642161/apps/character.js#L19) | ` '上传角色写真' ` | ` /^#*(喵喵)?(上传\|添加)(.+)(照片\|写真\|图片\|图像)\s*$/ ` | 由上层或处理函数决定 |
| [apps/character.js:24](https://github.com/yoimiya-kokomi/miao-plugin/blob/7f6f1c84c89102bc6b1c58c8e1b06c61f4642161/apps/character.js#L24) | ` '#老公 #老婆 查询' ` | ` Wife.reg ` | 由上层或处理函数决定 |
| [apps/character.js:29](https://github.com/yoimiya-kokomi/miao-plugin/blob/7f6f1c84c89102bc6b1c58c8e1b06c61f4642161/apps/character.js#L29) | ` '【#原图】 回复角色卡片，可获取原图' ` | ` /^#?(获取\|给我\|我要\|求\|发\|发下\|发个\|发一下)?原图(吧\|呗)?$/ ` | 由上层或处理函数决定 |
| [apps/gacha.js:9](https://github.com/yoimiya-kokomi/miao-plugin/blob/7f6f1c84c89102bc6b1c58c8e1b06c61f4642161/apps/gacha.js#L9) | ` '抽卡记录' ` | ` /^#*(星铁)?喵喵(抽卡\|抽奖\|角色\|武器\|光锥\|常驻\|集录\|up)+池?(记录\|祈愿\|分析)$/ ` | 由上层或处理函数决定 |
| [apps/gacha.js:16](https://github.com/yoimiya-kokomi/miao-plugin/blob/7f6f1c84c89102bc6b1c58c8e1b06c61f4642161/apps/gacha.js#L16) | ` '抽卡统计' ` | ` /^#*(星铁)?喵喵(全部\|抽卡\|抽奖\|角色\|武器\|光锥\|常驻\|集录\|up\|版本)+池?统计$/ ` | 由上层或处理函数决定 |
| [apps/gacha.js:23](https://github.com/yoimiya-kokomi/miao-plugin/blob/7f6f1c84c89102bc6b1c58c8e1b06c61f4642161/apps/gacha.js#L23) | ` '卡池信息' ` | ` /^#(星铁)?((?:\d+\.)+\d+)(上半\|下半)?卡池$/ ` | 由上层或处理函数决定 |
| [apps/gacha.js:28](https://github.com/yoimiya-kokomi/miao-plugin/blob/7f6f1c84c89102bc6b1c58c8e1b06c61f4642161/apps/gacha.js#L28) | ` '卡池查询帮助' ` | ` /^#(星铁)?卡池(帮助)?$/ ` | 由上层或处理函数决定 |
| [apps/gacha.js:33](https://github.com/yoimiya-kokomi/miao-plugin/blob/7f6f1c84c89102bc6b1c58c8e1b06c61f4642161/apps/gacha.js#L33) | ` '卡池角色/武器查询' ` | ` /^#(星铁)?(.+?)卡池(详情\|详细)?$/ ` | 由上层或处理函数决定 |
| [apps/help.js:11](https://github.com/yoimiya-kokomi/miao-plugin/blob/7f6f1c84c89102bc6b1c58c8e1b06c61f4642161/apps/help.js#L11) | ` '【#/帮助】 #/喵喵帮助' ` | ` /^(\/\|#)?(喵喵)?(命令\|帮助\|菜单\|help\|说明\|功能\|指令\|使用说明)$/ ` | 由上层或处理函数决定 |
| [apps/help.js:16](https://github.com/yoimiya-kokomi/miao-plugin/blob/7f6f1c84c89102bc6b1c58c8e1b06c61f4642161/apps/help.js#L16) | ` '【#/帮助】 #/喵喵版本介绍' ` | ` /^(\/\|#)?喵喵版本$/ ` | 由上层或处理函数决定 |
| [apps/poke.js:11](https://github.com/yoimiya-kokomi/miao-plugin/blob/7f6f1c84c89102bc6b1c58c8e1b06c61f4642161/apps/poke.js#L11) | ` '#老公 #老婆 查询' ` | ` (event handler; App defaults to catch-all) ` | 由上层或处理函数决定 |
| [apps/profile.js:17](https://github.com/yoimiya-kokomi/miao-plugin/blob/7f6f1c84c89102bc6b1c58c8e1b06c61f4642161/apps/profile.js#L17) | ` '面板角色列表' ` | ` /^#(星铁\|原神)?(面板角色\|角色面板\|面板)(列表)?\s*(\d{9,10})?$/ ` | 由上层或处理函数决定 |
| [apps/profile.js:24](https://github.com/yoimiya-kokomi/miao-plugin/blob/7f6f1c84c89102bc6b1c58c8e1b06c61f4642161/apps/profile.js#L24) | ` '角色面板' ` | ` /^#*([^#]+)\s*(详细\|详情\|面板\|面版\|圣遗物\|遗器\|武器[1-7]?\|伤害([1-9]+\d*)?)\s*(\d{9,10})*(.*[换变改].*)?$/ ` | 由上层或处理函数决定 |
| [apps/profile.js:30](https://github.com/yoimiya-kokomi/miao-plugin/blob/7f6f1c84c89102bc6b1c58c8e1b06c61f4642161/apps/profile.js#L30) | ` '角色面板计算' ` | ` /^#.+换.+$/ ` | 由上层或处理函数决定 |
| [apps/profile.js:36](https://github.com/yoimiya-kokomi/miao-plugin/blob/7f6f1c84c89102bc6b1c58c8e1b06c61f4642161/apps/profile.js#L36) | ` '群内最强' ` | ` /^#(星铁\|原神)?(群\|群内)?(排名\|排行)?(最强\|最高\|最高分\|最牛\|第一\|极限)+.+/ ` | 由上层或处理函数决定 |
| [apps/profile.js:42](https://github.com/yoimiya-kokomi/miao-plugin/blob/7f6f1c84c89102bc6b1c58c8e1b06c61f4642161/apps/profile.js#L42) | ` '重置排名' ` | ` /^#(星铁\|原神)?(重置\|重设)(.*)(排名\|排行)$/ ` | 由上层或处理函数决定 |
| [apps/profile.js:48](https://github.com/yoimiya-kokomi/miao-plugin/blob/7f6f1c84c89102bc6b1c58c8e1b06c61f4642161/apps/profile.js#L48) | ` '刷新排名' ` | ` /^#(星铁\|原神)?(刷新\|更新\|重新加载)(群内\|群\|全部)*(排名\|排行)$/ ` | 由上层或处理函数决定 |
| [apps/profile.js:54](https://github.com/yoimiya-kokomi/miao-plugin/blob/7f6f1c84c89102bc6b1c58c8e1b06c61f4642161/apps/profile.js#L54) | ` '打开关闭' ` | ` /^#(开启\|打开\|启用\|关闭\|禁用)(群内\|群\|全部)*(排名\|排行)$/ ` | 由上层或处理函数决定 |
| [apps/profile.js:60](https://github.com/yoimiya-kokomi/miao-plugin/blob/7f6f1c84c89102bc6b1c58c8e1b06c61f4642161/apps/profile.js#L60) | ` '面板排名榜' ` | ` /^#(星铁\|原神)?(群\|群内)?.+(排名\|排行)(榜)?$/ ` | 由上层或处理函数决定 |
| [apps/profile.js:66](https://github.com/yoimiya-kokomi/miao-plugin/blob/7f6f1c84c89102bc6b1c58c8e1b06c61f4642161/apps/profile.js#L66) | ` '面板圣遗物列表' ` | ` /^#(星铁\|原神)?(圣遗物\|遗器)列表\s*(\d{9,10})?$/ ` | 由上层或处理函数决定 |
| [apps/profile.js:72](https://github.com/yoimiya-kokomi/miao-plugin/blob/7f6f1c84c89102bc6b1c58c8e1b06c61f4642161/apps/profile.js#L72) | ` '面板练度统计' ` | ` /^#(星铁\|原神)?(面板\|喵喵)?练度统计$/ ` | 由上层或处理函数决定 |
| [apps/profile.js:80](https://github.com/yoimiya-kokomi/miao-plugin/blob/7f6f1c84c89102bc6b1c58c8e1b06c61f4642161/apps/profile.js#L80) | ` '幻想真境剧诗入门角色统计' ` | ` /^#202\d{3}(幻想\|真境\|剧诗\|幻想真境剧诗)练度统计$/ ` | 由上层或处理函数决定 |
| [apps/profile.js:88](https://github.com/yoimiya-kokomi/miao-plugin/blob/7f6f1c84c89102bc6b1c58c8e1b06c61f4642161/apps/profile.js#L88) | ` '天赋统计' ` | ` /^#*(我的)?(今日\|今天\|明日\|明天\|周.*)?([五四54]星)?(技能\|天赋)+(汇总\|统计\|列表)?[ \|0-9]*$/ ` | 由上层或处理函数决定 |
| [apps/profile.js:94](https://github.com/yoimiya-kokomi/miao-plugin/blob/7f6f1c84c89102bc6b1c58c8e1b06c61f4642161/apps/profile.js#L94) | ` '角色查询' ` | ` /^#喵喵(角色\|查询)[ \|0-9]*$/ ` | 由上层或处理函数决定 |
| [apps/profile.js:102](https://github.com/yoimiya-kokomi/miao-plugin/blob/7f6f1c84c89102bc6b1c58c8e1b06c61f4642161/apps/profile.js#L102) | ` '强制刷新天赋' ` | ` /^#(星铁\|原神)?(强制)?(刷新\|更新)(所有\|角色)*(天赋\|技能\|行迹)$/ ` | 由上层或处理函数决定 |
| [apps/profile.js:108](https://github.com/yoimiya-kokomi/miao-plugin/blob/7f6f1c84c89102bc6b1c58c8e1b06c61f4642161/apps/profile.js#L108) | ` '角色面板帮助' ` | ` /^#(角色\|换\|更换)?面[板版]帮助$/ ` | 由上层或处理函数决定 |
| [apps/profile.js:114](https://github.com/yoimiya-kokomi/miao-plugin/blob/7f6f1c84c89102bc6b1c58c8e1b06c61f4642161/apps/profile.js#L114) | ` '敌人等级' ` | ` /^#(敌人\|怪物)等级\s*\d{1,3}\s*$/ ` | 由上层或处理函数决定 |
| [apps/profile.js:121](https://github.com/yoimiya-kokomi/miao-plugin/blob/7f6f1c84c89102bc6b1c58c8e1b06c61f4642161/apps/profile.js#L121) | ` '面板更新' ` | ` /^#(星铁\|原神)?(全部面板更新\|更新全部面板\|获取游戏角色详情\|更新面板\|面板更新)\s*(\d{9,10})?$/ ` | 由上层或处理函数决定 |
| [apps/profile.js:128](https://github.com/yoimiya-kokomi/miao-plugin/blob/7f6f1c84c89102bc6b1c58c8e1b06c61f4642161/apps/profile.js#L128) | ` '米游社面板更新' ` | ` /^#(星铁\|原神)?(米游社\|mys)(全部面板更新\|更新全部面板\|获取游戏角色详情\|更新面板\|面板更新)(\d{9,10})?$/ ` | 由上层或处理函数决定 |
| [apps/profile.js:135](https://github.com/yoimiya-kokomi/miao-plugin/blob/7f6f1c84c89102bc6b1c58c8e1b06c61f4642161/apps/profile.js#L135) | ` '上传面板图' ` | ` /^#?\s*(?:上传\|添加)(.+)(?:面板图)\s*$/ ` | 由上层或处理函数决定 |
| [apps/profile.js:142](https://github.com/yoimiya-kokomi/miao-plugin/blob/7f6f1c84c89102bc6b1c58c8e1b06c61f4642161/apps/profile.js#L142) | ` '删除面板图' ` | ` /^#?\s*(?:移除\|清除\|删除)(.+)(?:面板图)(\d){1,}\s*$/ ` | 由上层或处理函数决定 |
| [apps/profile.js:149](https://github.com/yoimiya-kokomi/miao-plugin/blob/7f6f1c84c89102bc6b1c58c8e1b06c61f4642161/apps/profile.js#L149) | ` '面板图列表' ` | ` /^#?\s*(.+)(?:面板图列表)\s*$/ ` | 由上层或处理函数决定 |
| [apps/profile.js:156](https://github.com/yoimiya-kokomi/miao-plugin/blob/7f6f1c84c89102bc6b1c58c8e1b06c61f4642161/apps/profile.js#L156) | ` '删除面板' ` | ` /^#(星铁\|原神)?(删除全部面板\|删除面板\|删除面板数据)\s*(\d{9,10})?$/ ` | 由上层或处理函数决定 |
| [apps/profile.js:163](https://github.com/yoimiya-kokomi/miao-plugin/blob/7f6f1c84c89102bc6b1c58c8e1b06c61f4642161/apps/profile.js#L163) | ` '重新加载面板' ` | ` /^#(星铁\|原神)?(加载\|重新加载\|重载)面板\s*(\d{9,10})?$/ ` | 由上层或处理函数决定 |
| [apps/stat.js:19](https://github.com/yoimiya-kokomi/miao-plugin/blob/7f6f1c84c89102bc6b1c58c8e1b06c61f4642161/apps/stat.js#L19) | ` '【#统计】 #角色持有率 #角色5命统计' ` | ` /^#(喵喵)?角色(持有\|持有率\|命座\|命之座\|.命)(分布\|统计\|持有\|持有率)?$/ ` | 由上层或处理函数决定 |
| [apps/stat.js:24](https://github.com/yoimiya-kokomi/miao-plugin/blob/7f6f1c84c89102bc6b1c58c8e1b06c61f4642161/apps/stat.js#L24) | ` '【#统计】 #深渊出场率 #深渊12层出场率' ` | ` /^#(喵喵)?(深渊\|幽境\|危战\|幽境危战)(第?.{1,2}层)?(角色)?(出场\|使用)(率\|统计)*$/ ` | 由上层或处理函数决定 |
| [apps/stat.js:29](https://github.com/yoimiya-kokomi/miao-plugin/blob/7f6f1c84c89102bc6b1c58c8e1b06c61f4642161/apps/stat.js#L29) | ` '【#角色】 #深渊组队' ` | ` /^#深渊(组队\|配队\|配对)$/ ` | 由上层或处理函数决定 |
| [apps/stat.js:34](https://github.com/yoimiya-kokomi/miao-plugin/blob/7f6f1c84c89102bc6b1c58c8e1b06c61f4642161/apps/stat.js#L34) | ` '上传深渊' ` | ` /^#*(喵喵\|上传\|本期)*(深渊\|深境\|深境螺旋)[ \|0-9]*(数据)?$/ ` | 由上层或处理函数决定 |
| [apps/stat.js:39](https://github.com/yoimiya-kokomi/miao-plugin/blob/7f6f1c84c89102bc6b1c58c8e1b06c61f4642161/apps/stat.js#L39) | ` '幻想真境剧诗' ` | ` /^#*(喵喵)*(本期\|上期)?(幻想\|幻境\|剧诗\|幻想真境剧诗)[ \|0-9]*(数据)?$/ ` | 由上层或处理函数决定 |
| [apps/stat.js:44](https://github.com/yoimiya-kokomi/miao-plugin/blob/7f6f1c84c89102bc6b1c58c8e1b06c61f4642161/apps/stat.js#L44) | ` '「月谕圣牌」收藏' ` | ` /^#*(喵喵)*(月谕\|越狱\|幻想\|幻境\|剧诗\|幻想真境剧诗)(圣牌\|卡片\|卡牌\|塔罗牌\|card\|tarot)(收藏\|收集)?[ \|0-9]*(数据)?$/ ` | 由上层或处理函数决定 |
| [apps/stat.js:49](https://github.com/yoimiya-kokomi/miao-plugin/blob/7f6f1c84c89102bc6b1c58c8e1b06c61f4642161/apps/stat.js#L49) | ` '「月谕圣牌」交换匹配' ` | ` /^#*(喵喵)*(月谕\|越狱\|幻想\|幻境\|剧诗\|幻想真境剧诗)(圣牌\|卡片\|卡牌\|塔罗牌\|card\|tarot)(交换\|互换\|换牌)$/ ` | 由上层或处理函数决定 |
| [apps/stat.js:54](https://github.com/yoimiya-kokomi/miao-plugin/blob/7f6f1c84c89102bc6b1c58c8e1b06c61f4642161/apps/stat.js#L54) | ` '幽境危战' ` | ` /^#*(喵喵)*(本期\|上期)?(幽境\|危战\|幽境危战)(单人\|单挑\|组队\|多人\|合作\|最佳)?[ \|0-9]*(数据)?$/ ` | 由上层或处理函数决定 |
| [apps/wiki.js:15](https://github.com/yoimiya-kokomi/miao-plugin/blob/7f6f1c84c89102bc6b1c58c8e1b06c61f4642161/apps/wiki.js#L15) | ` CharWiki.wiki ` | ` '^#喵喵WIKI$' ` | 由上层或处理函数决定 |
| [apps/wiki.js:22](https://github.com/yoimiya-kokomi/miao-plugin/blob/7f6f1c84c89102bc6b1c58c8e1b06c61f4642161/apps/wiki.js#L22) | ` Calendar.render ` | ` /^(#\|喵喵)+(日历\|日历列表)$/ ` | 由上层或处理函数决定 |
| [apps/wiki.js:28](https://github.com/yoimiya-kokomi/miao-plugin/blob/7f6f1c84c89102bc6b1c58c8e1b06c61f4642161/apps/wiki.js#L28) | ` CalendarSr.render ` | ` /^#(星铁)+(日历\|日历列表)$/ ` | 由上层或处理函数决定 |
| [apps/wiki.js:34](https://github.com/yoimiya-kokomi/miao-plugin/blob/7f6f1c84c89102bc6b1c58c8e1b06c61f4642161/apps/wiki.js#L34) | ` CalendarZZZ.render ` | ` /^#(绝区零)+(日历\|日历列表)$/ ` | 由上层或处理函数决定 |
| [apps/wiki.js:40](https://github.com/yoimiya-kokomi/miao-plugin/blob/7f6f1c84c89102bc6b1c58c8e1b06c61f4642161/apps/wiki.js#L40) | ` TodayMaterial.render ` | ` /^#(今日\|今天\|每日\|我的\|明天\|明日\|周([1-7]\|一\|二\|三\|四\|五\|六\|日))*(素材\|材料\|天赋)[ \|0-9]*$/ ` | 由上层或处理函数决定 |

无静态命令对的顶层模块（仍需覆盖）：[apps/index.js](https://github.com/yoimiya-kokomi/miao-plugin/blob/7f6f1c84c89102bc6b1c58c8e1b06c61f4642161/apps/index.js)。

## StarRail-plugin

固定提交：`090e411cf9be28b1644721fab54eab0ad283668f`。静态声明：54。

| 源文件与行 | 标签或处理函数 | 规则表达式 | 参考权限 |
| --- | --- | --- | --- |
| [apps/alias.js:17](https://github.com/TsukinaKasumi/StarRail-plugin/blob/090e411cf9be28b1644721fab54eab0ad283668f/apps/alias.js#L17) | ` 'addAlias' ` | `` `^${rulePrefix}(设置\|配置\|添加)(.*)(别名\|昵称)$` `` | 由上层或处理函数决定 |
| [apps/alias.js:21](https://github.com/TsukinaKasumi/StarRail-plugin/blob/090e411cf9be28b1644721fab54eab0ad283668f/apps/alias.js#L21) | ` 'delAlias' ` | `` `^${rulePrefix}删除(别名\|昵称)(.*)$` `` | 由上层或处理函数决定 |
| [apps/alias.js:25](https://github.com/TsukinaKasumi/StarRail-plugin/blob/090e411cf9be28b1644721fab54eab0ad283668f/apps/alias.js#L25) | ` 'aliasList' ` | `` `^${rulePrefix}(.*)(别名\|昵称)$` `` | 由上层或处理函数决定 |
| [apps/challenge.js:20](https://github.com/TsukinaKasumi/StarRail-plugin/blob/090e411cf9be28b1644721fab54eab0ad283668f/apps/challenge.js#L20) | ` 'challenge' ` | `` `^${rulePrefix}(上期\|本期)?(简易)?(深渊)` `` | 由上层或处理函数决定 |
| [apps/challenge.js:24](https://github.com/TsukinaKasumi/StarRail-plugin/blob/090e411cf9be28b1644721fab54eab0ad283668f/apps/challenge.js#L24) | ` 'challengeCurrent' ` | `` `^${rulePrefix}(最新\|当期)(简易)?(深渊)` `` | 由上层或处理函数决定 |
| [apps/challenge.js:28](https://github.com/TsukinaKasumi/StarRail-plugin/blob/090e411cf9be28b1644721fab54eab0ad283668f/apps/challenge.js#L28) | ` 'challengeForgottenHall' ` | `` `^${rulePrefix}(上期\|本期)?(简易)?(忘却\|忘却之庭\|混沌\|混沌回忆)` `` | 由上层或处理函数决定 |
| [apps/challenge.js:32](https://github.com/TsukinaKasumi/StarRail-plugin/blob/090e411cf9be28b1644721fab54eab0ad283668f/apps/challenge.js#L32) | ` 'challengeStory' ` | `` `^${rulePrefix}(上期\|本期)?(简易)?(虚构\|虚构叙事)` `` | 由上层或处理函数决定 |
| [apps/challenge.js:36](https://github.com/TsukinaKasumi/StarRail-plugin/blob/090e411cf9be28b1644721fab54eab0ad283668f/apps/challenge.js#L36) | ` 'challengeBoss' ` | `` `^${rulePrefix}(上期\|本期)?(简易)?(末日\|末日幻影)` `` | 由上层或处理函数决定 |
| [apps/challenge.js:40](https://github.com/TsukinaKasumi/StarRail-plugin/blob/090e411cf9be28b1644721fab54eab0ad283668f/apps/challenge.js#L40) | ` 'challengePeak' ` | `` `^${rulePrefix}(往期\|上期\|本期)?(简易)?(异乡\|异相\|异向\|仲裁\|异相仲裁)` `` | 由上层或处理函数决定 |
| [apps/gachasimulation.js:24](https://github.com/TsukinaKasumi/StarRail-plugin/blob/090e411cf9be28b1644721fab54eab0ad283668f/apps/gachasimulation.js#L24) | ` 'main' ` | `` `^${rulePrefix}(抽卡\|十连)(角色\|光锥\|常驻)?$` `` | 由上层或处理函数决定 |
| [apps/gatcha.js:20](https://github.com/TsukinaKasumi/StarRail-plugin/blob/090e411cf9be28b1644721fab54eab0ad283668f/apps/gatcha.js#L20) | ` 'bindAuthKey' ` | `` `^${rulePrefix}抽卡链接(绑定)?$` `` | 由上层或处理函数决定 |
| [apps/gatcha.js:24](https://github.com/TsukinaKasumi/StarRail-plugin/blob/090e411cf9be28b1644721fab54eab0ad283668f/apps/gatcha.js#L24) | ` 'gatcha' ` | `` `^${rulePrefix}(角色\|光锥\|武器\|常驻\|新手)?(跃迁\|抽卡)?(记录\|分析\|统计)` `` | 由上层或处理函数决定 |
| [apps/gatcha.js:28](https://github.com/TsukinaKasumi/StarRail-plugin/blob/090e411cf9be28b1644721fab54eab0ad283668f/apps/gatcha.js#L28) | ` 'gatchahelp' ` | `` `^${rulePrefix}抽卡帮助$` `` | 由上层或处理函数决定 |
| [apps/gatcha.js:32](https://github.com/TsukinaKasumi/StarRail-plugin/blob/090e411cf9be28b1644721fab54eab0ad283668f/apps/gatcha.js#L32) | ` 'updateGatcha' ` | `` `^${rulePrefix}更新(抽卡\|跃迁)(记录)?$` `` | 由上层或处理函数决定 |
| [apps/gridFight.js:19](https://github.com/TsukinaKasumi/StarRail-plugin/blob/090e411cf9be28b1644721fab54eab0ad283668f/apps/gridFight.js#L19) | ` 'grid_fight' ` | `` `^${rulePrefix}(货币(战争)?\|币战)$` `` | 由上层或处理函数决定 |
| [apps/gridFight.js:23](https://github.com/TsukinaKasumi/StarRail-plugin/blob/090e411cf9be28b1644721fab54eab0ad283668f/apps/gridFight.js#L23) | ` 'grid_fight_archive' ` | `` `^${rulePrefix}(货币(战争)?\|币战)(战绩\|回顾\|战报\|记录)(一\|二\|三\|四\|五\|六\|七\|八\|九\|十)?$` `` | 由上层或处理函数决定 |
| [apps/help.js:16](https://github.com/TsukinaKasumi/StarRail-plugin/blob/090e411cf9be28b1644721fab54eab0ad283668f/apps/help.js#L16) | ` 'help' ` | `` `^${rulePrefix}(帮助\|help)$` `` | 由上层或处理函数决定 |
| [apps/hkrpg.js:23](https://github.com/TsukinaKasumi/StarRail-plugin/blob/090e411cf9be28b1644721fab54eab0ad283668f/apps/hkrpg.js#L23) | ` 'bindSRUid' ` | `` `^${rulePrefix}绑定(uid\|UID)?(\\s)?[1-9][0-9]{8}$` `` | 由上层或处理函数决定 |
| [apps/hkrpg.js:29](https://github.com/TsukinaKasumi/StarRail-plugin/blob/090e411cf9be28b1644721fab54eab0ad283668f/apps/hkrpg.js#L29) | ` 'card' ` | `` `^${rulePrefix}(卡片\|探索\|角色)` `` | 由上层或处理函数决定 |
| [apps/hkrpg.js:33](https://github.com/TsukinaKasumi/StarRail-plugin/blob/090e411cf9be28b1644721fab54eab0ad283668f/apps/hkrpg.js#L33) | ` 'getPayLog' ` | `` `^${rulePrefix}(星琼\|古老梦华\|体力\|遗器\|光锥\|充值\|武器)记录(\\d){0,}$` `` | 由上层或处理函数决定 |
| [apps/hkrpg.js:37](https://github.com/TsukinaKasumi/StarRail-plugin/blob/090e411cf9be28b1644721fab54eab0ad283668f/apps/hkrpg.js#L37) | ` 'statisticsOnline' ` | `` `^${rulePrefix}在线(时长)?(统计\|分析)?` `` | 由上层或处理函数决定 |
| [apps/MiaoOrStarrail.js:21](https://github.com/TsukinaKasumi/StarRail-plugin/blob/090e411cf9be28b1644721fab54eab0ad283668f/apps/MiaoOrStarrail.js#L21) | ` 'update' ` | ` '^#星铁更新面板.*$' ` | 由上层或处理函数决定 |
| [apps/MiaoOrStarrail.js:25](https://github.com/TsukinaKasumi/StarRail-plugin/blob/090e411cf9be28b1644721fab54eab0ad283668f/apps/MiaoOrStarrail.js#L25) | ` 'StarRail' ` | ` '^#星铁(?!插件)(?!米游社)(?!更新)(.+)面板$' ` | 由上层或处理函数决定 |
| [apps/MiaoOrStarrail.js:29](https://github.com/TsukinaKasumi/StarRail-plugin/blob/090e411cf9be28b1644721fab54eab0ad283668f/apps/MiaoOrStarrail.js#L29) | ` 'Sr' ` | ` '^#(喵喵\|星铁)?插件面板(开启\|关闭\|状态)$' ` | 由上层或处理函数决定 |
| [apps/month.js:19](https://github.com/TsukinaKasumi/StarRail-plugin/blob/090e411cf9be28b1644721fab54eab0ad283668f/apps/month.js#L19) | ` 'month' ` | `` `^${rulePrefix}(星琼获取\|月历\|月收入\|收入\|原石)$` `` | 由上层或处理函数决定 |
| [apps/note.js:19](https://github.com/TsukinaKasumi/StarRail-plugin/blob/090e411cf9be28b1644721fab54eab0ad283668f/apps/note.js#L19) | ` 'note' ` | `` `^${rulePrefix}体力$` `` | 由上层或处理函数决定 |
| [apps/panel.js:46](https://github.com/TsukinaKasumi/StarRail-plugin/blob/090e411cf9be28b1644721fab54eab0ad283668f/apps/panel.js#L46) | ` 'panel' ` | `` `^${rulePrefix}(?!米游社)(.+)面板(更新)?(.*)` `` | 由上层或处理函数决定 |
| [apps/panel.js:50](https://github.com/TsukinaKasumi/StarRail-plugin/blob/090e411cf9be28b1644721fab54eab0ad283668f/apps/panel.js#L50) | ` 'plmb' ` | `` `^${rulePrefix}面板(列表)?$` `` | 由上层或处理函数决定 |
| [apps/panel.js:54](https://github.com/TsukinaKasumi/StarRail-plugin/blob/090e411cf9be28b1644721fab54eab0ad283668f/apps/panel.js#L54) | ` 'update' ` | `` `^${rulePrefix}(更新面板\|面板更新)(.*)` `` | 由上层或处理函数决定 |
| [apps/panel.js:58](https://github.com/TsukinaKasumi/StarRail-plugin/blob/090e411cf9be28b1644721fab54eab0ad283668f/apps/panel.js#L58) | ` 'changeApi' ` | `` `^${rulePrefix}(设置\|切换)面板(API\|api)?` `` | 由上层或处理函数决定 |
| [apps/panel.js:62](https://github.com/TsukinaKasumi/StarRail-plugin/blob/090e411cf9be28b1644721fab54eab0ad283668f/apps/panel.js#L62) | ` 'apiList' ` | `` `^${rulePrefix}(API\|api)列表$` `` | 由上层或处理函数决定 |
| [apps/panel.js:66](https://github.com/TsukinaKasumi/StarRail-plugin/blob/090e411cf9be28b1644721fab54eab0ad283668f/apps/panel.js#L66) | ` 'origImg' ` | ` '^#?原图$' ` | 由上层或处理函数决定 |
| [apps/rogue.js:20](https://github.com/TsukinaKasumi/StarRail-plugin/blob/090e411cf9be28b1644721fab54eab0ad283668f/apps/rogue.js#L20) | ` 'rogue' ` | `` `^${rulePrefix}(上期\|本期)?(模拟)?宇宙` `` | 由上层或处理函数决定 |
| [apps/rogue.js:24](https://github.com/TsukinaKasumi/StarRail-plugin/blob/090e411cf9be28b1644721fab54eab0ad283668f/apps/rogue.js#L24) | ` 'rogue_locust' ` | `` `^${rulePrefix}(寰宇)?蝗灾` `` | 由上层或处理函数决定 |
| [apps/rogue.js:28](https://github.com/TsukinaKasumi/StarRail-plugin/blob/090e411cf9be28b1644721fab54eab0ad283668f/apps/rogue.js#L28) | ` 'rogue_nous' ` | `` `^${rulePrefix}(黄金与机械\|黄金\|机械\|黄金机械)` `` | 由上层或处理函数决定 |
| [apps/rogue.js:32](https://github.com/TsukinaKasumi/StarRail-plugin/blob/090e411cf9be28b1644721fab54eab0ad283668f/apps/rogue.js#L32) | ` 'rogue_magic' ` | `` `^${rulePrefix}(不可知域)` `` | 由上层或处理函数决定 |
| [apps/rogue.js:36](https://github.com/TsukinaKasumi/StarRail-plugin/blob/090e411cf9be28b1644721fab54eab0ad283668f/apps/rogue.js#L36) | ` 'rogue_tourn' ` | `` `^${rulePrefix}(差分宇宙\|差分)` `` | 由上层或处理函数决定 |
| [apps/rogue.js:40](https://github.com/TsukinaKasumi/StarRail-plugin/blob/090e411cf9be28b1644721fab54eab0ad283668f/apps/rogue.js#L40) | ` 'rogue_tourn_normal' ` | `` `^${rulePrefix}常规(差分(宇宙)?\|演算)(战绩\|回顾\|战报\|记录)?(一\|二\|三)?` `` | 由上层或处理函数决定 |
| [apps/rogue.js:44](https://github.com/TsukinaKasumi/StarRail-plugin/blob/090e411cf9be28b1644721fab54eab0ad283668f/apps/rogue.js#L44) | ` 'rogue_tourn_week' ` | `` `^${rulePrefix}(本期\|上期\|本周\|上周)(差分(宇宙)?\|演算)(战绩\|回顾\|战报\|记录)?(一\|二\|三)?` `` | 由上层或处理函数决定 |
| [apps/rogue.js:48](https://github.com/TsukinaKasumi/StarRail-plugin/blob/090e411cf9be28b1644721fab54eab0ad283668f/apps/rogue.js#L48) | ` 'rogue_tourn_week_help' ` | `` `^${rulePrefix}周期(差分(宇宙)?\|演算)(战绩\|回顾\|战报\|记录)?(一\|二\|三)?` `` | 由上层或处理函数决定 |
| [apps/srxx.js:23](https://github.com/TsukinaKasumi/StarRail-plugin/blob/090e411cf9be28b1644721fab54eab0ad283668f/apps/srxx.js#L23) | ` 'cankao' ` | `` `^${rulePrefix}(.*)参考面板$` `` | 由上层或处理函数决定 |
| [apps/srxx.js:27](https://github.com/TsukinaKasumi/StarRail-plugin/blob/090e411cf9be28b1644721fab54eab0ad283668f/apps/srxx.js#L27) | ` 'srcankaohelp' ` | `` `^${rulePrefix}参考面板帮助$` `` | 由上层或处理函数决定 |
| [apps/srxx.js:31](https://github.com/TsukinaKasumi/StarRail-plugin/blob/090e411cf9be28b1644721fab54eab0ad283668f/apps/srxx.js#L31) | ` 'srsy' ` | `` `^${rulePrefix}收益曲线$` `` | 由上层或处理函数决定 |
| [apps/srxx.js:35](https://github.com/TsukinaKasumi/StarRail-plugin/blob/090e411cf9be28b1644721fab54eab0ad283668f/apps/srxx.js#L35) | ` 'srqd' ` | `` `^${rulePrefix}(全角色)?强度榜$` `` | 由上层或处理函数决定 |
| [apps/srxx.js:39](https://github.com/TsukinaKasumi/StarRail-plugin/blob/090e411cf9be28b1644721fab54eab0ad283668f/apps/srxx.js#L39) | ` 'srgl' ` | `` `^${rulePrefix}攻略$` `` | 由上层或处理函数决定 |
| [apps/srxx.js:43](https://github.com/TsukinaKasumi/StarRail-plugin/blob/090e411cf9be28b1644721fab54eab0ad283668f/apps/srxx.js#L43) | ` 'srEstimate' ` | `` `^${rulePrefix}预估$` `` | 由上层或处理函数决定 |
| [apps/srxx.js:47](https://github.com/TsukinaKasumi/StarRail-plugin/blob/090e411cf9be28b1644721fab54eab0ad283668f/apps/srxx.js#L47) | ` 'srsygl' ` | `` `^${rulePrefix}(深渊\|忘却之庭)攻略$` `` | 由上层或处理函数决定 |
| [apps/srxx.js:51](https://github.com/TsukinaKasumi/StarRail-plugin/blob/090e411cf9be28b1644721fab54eab0ad283668f/apps/srxx.js#L51) | ` 'srgz' ` | `` `^${rulePrefix}商店光锥推荐$` `` | 由上层或处理函数决定 |
| [apps/strategy.js:18](https://github.com/TsukinaKasumi/StarRail-plugin/blob/090e411cf9be28b1644721fab54eab0ad283668f/apps/strategy.js#L18) | ` 'strategy' ` | `` `^${rulePrefix}?(更新)?\\S+攻略(\\d+\|all)?$` `` | 由上层或处理函数决定 |
| [apps/strategy.js:22](https://github.com/TsukinaKasumi/StarRail-plugin/blob/090e411cf9be28b1644721fab54eab0ad283668f/apps/strategy.js#L22) | ` 'strategy_help' ` | `` `^${rulePrefix}攻略(说明\|帮助)$` `` | 由上层或处理函数决定 |
| [apps/strategy.js:26](https://github.com/TsukinaKasumi/StarRail-plugin/blob/090e411cf9be28b1644721fab54eab0ad283668f/apps/strategy.js#L26) | ` 'strategy_setting' ` | `` `^${rulePrefix}设置默认攻略(\\d+)?$` `` | 由上层或处理函数决定 |
| [apps/update.js:40](https://github.com/TsukinaKasumi/StarRail-plugin/blob/090e411cf9be28b1644721fab54eab0ad283668f/apps/update.js#L40) | ` 'update' ` | `` `^${rulePrefix}(插件)?(强制)?更新$` `` | 由上层或处理函数决定 |
| [apps/update.js:44](https://github.com/TsukinaKasumi/StarRail-plugin/blob/090e411cf9be28b1644721fab54eab0ad283668f/apps/update.js#L44) | ` '【#管理】更新素材' ` | `` `^${rulePrefix}(强制)?(更新图像\|图像更新)(github\|gitee)?$` `` | 由上层或处理函数决定 |
| [apps/version.js:13](https://github.com/TsukinaKasumi/StarRail-plugin/blob/090e411cf9be28b1644721fab54eab0ad283668f/apps/version.js#L13) | ` 'updateLog' ` | `` `^${rulePrefix}(插件)?更新日志$` `` | 由上层或处理函数决定 |

## ZZZ-Plugin

固定提交：`e35d30d52a395eae06d58f41d0ffca1f998b183b`。静态声明：82。

| 源文件与行 | 标签或处理函数 | 规则表达式 | 参考权限 |
| --- | --- | --- | --- |
| [dist/apps/abyss.js:15](https://github.com/ZZZure/ZZZ-Plugin/blob/e35d30d52a395eae06d58f41d0ffca1f998b183b/dist/apps/abyss.js#L15) | ` 'abyss' ` | `` `${rulePrefix}(上期\|往期)?(式舆防卫战\|式舆\|深渊\|防卫战\|防卫)$` `` | 由上层或处理函数决定 |
| [dist/apps/calendar.js:14](https://github.com/ZZZure/ZZZ-Plugin/blob/e35d30d52a395eae06d58f41d0ffca1f998b183b/dist/apps/calendar.js#L14) | ` 'calendar' ` | `` `${rulePrefix}(cal\|日历)$` `` | 由上层或处理函数决定 |
| [dist/apps/card.js:14](https://github.com/ZZZure/ZZZ-Plugin/blob/e35d30d52a395eae06d58f41d0ffca1f998b183b/dist/apps/card.js#L14) | ` 'card' ` | `` `${rulePrefix}(card\|卡片\|个人信息\|角色)$` `` | 由上层或处理函数决定 |
| [dist/apps/climbingTower.js:15](https://github.com/ZZZure/ZZZ-Plugin/blob/e35d30d52a395eae06d58f41d0ffca1f998b183b/dist/apps/climbingTower.js#L15) | ` "climbingTower" ` | `` `${rulePrefix}(拟真鏖战试炼\|鏖战\|爬塔)$` `` | 由上层或处理函数决定 |
| [dist/apps/code.js:14](https://github.com/ZZZure/ZZZ-Plugin/blob/e35d30d52a395eae06d58f41d0ffca1f998b183b/dist/apps/code.js#L14) | ` 'code' ` | `` `${rulePrefix}(code\|兑换码)$` `` | 由上层或处理函数决定 |
| [dist/apps/damage.js:14](https://github.com/ZZZure/ZZZ-Plugin/blob/e35d30d52a395eae06d58f41d0ffca1f998b183b/dist/apps/damage.js#L14) | ` 'charDamagePanel' ` | `` `${rulePrefix}(.+)伤害\\d*$` `` | 由上层或处理函数决定 |
| [dist/apps/deadly.js:16](https://github.com/ZZZure/ZZZ-Plugin/blob/e35d30d52a395eae06d58f41d0ffca1f998b183b/dist/apps/deadly.js#L16) | ` 'deadly' ` | `` `${rulePrefix}(上期\|往期)?(危局强袭战\|危局\|强袭\|强袭战)$` `` | 由上层或处理函数决定 |
| [dist/apps/explorationDetail.js:12](https://github.com/ZZZure/ZZZ-Plugin/blob/e35d30d52a395eae06d58f41d0ffca1f998b183b/dist/apps/explorationDetail.js#L12) | ` "explorationDetail" ` | `` `${rulePrefix}(区域收集\|收集\|探索\|探索度)$` `` | 由上层或处理函数决定 |
| [dist/apps/gachalog.js:19](https://github.com/ZZZure/ZZZ-Plugin/blob/e35d30d52a395eae06d58f41d0ffca1f998b183b/dist/apps/gachalog.js#L19) | ` 'gachaHelp' ` | `` `^${rulePrefix}抽卡帮助$` `` | 由上层或处理函数决定 |
| [dist/apps/gachalog.js:23](https://github.com/ZZZure/ZZZ-Plugin/blob/e35d30d52a395eae06d58f41d0ffca1f998b183b/dist/apps/gachalog.js#L23) | ` 'startGachaLog' ` | `` `${rulePrefix}抽卡链接$` `` | 由上层或处理函数决定 |
| [dist/apps/gachalog.js:27](https://github.com/ZZZure/ZZZ-Plugin/blob/e35d30d52a395eae06d58f41d0ffca1f998b183b/dist/apps/gachalog.js#L27) | ` 'refreshGachaLog' ` | `` `${rulePrefix}(刷新\|更新)抽卡(链接\|记录)?$` `` | 由上层或处理函数决定 |
| [dist/apps/gachalog.js:31](https://github.com/ZZZure/ZZZ-Plugin/blob/e35d30d52a395eae06d58f41d0ffca1f998b183b/dist/apps/gachalog.js#L31) | ` 'gachaLogAnalysis' ` | `` `^${rulePrefix}抽卡(分析\|记录\|统计)$` `` | 由上层或处理函数决定 |
| [dist/apps/gachalog.js:35](https://github.com/ZZZure/ZZZ-Plugin/blob/e35d30d52a395eae06d58f41d0ffca1f998b183b/dist/apps/gachalog.js#L35) | ` 'getGachaLink' ` | `` `^${rulePrefix}获取抽卡链接$` `` | 由上层或处理函数决定 |
| [dist/apps/guide.js:24](https://github.com/ZZZure/ZZZ-Plugin/blob/e35d30d52a395eae06d58f41d0ffca1f998b183b/dist/apps/guide.js#L24) | ` 'GuideHelp' ` | `` `^${rulePrefix}攻略(说明\|帮助)$` `` | 由上层或处理函数决定 |
| [dist/apps/guide.js:28](https://github.com/ZZZure/ZZZ-Plugin/blob/e35d30d52a395eae06d58f41d0ffca1f998b183b/dist/apps/guide.js#L28) | ` 'Guide' ` | `` `${rulePrefix}(更新)?\\S+攻略(\\d+\|all)?$` `` | 由上层或处理函数决定 |
| [dist/apps/help.js:520](https://github.com/ZZZure/ZZZ-Plugin/blob/e35d30d52a395eae06d58f41d0ffca1f998b183b/dist/apps/help.js#L520) | ` 'help' ` | `` `${rulePrefix}(帮助\|help)$` `` | 由上层或处理函数决定 |
| [dist/apps/hollowZero.js:12](https://github.com/ZZZure/ZZZ-Plugin/blob/e35d30d52a395eae06d58f41d0ffca1f998b183b/dist/apps/hollowZero.js#L12) | ` "hollowZeroHelp" ` | `` `${rulePrefix}(零号空洞\|零号\|空洞)$` `` | 由上层或处理函数决定 |
| [dist/apps/hollowZero.js:16](https://github.com/ZZZure/ZZZ-Plugin/blob/e35d30d52a395eae06d58f41d0ffca1f998b183b/dist/apps/hollowZero.js#L16) | ` "hollowZero" ` | `` `${rulePrefix}(枯萎苗圃\|枯萎\|苗圃)$` `` | 由上层或处理函数决定 |
| [dist/apps/hollowZero.js:20](https://github.com/ZZZure/ZZZ-Plugin/blob/e35d30d52a395eae06d58f41d0ffca1f998b183b/dist/apps/hollowZero.js#L20) | ` "hollowZeroS2" ` | `` `${rulePrefix}(迷失之地\|迷失)$` `` | 由上层或处理函数决定 |
| [dist/apps/holoBoss.js:16](https://github.com/ZZZure/ZZZ-Plugin/blob/e35d30d52a395eae06d58f41d0ffca1f998b183b/dist/apps/holoBoss.js#L16) | ` 'holoBoss' ` | `` `${rulePrefix}(上期\|往期)?(拟境湮灭战\|拟境\|湮灭\|湮灭战)$` `` | 由上层或处理函数决定 |
| [dist/apps/manage.js:37](https://github.com/ZZZure/ZZZ-Plugin/blob/e35d30d52a395eae06d58f41d0ffca1f998b183b/dist/apps/manage.js#L37) | ` 'downloadAll' ` | `` `${rulePrefix}下载(全部\|所有)资源$` `` | 由上层或处理函数决定 |
| [dist/apps/manage.js:41](https://github.com/ZZZure/ZZZ-Plugin/blob/e35d30d52a395eae06d58f41d0ffca1f998b183b/dist/apps/manage.js#L41) | ` 'deleteAll' ` | `` `${rulePrefix}删除(全部\|所有)资源$` `` | 由上层或处理函数决定 |
| [dist/apps/manage.js:45](https://github.com/ZZZure/ZZZ-Plugin/blob/e35d30d52a395eae06d58f41d0ffca1f998b183b/dist/apps/manage.js#L45) | ` 'setDefaultGuide' ` | `` `${rulePrefix}设置默认攻略(\\d+\|all)$` `` | 由上层或处理函数决定 |
| [dist/apps/manage.js:49](https://github.com/ZZZure/ZZZ-Plugin/blob/e35d30d52a395eae06d58f41d0ffca1f998b183b/dist/apps/manage.js#L49) | ` 'setMaxForwardGuide' ` | `` `${rulePrefix}设置所有攻略显示个数(\\d+)$` `` | 由上层或处理函数决定 |
| [dist/apps/manage.js:53](https://github.com/ZZZure/ZZZ-Plugin/blob/e35d30d52a395eae06d58f41d0ffca1f998b183b/dist/apps/manage.js#L53) | ` 'setRenderPrecision' ` | `` `${rulePrefix}设置渲染精度(\\d+)$` `` | 由上层或处理函数决定 |
| [dist/apps/manage.js:57](https://github.com/ZZZure/ZZZ-Plugin/blob/e35d30d52a395eae06d58f41d0ffca1f998b183b/dist/apps/manage.js#L57) | ` 'setRefreshGachaInterval' ` | `` `${rulePrefix}刷新抽卡间隔(\\d+)$` `` | 由上层或处理函数决定 |
| [dist/apps/manage.js:61](https://github.com/ZZZure/ZZZ-Plugin/blob/e35d30d52a395eae06d58f41d0ffca1f998b183b/dist/apps/manage.js#L61) | ` 'setRefreshPanelInterval' ` | `` `${rulePrefix}刷新面板间隔(\\d+)$` `` | 由上层或处理函数决定 |
| [dist/apps/manage.js:65](https://github.com/ZZZure/ZZZ-Plugin/blob/e35d30d52a395eae06d58f41d0ffca1f998b183b/dist/apps/manage.js#L65) | ` 'setRefreshCharInterval' ` | `` `${rulePrefix}刷新角色间隔(\\d+)$` `` | 由上层或处理函数决定 |
| [dist/apps/manage.js:69](https://github.com/ZZZure/ZZZ-Plugin/blob/e35d30d52a395eae06d58f41d0ffca1f998b183b/dist/apps/manage.js#L69) | ` 'addAlias' ` | `` `${rulePrefix}添加(\\S+)别名(\\S+)$` `` | 由上层或处理函数决定 |
| [dist/apps/manage.js:73](https://github.com/ZZZure/ZZZ-Plugin/blob/e35d30d52a395eae06d58f41d0ffca1f998b183b/dist/apps/manage.js#L73) | ` 'deleteAlias' ` | `` `${rulePrefix}删除别名(\\S+)$` `` | 由上层或处理函数决定 |
| [dist/apps/manage.js:77](https://github.com/ZZZure/ZZZ-Plugin/blob/e35d30d52a395eae06d58f41d0ffca1f998b183b/dist/apps/manage.js#L77) | ` 'listAlias' ` | `` `${rulePrefix}(\\S+)别名$` `` | 由上层或处理函数决定 |
| [dist/apps/manage.js:81](https://github.com/ZZZure/ZZZ-Plugin/blob/e35d30d52a395eae06d58f41d0ffca1f998b183b/dist/apps/manage.js#L81) | ` 'uploadCharacterImg' ` | `` `${rulePrefix}(上传\|添加)(\\S+)(角色\|面板)图$` `` | 由上层或处理函数决定 |
| [dist/apps/manage.js:85](https://github.com/ZZZure/ZZZ-Plugin/blob/e35d30d52a395eae06d58f41d0ffca1f998b183b/dist/apps/manage.js#L85) | ` 'getCharacterImages' ` | `` `${rulePrefix}(获取\|查看)(\\S+)(角色\|面板)图(\\d+)?$` `` | 由上层或处理函数决定 |
| [dist/apps/manage.js:89](https://github.com/ZZZure/ZZZ-Plugin/blob/e35d30d52a395eae06d58f41d0ffca1f998b183b/dist/apps/manage.js#L89) | ` 'deleteCharacterImg' ` | `` `${rulePrefix}删除(\\S+)(角色\|面板)图(.+)$` `` | 由上层或处理函数决定 |
| [dist/apps/manage.js:93](https://github.com/ZZZure/ZZZ-Plugin/blob/e35d30d52a395eae06d58f41d0ffca1f998b183b/dist/apps/manage.js#L93) | ` 'switchGroupRank' ` | `` `${rulePrefix}(开启\|打开\|on\|启用\|启动\|关闭\|关掉\|off\|禁用\|停止)群(内)?(式舆防卫战\|式舆\|深渊\|防卫战\|防卫\|危局强袭战\|危局\|强袭\|强袭战\|临界推演\|临界\|推演)排名$` `` | 由上层或处理函数决定 |
| [dist/apps/manage.js:97](https://github.com/ZZZure/ZZZ-Plugin/blob/e35d30d52a395eae06d58f41d0ffca1f998b183b/dist/apps/manage.js#L97) | ` 'getChangeLog' ` | `` `${rulePrefix}(插件)?版本$` `` | 由上层或处理函数决定 |
| [dist/apps/manage.js:101](https://github.com/ZZZure/ZZZ-Plugin/blob/e35d30d52a395eae06d58f41d0ffca1f998b183b/dist/apps/manage.js#L101) | ` 'getCommitLog' ` | `` `^${rulePrefix}(插件)?更新日志$` `` | 由上层或处理函数决定 |
| [dist/apps/manage.js:105](https://github.com/ZZZure/ZZZ-Plugin/blob/e35d30d52a395eae06d58f41d0ffca1f998b183b/dist/apps/manage.js#L105) | ` 'hasUpdate' ` | `` `^${rulePrefix}(插件)?检查更新$` `` | 由上层或处理函数决定 |
| [dist/apps/manage.js:109](https://github.com/ZZZure/ZZZ-Plugin/blob/e35d30d52a395eae06d58f41d0ffca1f998b183b/dist/apps/manage.js#L109) | ` 'setDefaultDevice' ` | `` `${rulePrefix}设置默认设备` `` | 由上层或处理函数决定 |
| [dist/apps/manage.js:113](https://github.com/ZZZure/ZZZ-Plugin/blob/e35d30d52a395eae06d58f41d0ffca1f998b183b/dist/apps/manage.js#L113) | ` 'enableAutoUpdatePush' ` | `` `${rulePrefix}(开启\|关闭)更新推送$` `` | 由上层或处理函数决定 |
| [dist/apps/manage.js:117](https://github.com/ZZZure/ZZZ-Plugin/blob/e35d30d52a395eae06d58f41d0ffca1f998b183b/dist/apps/manage.js#L117) | ` 'setCheckUpdateCron' ` | `` `${rulePrefix}设置检查更新时间(.+)$` `` | 由上层或处理函数决定 |
| [dist/apps/manage.js:121](https://github.com/ZZZure/ZZZ-Plugin/blob/e35d30d52a395eae06d58f41d0ffca1f998b183b/dist/apps/manage.js#L121) | ` 'resetGroupRank' ` | `` `${rulePrefix}(重置\|清空)(式舆防卫战\|式舆\|深渊\|防卫战\|防卫)排名$` `` | 由上层或处理函数决定 |
| [dist/apps/monthly.js:15](https://github.com/ZZZure/ZZZ-Plugin/blob/e35d30d52a395eae06d58f41d0ffca1f998b183b/dist/apps/monthly.js#L15) | ` 'monthly' ` | `` `${rulePrefix}(monthly\|菲林\|邦布券\|收入\|月报)((\\d{4})年)?((\\d{1,2}\|上)月)?$` `` | 由上层或处理函数决定 |
| [dist/apps/monthly.js:19](https://github.com/ZZZure/ZZZ-Plugin/blob/e35d30d52a395eae06d58f41d0ffca1f998b183b/dist/apps/monthly.js#L19) | ` 'monthlyCollect' ` | `` `${rulePrefix}(monthly\|菲林\|邦布券\|收入\|月报)统计$` `` | 由上层或处理函数决定 |
| [dist/apps/note.js:14](https://github.com/ZZZure/ZZZ-Plugin/blob/e35d30d52a395eae06d58f41d0ffca1f998b183b/dist/apps/note.js#L14) | ` 'note' ` | `` `${rulePrefix}(note\|每日\|体力\|便笺\|便签)$` `` | 由上层或处理函数决定 |
| [dist/apps/panel.js:16](https://github.com/ZZZure/ZZZ-Plugin/blob/e35d30d52a395eae06d58f41d0ffca1f998b183b/dist/apps/panel.js#L16) | ` 'handleRule' ` | `` `${rulePrefix}(.*)面板(展柜)?(刷新\|更新\|列表)?$` `` | 由上层或处理函数决定 |
| [dist/apps/panel.js:20](https://github.com/ZZZure/ZZZ-Plugin/blob/e35d30d52a395eae06d58f41d0ffca1f998b183b/dist/apps/panel.js#L20) | ` 'proficiency' ` | `` `${rulePrefix}练度(统计)?$` `` | 由上层或处理函数决定 |
| [dist/apps/panel.js:24](https://github.com/ZZZure/ZZZ-Plugin/blob/e35d30d52a395eae06d58f41d0ffca1f998b183b/dist/apps/panel.js#L24) | ` 'getCharOriImage' ` | `` `${rulePrefix}原图$` `` | 由上层或处理函数决定 |
| [dist/apps/poolHistory.js:19](https://github.com/ZZZure/ZZZ-Plugin/blob/e35d30d52a395eae06d58f41d0ffca1f998b183b/dist/apps/poolHistory.js#L19) | ` 'dispatchHandler' ` | `` `${rulePrefix}.+(复刻\|卡池)(统计\|记录\|历史)$` `` | 由上层或处理函数决定 |
| [dist/apps/poolHistory.js:23](https://github.com/ZZZure/ZZZ-Plugin/blob/e35d30d52a395eae06d58f41d0ffca1f998b183b/dist/apps/poolHistory.js#L23) | ` 'queryCurrentPool' ` | `` `${rulePrefix}(当前\|本期\|当期)?卡池$` `` | 由上层或处理函数决定 |
| [dist/apps/poolHistory.js:27](https://github.com/ZZZure/ZZZ-Plugin/blob/e35d30d52a395eae06d58f41d0ffca1f998b183b/dist/apps/poolHistory.js#L27) | ` 'queryAllPool' ` | `` `${rulePrefix}(复刻\|卡池)(统计\|记录\|历史)$` `` | 由上层或处理函数决定 |
| [dist/apps/poolHistory.js:31](https://github.com/ZZZure/ZZZ-Plugin/blob/e35d30d52a395eae06d58f41d0ffca1f998b183b/dist/apps/poolHistory.js#L31) | ` 'queryVersionPool' ` | `` `${rulePrefix}v?(\\d+\\.\\d+)(上半\|下半)?卡池$` `` | 由上层或处理函数决定 |
| [dist/apps/rank.js:20](https://github.com/ZZZure/ZZZ-Plugin/blob/e35d30d52a395eae06d58f41d0ffca1f998b183b/dist/apps/rank.js#L20) | ` 'abyssRank' ` | `` `${rulePrefix}(式舆防卫战\|式舆\|深渊\|防卫战\|防卫)排名$` `` | 由上层或处理函数决定 |
| [dist/apps/rank.js:24](https://github.com/ZZZure/ZZZ-Plugin/blob/e35d30d52a395eae06d58f41d0ffca1f998b183b/dist/apps/rank.js#L24) | ` 'deadlyRank' ` | `` `${rulePrefix}(危局强袭战\|危局\|强袭\|强袭战)排名$` `` | 由上层或处理函数决定 |
| [dist/apps/rank.js:28](https://github.com/ZZZure/ZZZ-Plugin/blob/e35d30d52a395eae06d58f41d0ffca1f998b183b/dist/apps/rank.js#L28) | ` 'deadlyHardRank' ` | `` `${rulePrefix}(绝境\|(危局\|强袭\|强袭战\|危局强袭战)绝境\|绝境(危局\|强袭\|强袭战\|危局强袭战))排名$` `` | 由上层或处理函数决定 |
| [dist/apps/rank.js:32](https://github.com/ZZZure/ZZZ-Plugin/blob/e35d30d52a395eae06d58f41d0ffca1f998b183b/dist/apps/rank.js#L32) | ` 'holoBossRank' ` | `` `${rulePrefix}(拟境湮灭战\|拟境\|湮灭\|湮灭战)排名$` `` | 由上层或处理函数决定 |
| [dist/apps/rank.js:36](https://github.com/ZZZure/ZZZ-Plugin/blob/e35d30d52a395eae06d58f41d0ffca1f998b183b/dist/apps/rank.js#L36) | ` 'voidFrontBattleRank' ` | `` `${rulePrefix}(临界推演\|临界\|推演)排名$` `` | 由上层或处理函数决定 |
| [dist/apps/rank.js:40](https://github.com/ZZZure/ZZZ-Plugin/blob/e35d30d52a395eae06d58f41d0ffca1f998b183b/dist/apps/rank.js#L40) | ` 'climbingTowerHelp' ` | `` `${rulePrefix}(爬塔\|鏖战)排名$` `` | 由上层或处理函数决定 |
| [dist/apps/rank.js:44](https://github.com/ZZZure/ZZZ-Plugin/blob/e35d30d52a395eae06d58f41d0ffca1f998b183b/dist/apps/rank.js#L44) | ` 'climbingTowerS1' ` | `` `${rulePrefix}(爬塔S1\|爬塔s1\|拟真鏖战试炼)排名$` `` | 由上层或处理函数决定 |
| [dist/apps/rank.js:48](https://github.com/ZZZure/ZZZ-Plugin/blob/e35d30d52a395eae06d58f41d0ffca1f998b183b/dist/apps/rank.js#L48) | ` 'climbingTowerS2' ` | `` `${rulePrefix}(爬塔S2\|爬塔s2\|鏖战试炼末路\|鏖战试炼：末路\|鏖战试炼:末路)排名$` `` | 由上层或处理函数决定 |
| [dist/apps/rank.js:52](https://github.com/ZZZure/ZZZ-Plugin/blob/e35d30d52a395eae06d58f41d0ffca1f998b183b/dist/apps/rank.js#L52) | ` 'climbingTowerS3' ` | `` `${rulePrefix}(爬塔S3\|爬塔s3\|鏖战试炼荣耀\|鏖战试炼：荣耀\|鏖战试炼:荣耀)排名$` `` | 由上层或处理函数决定 |
| [dist/apps/rank.js:56](https://github.com/ZZZure/ZZZ-Plugin/blob/e35d30d52a395eae06d58f41d0ffca1f998b183b/dist/apps/rank.js#L56) | ` 'climbingTowerS4' ` | `` `${rulePrefix}(爬塔S4\|爬塔s4\|鏖战试炼狂澜\|鏖战试炼：狂澜\|鏖战试炼:狂澜)排名$` `` | 由上层或处理函数决定 |
| [dist/apps/rank.js:60](https://github.com/ZZZure/ZZZ-Plugin/blob/e35d30d52a395eae06d58f41d0ffca1f998b183b/dist/apps/rank.js#L60) | ` 'switchRank' ` | `` `${rulePrefix}(显示\|展示\|开启\|打开\|on\|启用\|启动\|隐藏\|取消显示\|关闭\|关掉\|off\|禁用\|停止)(式舆防卫战\|式舆\|深渊\|防卫战\|防卫\|危局强袭战\|危局\|强袭\|强袭战\|拟境湮灭战\|拟境\|湮灭\|湮灭战\|临界推演\|临界\|推演\|爬塔S1\|爬塔S2\|爬塔S3\|爬塔 s1\|爬塔 s2\|爬塔 s3\|爬塔s1\|爬塔s2\|爬塔s3)?(群(内)?)?排名$` `` | 由上层或处理函数决定 |
| [dist/apps/remind.js:14](https://github.com/ZZZure/ZZZ-Plugin/blob/e35d30d52a395eae06d58f41d0ffca1f998b183b/dist/apps/remind.js#L14) | ` 'setSubscribeEnable' ` | `` `${rulePrefix}(开启\|关闭)挑战提醒$` `` | 由上层或处理函数决定 |
| [dist/apps/remind.js:18](https://github.com/ZZZure/ZZZ-Plugin/blob/e35d30d52a395eae06d58f41d0ffca1f998b183b/dist/apps/remind.js#L18) | ` 'setGlobalRemindEnable' ` | `` `${rulePrefix}(开启\|启用\|关闭\|禁用)全局挑战提醒$` `` | ` 'master' ` |
| [dist/apps/remind.js:23](https://github.com/ZZZure/ZZZ-Plugin/blob/e35d30d52a395eae06d58f41d0ffca1f998b183b/dist/apps/remind.js#L23) | ` 'setAbyssThreshold' ` | `` `${rulePrefix}设置(全局)?式舆阈值\\s*(\\d+)` `` | 由上层或处理函数决定 |
| [dist/apps/remind.js:27](https://github.com/ZZZure/ZZZ-Plugin/blob/e35d30d52a395eae06d58f41d0ffca1f998b183b/dist/apps/remind.js#L27) | ` 'setDeadlyThreshold' ` | `` `${rulePrefix}设置(全局)?危局阈值\\s*(\\d+)` `` | 由上层或处理函数决定 |
| [dist/apps/remind.js:31](https://github.com/ZZZure/ZZZ-Plugin/blob/e35d30d52a395eae06d58f41d0ffca1f998b183b/dist/apps/remind.js#L31) | ` 'checkNow' ` | `` `${rulePrefix}查询挑战状态$` `` | 由上层或处理函数决定 |
| [dist/apps/remind.js:35](https://github.com/ZZZure/ZZZ-Plugin/blob/e35d30d52a395eae06d58f41d0ffca1f998b183b/dist/apps/remind.js#L35) | ` 'setMyRemindTime' ` | `` `${rulePrefix}设置个人提醒时间\\s*(每日\\d+时(?:(\\d+)分)?\|每周.\\d+时(?:(\\d+)分)?)` `` | 由上层或处理函数决定 |
| [dist/apps/remind.js:39](https://github.com/ZZZure/ZZZ-Plugin/blob/e35d30d52a395eae06d58f41d0ffca1f998b183b/dist/apps/remind.js#L39) | ` 'viewMyRemindTime' ` | `` `${rulePrefix}个人提醒(状态\|时间)$` `` | 由上层或处理函数决定 |
| [dist/apps/remind.js:43](https://github.com/ZZZure/ZZZ-Plugin/blob/e35d30d52a395eae06d58f41d0ffca1f998b183b/dist/apps/remind.js#L43) | ` 'deleteMyRemindTime' ` | `` `${rulePrefix}(重置\|删除\|取消)个人提醒时间` `` | 由上层或处理函数决定 |
| [dist/apps/remind.js:47](https://github.com/ZZZure/ZZZ-Plugin/blob/e35d30d52a395eae06d58f41d0ffca1f998b183b/dist/apps/remind.js#L47) | ` 'setGlobalRemindTime' ` | `` `${rulePrefix}设置全局提醒时间\\s*(每日\\d+时(?:(\\d+)分)?\|每周.\\d+时(?:(\\d+)分)?)` `` | ` 'master' ` |
| [dist/apps/remind.js:52](https://github.com/ZZZure/ZZZ-Plugin/blob/e35d30d52a395eae06d58f41d0ffca1f998b183b/dist/apps/remind.js#L52) | ` 'viewGlobalRemindTime' ` | `` `${rulePrefix}全局提醒时间$` `` | 由上层或处理函数决定 |
| [dist/apps/update.js:18](https://github.com/ZZZure/ZZZ-Plugin/blob/e35d30d52a395eae06d58f41d0ffca1f998b183b/dist/apps/update.js#L18) | ` 'update' ` | `` `^${rulePrefix}(插件)?(强制)?更新(插件)?$` `` | 由上层或处理函数决定 |
| [dist/apps/user.js:14](https://github.com/ZZZure/ZZZ-Plugin/blob/e35d30d52a395eae06d58f41d0ffca1f998b183b/dist/apps/user.js#L14) | ` 'bindDevice' ` | `` `${rulePrefix}绑定设备$` `` | 由上层或处理函数决定 |
| [dist/apps/user.js:18](https://github.com/ZZZure/ZZZ-Plugin/blob/e35d30d52a395eae06d58f41d0ffca1f998b183b/dist/apps/user.js#L18) | ` 'deleteBind' ` | `` `${rulePrefix}解绑设备$` `` | 由上层或处理函数决定 |
| [dist/apps/user.js:22](https://github.com/ZZZure/ZZZ-Plugin/blob/e35d30d52a395eae06d58f41d0ffca1f998b183b/dist/apps/user.js#L22) | ` 'bindDeviceHelp' ` | `` `${rulePrefix}绑定设备帮助$` `` | 由上层或处理函数决定 |
| [dist/apps/voidFrontBattle.js:15](https://github.com/ZZZure/ZZZ-Plugin/blob/e35d30d52a395eae06d58f41d0ffca1f998b183b/dist/apps/voidFrontBattle.js#L15) | ` "voidFrontBattle" ` | `` `${rulePrefix}(上期\|往期)?(临界推演\|临界\|推演)$` `` | 由上层或处理函数决定 |
| [dist/apps/wiki.js:41](https://github.com/ZZZure/ZZZ-Plugin/blob/e35d30d52a395eae06d58f41d0ffca1f998b183b/dist/apps/wiki.js#L41) | ` 'skills' ` | `` `${rulePrefix}(.*)(天赋\|技能)(.*)$` `` | 由上层或处理函数决定 |
| [dist/apps/wiki.js:45](https://github.com/ZZZure/ZZZ-Plugin/blob/e35d30d52a395eae06d58f41d0ffca1f998b183b/dist/apps/wiki.js#L45) | ` 'cinema' ` | `` `${rulePrefix}(.*)(意象影画\|意象\|影画\|命座)$` `` | 由上层或处理函数决定 |
| [dist/apps/zenkov.js:12](https://github.com/ZZZure/ZZZ-Plugin/blob/e35d30d52a395eae06d58f41d0ffca1f998b183b/dist/apps/zenkov.js#L12) | ` 'zenkov' ` | `` `${rulePrefix}(迷宫诡域\|迷宫\|诡域\|搜打撤\|塔科夫\|塔可夫\|塔克夫\|鸭科夫\|绝科夫)$` `` | 由上层或处理函数决定 |
| [dist/apps/zenkov.js:16](https://github.com/ZZZure/ZZZ-Plugin/blob/e35d30d52a395eae06d58f41d0ffca1f998b183b/dist/apps/zenkov.js#L16) | ` 'zenkovDetail' ` | `` `${rulePrefix}(迷宫诡域\|迷宫\|诡域\|搜打撤\|塔科夫\|塔可夫\|塔克夫\|鸭科夫\|绝科夫)(战绩\|记录\|详情\|回顾)$` `` | 由上层或处理函数决定 |

## Yunzai-genshin

固定提交：`4a2e1fb8f094b2a8f039ce0768773701718b60b9`。静态声明：67。

| 源文件与行 | 标签或处理函数 | 规则表达式 | 参考权限 |
| --- | --- | --- | --- |
| [apps/abbrSet.js:16](https://github.com/TimeRainStarSky/Yunzai-genshin/blob/4a2e1fb8f094b2a8f039ce0768773701718b60b9/apps/abbrSet.js#L16) | ` "abbr" ` | ` "^#(星铁)?(设置\|配置)(.*)(别名\|昵称)$" ` | 由上层或处理函数决定 |
| [apps/abbrSet.js:20](https://github.com/TimeRainStarSky/Yunzai-genshin/blob/4a2e1fb8f094b2a8f039ce0768773701718b60b9/apps/abbrSet.js#L20) | ` "delAbbr" ` | ` "^#(星铁)?删除(别名\|昵称)(.*)$" ` | 由上层或处理函数决定 |
| [apps/abbrSet.js:24](https://github.com/TimeRainStarSky/Yunzai-genshin/blob/4a2e1fb8f094b2a8f039ce0768773701718b60b9/apps/abbrSet.js#L24) | ` "abbrList" ` | ` "^#(星铁)?(.*)(别名\|昵称)$" ` | 由上层或处理函数决定 |
| [apps/buddy.js:12](https://github.com/TimeRainStarSky/Yunzai-genshin/blob/4a2e1fb8f094b2a8f039ce0768773701718b60b9/apps/buddy.js#L12) | ` "note" ` | ` "^#*绝区零?(邦布\|人偶)$" ` | 由上层或处理函数决定 |
| [apps/calculator.js:15](https://github.com/TimeRainStarSky/Yunzai-genshin/blob/4a2e1fb8f094b2a8f039ce0768773701718b60b9/apps/calculator.js#L15) | ` "Calculator" ` | ` "^#*(星铁)?(.*)(养成\|计算)([0-9]\|,\|，\| )*$" ` | 由上层或处理函数决定 |
| [apps/calculator.js:19](https://github.com/TimeRainStarSky/Yunzai-genshin/blob/4a2e1fb8f094b2a8f039ce0768773701718b60b9/apps/calculator.js#L19) | ` "calculatorHelp" ` | ` "^#*(星铁)?角色(养成\|计算\|养成计算)$" ` | 由上层或处理函数决定 |
| [apps/calculator.js:23](https://github.com/TimeRainStarSky/Yunzai-genshin/blob/4a2e1fb8f094b2a8f039ce0768773701718b60b9/apps/calculator.js#L23) | ` "blueprintHelp" ` | ` "^#*尘歌壶模数(养成\|计算\|养成计算)$" ` | 由上层或处理函数决定 |
| [apps/calculator.js:27](https://github.com/TimeRainStarSky/Yunzai-genshin/blob/4a2e1fb8f094b2a8f039ce0768773701718b60b9/apps/calculator.js#L27) | ` "Blueprint" ` | ` "^#*尘歌壶(模数\|养成\|养成计算)(\\d{10,15})$" ` | 由上层或处理函数决定 |
| [apps/dailyNote.js:16](https://github.com/TimeRainStarSky/Yunzai-genshin/blob/4a2e1fb8f094b2a8f039ce0768773701718b60b9/apps/dailyNote.js#L16) | ` "note" ` | ` "^#*(原神\|星铁)?(体力\|树脂\|查询体力)$" ` | 由上层或处理函数决定 |
| [apps/exchange.js:14](https://github.com/TimeRainStarSky/Yunzai-genshin/blob/4a2e1fb8f094b2a8f039ce0768773701718b60b9/apps/exchange.js#L14) | ` "getCode" ` | ` /^(#\|\*)?(原神\|星铁\|崩铁\|崩三\|崩坏三\|崩坏3\|绝区零)?(直播\|前瞻)?兑换码$/ ` | 由上层或处理函数决定 |
| [apps/exchange.js:18](https://github.com/TimeRainStarSky/Yunzai-genshin/blob/4a2e1fb8f094b2a8f039ce0768773701718b60b9/apps/exchange.js#L18) | ` "useCode" ` | ` "^#(原神\|星铁\|绝区零)?(兑换码使用\|cdk-u).+" ` | 由上层或处理函数决定 |
| [apps/gacha.js:15](https://github.com/TimeRainStarSky/Yunzai-genshin/blob/4a2e1fb8f094b2a8f039ce0768773701718b60b9/apps/gacha.js#L15) | ` "gacha" ` | ` "^#*(10\|[武器池常驻]*[十]+\|抽\|单)[连抽卡奖][123武器池常驻]*$" ` | 由上层或处理函数决定 |
| [apps/gacha.js:19](https://github.com/TimeRainStarSky/Yunzai-genshin/blob/4a2e1fb8f094b2a8f039ce0768773701718b60b9/apps/gacha.js#L19) | ` "weaponBing" ` | ` "(^#*定轨\|^#定轨(.*))$" ` | 由上层或处理函数决定 |
| [apps/gcLog.js:17](https://github.com/TimeRainStarSky/Yunzai-genshin/blob/4a2e1fb8f094b2a8f039ce0768773701718b60b9/apps/gcLog.js#L17) | ` "logUrl" ` | ` "(.*)authkey=(.*)" ` | 由上层或处理函数决定 |
| [apps/gcLog.js:21](https://github.com/TimeRainStarSky/Yunzai-genshin/blob/4a2e1fb8f094b2a8f039ce0768773701718b60b9/apps/gcLog.js#L21) | ` "logJson" ` | ` "^#?(原神\|星铁)?(强制)?导入记录(json)?$" ` | 由上层或处理函数决定 |
| [apps/gcLog.js:25](https://github.com/TimeRainStarSky/Yunzai-genshin/blob/4a2e1fb8f094b2a8f039ce0768773701718b60b9/apps/gcLog.js#L25) | ` "getLog" ` | ` "^#?(原神\|星铁)?(全部)?(抽卡\|抽奖\|角色\|角色联动\|武器\|武器联动\|集录\|常驻\|up\|新手\|光锥\|光锥联动\|全部)池*(记录\|祈愿\|分析)$" ` | 由上层或处理函数决定 |
| [apps/gcLog.js:29](https://github.com/TimeRainStarSky/Yunzai-genshin/blob/4a2e1fb8f094b2a8f039ce0768773701718b60b9/apps/gcLog.js#L29) | ` "exportLog" ` | ` "^#?(原神\|星铁)?(强制)?导出记录(json)?(v2\|v4)?$" ` | 由上层或处理函数决定 |
| [apps/gcLog.js:33](https://github.com/TimeRainStarSky/Yunzai-genshin/blob/4a2e1fb8f094b2a8f039ce0768773701718b60b9/apps/gcLog.js#L33) | ` "help" ` | ` "^#?(记录帮助\|抽卡帮助)$" ` | 由上层或处理函数决定 |
| [apps/gcLog.js:37](https://github.com/TimeRainStarSky/Yunzai-genshin/blob/4a2e1fb8f094b2a8f039ce0768773701718b60b9/apps/gcLog.js#L37) | ` "helpPort" ` | ` "^#?(安卓\|苹果\|电脑\|pc\|ios)帮助$" ` | 由上层或处理函数决定 |
| [apps/gcLog.js:41](https://github.com/TimeRainStarSky/Yunzai-genshin/blob/4a2e1fb8f094b2a8f039ce0768773701718b60b9/apps/gcLog.js#L41) | ` "logCount" ` | ` "^#?(原神\|星铁)?(抽卡\|抽奖\|角色\|武器\|集录\|常驻\|up\|新手\|光锥)池*统计$" ` | 由上层或处理函数决定 |
| [apps/gcLog.js:45](https://github.com/TimeRainStarSky/Yunzai-genshin/blob/4a2e1fb8f094b2a8f039ce0768773701718b60b9/apps/gcLog.js#L45) | ` "setFetchFullLog" ` | ` "^#?设置全量(更新\|获取)(抽卡\|祈愿)记录\s*(开\|关\|on\|off)?$" ` | 由上层或处理函数决定 |
| [apps/ledger.js:14](https://github.com/TimeRainStarSky/Yunzai-genshin/blob/4a2e1fb8f094b2a8f039ce0768773701718b60b9/apps/ledger.js#L14) | ` "ledger" ` | ` "^(#原石\|#*札记\|#*(星铁)?星琼)([0-9]\|[一二两三四五六七八九十]+)*月*$" ` | 由上层或处理函数决定 |
| [apps/ledger.js:18](https://github.com/TimeRainStarSky/Yunzai-genshin/blob/4a2e1fb8f094b2a8f039ce0768773701718b60b9/apps/ledger.js#L18) | ` "ledgerTask" ` | ` "^#(原石\|(星铁)?星琼)任务$" ` | ` "master" ` |
| [apps/ledger.js:23](https://github.com/TimeRainStarSky/Yunzai-genshin/blob/4a2e1fb8f094b2a8f039ce0768773701718b60b9/apps/ledger.js#L23) | ` "ledgerCount" ` | ` "^#*(原石\|札记\|(星铁)?星琼)统计$" ` | 由上层或处理函数决定 |
| [apps/ledger.js:27](https://github.com/TimeRainStarSky/Yunzai-genshin/blob/4a2e1fb8f094b2a8f039ce0768773701718b60b9/apps/ledger.js#L27) | ` "ledgerCountHistory" ` | ` "^#*(去年\|今年\|\\d{4}年)(原石\|札记\|(星铁)?星琼)统计$" ` | 由上层或处理函数决定 |
| [apps/material.js:15](https://github.com/TimeRainStarSky/Yunzai-genshin/blob/4a2e1fb8f094b2a8f039ce0768773701718b60b9/apps/material.js#L15) | ` "material" ` | ` "^#?(星铁)?(.*)(突破\|材料\|素材\|培养)$" ` | 由上层或处理函数决定 |
| [apps/mysNews.js:19](https://github.com/TimeRainStarSky/Yunzai-genshin/blob/4a2e1fb8f094b2a8f039ce0768773701718b60b9/apps/mysNews.js#L19) | ` "news" ` | ` "^#*(官方\|星铁\|原神\|崩坏三\|崩三\|绝区零\|崩坏二\|崩二\|崩坏学园二\|未定\|未定事件簿)?(公告\|资讯\|活动)(列表\|[0-9])*$" ` | 由上层或处理函数决定 |
| [apps/mysNews.js:23](https://github.com/TimeRainStarSky/Yunzai-genshin/blob/4a2e1fb8f094b2a8f039ce0768773701718b60b9/apps/mysNews.js#L23) | ` "mysSearch" ` | ` "^(#米游社\|#mys)(.*)" ` | 由上层或处理函数决定 |
| [apps/mysNews.js:27](https://github.com/TimeRainStarSky/Yunzai-genshin/blob/4a2e1fb8f094b2a8f039ce0768773701718b60b9/apps/mysNews.js#L27) | ` "mysUrl" ` | ` "(.*)(bbs.mihoyo.com\|miyoushe.com)/ys(.*)/article(.*)" ` | 由上层或处理函数决定 |
| [apps/mysNews.js:31](https://github.com/TimeRainStarSky/Yunzai-genshin/blob/4a2e1fb8f094b2a8f039ce0768773701718b60b9/apps/mysNews.js#L31) | ` "mysEstimate" ` | ` "^#?(原(神\|石)\|星(铁\|琼)\|崩坏三\|崩三\|水晶\|绝区(零)\|zzz\|菲林)?(预估\|盘点)$" ` | 由上层或处理函数决定 |
| [apps/mysNews.js:35](https://github.com/TimeRainStarSky/Yunzai-genshin/blob/4a2e1fb8f094b2a8f039ce0768773701718b60b9/apps/mysNews.js#L35) | ` "setPush" ` | ` "^#*(星铁\|原神\|崩坏三\|崩三\|绝区零\|崩坏二\|崩二\|崩坏学园二\|未定\|未定事件簿)?(开启\|关闭)(公告\|资讯)推送$" ` | 由上层或处理函数决定 |
| [apps/mysNews.js:39](https://github.com/TimeRainStarSky/Yunzai-genshin/blob/4a2e1fb8f094b2a8f039ce0768773701718b60b9/apps/mysNews.js#L39) | ` "mysNewsTask" ` | ` "^#(星铁\|原神\|崩坏三\|崩三\|绝区零\|崩坏二\|崩二\|崩坏学园二\|未定\|未定事件簿)?推送(公告\|资讯)$" ` | ` "master" ` |
| [apps/mysNews.js:44](https://github.com/TimeRainStarSky/Yunzai-genshin/blob/4a2e1fb8f094b2a8f039ce0768773701718b60b9/apps/mysNews.js#L44) | ` "setActivityPush" ` | ` "^#(星铁\|原神)(开启\|关闭)到期活动(预警)?(推送)?$" ` | 由上层或处理函数决定 |
| [apps/noteZzz.js:12](https://github.com/TimeRainStarSky/Yunzai-genshin/blob/4a2e1fb8f094b2a8f039ce0768773701718b60b9/apps/noteZzz.js#L12) | ` "note" ` | ` "^#*绝区零?(体力\|树脂\|查询体力)$" ` | 由上层或处理函数决定 |
| [apps/payLog.js:20](https://github.com/TimeRainStarSky/Yunzai-genshin/blob/4a2e1fb8f094b2a8f039ce0768773701718b60b9/apps/payLog.js#L20) | ` "payLog" ` | ` "^#?(充值\|消费)(记录\|统计)$" ` | 由上层或处理函数决定 |
| [apps/payLog.js:24](https://github.com/TimeRainStarSky/Yunzai-genshin/blob/4a2e1fb8f094b2a8f039ce0768773701718b60b9/apps/payLog.js#L24) | ` "updatePayLog" ` | ` "^#?更新(充值\|消费)(记录\|统计)$" ` | 由上层或处理函数决定 |
| [apps/payLog.js:28](https://github.com/TimeRainStarSky/Yunzai-genshin/blob/4a2e1fb8f094b2a8f039ce0768773701718b60b9/apps/payLog.js#L28) | ` "getAuthKey" ` | ` "(.*)(user-game-search\|bill-record-user\|customer-claim\|player-log\|user.mihoyo.com)(.*)" ` | 由上层或处理函数决定 |
| [apps/payLog.js:33](https://github.com/TimeRainStarSky/Yunzai-genshin/blob/4a2e1fb8f094b2a8f039ce0768773701718b60b9/apps/payLog.js#L33) | ` "payLogHelp" ` | ` "^#?(充值\|消费)(记录\|统计)帮助$" ` | 由上层或处理函数决定 |
| [apps/role.js:16](https://github.com/TimeRainStarSky/Yunzai-genshin/blob/4a2e1fb8f094b2a8f039ce0768773701718b60b9/apps/role.js#L16) | ` "roleCard" ` | ` "^(#*角色3\|#*角色卡片\|角色)$" ` | 由上层或处理函数决定 |
| [apps/role.js:20](https://github.com/TimeRainStarSky/Yunzai-genshin/blob/4a2e1fb8f094b2a8f039ce0768773701718b60b9/apps/role.js#L20) | ` "abyss" ` | ` "^#[上期\|往期\|本期]*(深渊\|深境\|深境螺旋)[上期\|往期\|本期]*[ \|0-9]*$" ` | 由上层或处理函数决定 |
| [apps/role.js:24](https://github.com/TimeRainStarSky/Yunzai-genshin/blob/4a2e1fb8f094b2a8f039ce0768773701718b60b9/apps/role.js#L24) | ` "abyssFloor" ` | ` "^#*[上期\|往期\|本期]*(深渊\|深境\|深境螺旋)[上期\|往期\|本期]*[第]*(9\|10\|11\|12\|九\|十\|十一\|十二)层[ \|0-9]*$" ` | 由上层或处理函数决定 |
| [apps/role.js:28](https://github.com/TimeRainStarSky/Yunzai-genshin/blob/4a2e1fb8f094b2a8f039ce0768773701718b60b9/apps/role.js#L28) | ` "weapon" ` | ` "^#[五星\|四星\|5星\|4星]*武器[ \|0-9]*$" ` | 由上层或处理函数决定 |
| [apps/role.js:32](https://github.com/TimeRainStarSky/Yunzai-genshin/blob/4a2e1fb8f094b2a8f039ce0768773701718b60b9/apps/role.js#L32) | ` "roleExplore" ` | ` "^#(宝箱\|成就\|尘歌壶\|家园\|探索\|探险\|声望\|探险度\|探索度)[ \|0-9]*$" ` | 由上层或处理函数决定 |
| [apps/role.js:36](https://github.com/TimeRainStarSky/Yunzai-genshin/blob/4a2e1fb8f094b2a8f039ce0768773701718b60b9/apps/role.js#L36) | ` "combat" ` | ` "^#(幻想真境剧诗\|剧诗)$" ` | 由上层或处理函数决定 |
| [apps/setPubCk.js:17](https://github.com/TimeRainStarSky/Yunzai-genshin/blob/4a2e1fb8f094b2a8f039ce0768773701718b60b9/apps/setPubCk.js#L17) | ` "setPubCk" ` | ` /^#配置c(oo)?k(ie)?$\|^#*配置公共查询c(oo)?k(ie)?$/i ` | ` "master" ` |
| [apps/setPubCk.js:22](https://github.com/TimeRainStarSky/Yunzai-genshin/blob/4a2e1fb8f094b2a8f039ce0768773701718b60b9/apps/setPubCk.js#L22) | ` "setUserCk" ` | ` "^#使用(全部\|用户)ck$" ` | ` "master" ` |
| [apps/sevenSaints.js:13](https://github.com/TimeRainStarSky/Yunzai-genshin/blob/4a2e1fb8f094b2a8f039ce0768773701718b60b9/apps/sevenSaints.js#L13) | ` "deckIndex" ` | ` "^#*七圣(召唤)?查询(牌\|卡)组(列表)?[0-9]{0,2}$" ` | 由上层或处理函数决定 |
| [apps/sevenSaints.js:17](https://github.com/TimeRainStarSky/Yunzai-genshin/blob/4a2e1fb8f094b2a8f039ce0768773701718b60b9/apps/sevenSaints.js#L17) | ` "deck_cards" ` | ` "^#*七圣(召唤)?查询(角色\|行动)?(卡)?牌(列表)?$" ` | 由上层或处理函数决定 |
| [apps/strategy.js:29](https://github.com/TimeRainStarSky/Yunzai-genshin/blob/4a2e1fb8f094b2a8f039ce0768773701718b60b9/apps/strategy.js#L29) | ` "strategy" ` | ` "^#?(更新)?\\S+攻略([1-7])?$" ` | 由上层或处理函数决定 |
| [apps/strategy.js:33](https://github.com/TimeRainStarSky/Yunzai-genshin/blob/4a2e1fb8f094b2a8f039ce0768773701718b60b9/apps/strategy.js#L33) | ` "strategy_help" ` | ` "^#?攻略(说明\|帮助)?$" ` | 由上层或处理函数决定 |
| [apps/strategy.js:37](https://github.com/TimeRainStarSky/Yunzai-genshin/blob/4a2e1fb8f094b2a8f039ce0768773701718b60b9/apps/strategy.js#L37) | ` "strategy_setting" ` | ` "^#?设置默认攻略([1-7])?$" ` | 由上层或处理函数决定 |
| [apps/takeBirthdayPhoto.js:13](https://github.com/TimeRainStarSky/Yunzai-genshin/blob/4a2e1fb8f094b2a8f039ce0768773701718b60b9/apps/takeBirthdayPhoto.js#L13) | ` "birthdaystar" ` | ` "^#?(留影(叙佳期)?\|((领)?((角色)?生日)(卡)?))$" ` | 由上层或处理函数决定 |
| [apps/user.js:14](https://github.com/TimeRainStarSky/Yunzai-genshin/blob/4a2e1fb8f094b2a8f039ce0768773701718b60b9/apps/user.js#L14) | ` "ckHelp" ` | ` "^#?(体力\|[Cc](oo)?[Kk](ie)?)帮助" ` | 由上层或处理函数决定 |
| [apps/user.js:18](https://github.com/TimeRainStarSky/Yunzai-genshin/blob/4a2e1fb8f094b2a8f039ce0768773701718b60b9/apps/user.js#L18) | ` "ckCode" ` | ` "^#[Cc](oo)?[Kk](ie)?代码$" ` | 由上层或处理函数决定 |
| [apps/user.js:22](https://github.com/TimeRainStarSky/Yunzai-genshin/blob/4a2e1fb8f094b2a8f039ce0768773701718b60b9/apps/user.js#L22) | ` "bingCk" ` | ` /^#绑定c(oo)?k(ie)?$/i ` | 由上层或处理函数决定 |
| [apps/user.js:26](https://github.com/TimeRainStarSky/Yunzai-genshin/blob/4a2e1fb8f094b2a8f039ce0768773701718b60b9/apps/user.js#L26) | ` "noLogin" ` | ` "(.*)_MHYUUID(.*)" ` | 由上层或处理函数决定 |
| [apps/user.js:31](https://github.com/TimeRainStarSky/Yunzai-genshin/blob/4a2e1fb8f094b2a8f039ce0768773701718b60b9/apps/user.js#L31) | ` "myCk" ` | ` /^#?(原神\|星铁\|绝区零)?我的c(oo)?k(ie)?$/i ` | 由上层或处理函数决定 |
| [apps/user.js:36](https://github.com/TimeRainStarSky/Yunzai-genshin/blob/4a2e1fb8f094b2a8f039ce0768773701718b60b9/apps/user.js#L36) | ` "delCk" ` | ` /^#?(原神\|星铁\|绝区零)?删除c(oo)?k(ie)?$/i ` | 由上层或处理函数决定 |
| [apps/user.js:40](https://github.com/TimeRainStarSky/Yunzai-genshin/blob/4a2e1fb8f094b2a8f039ce0768773701718b60b9/apps/user.js#L40) | ` "delUid" ` | ` /^#?(原神\|星铁\|绝区零)?(删除\|解绑)uid(\s\|\+)*([0-9]{1,2})?$/i ` | 由上层或处理函数决定 |
| [apps/user.js:44](https://github.com/TimeRainStarSky/Yunzai-genshin/blob/4a2e1fb8f094b2a8f039ce0768773701718b60b9/apps/user.js#L44) | ` "bingUid" ` | ` /^#(原神\|星铁\|绝区零)?绑定(uid)?(\s\|\+)*((1[0-9]\|[1-9])[0-9]{8}\|[1-9][0-9]{7})$/i ` | 由上层或处理函数决定 |
| [apps/user.js:48](https://github.com/TimeRainStarSky/Yunzai-genshin/blob/4a2e1fb8f094b2a8f039ce0768773701718b60b9/apps/user.js#L48) | ` "showUid" ` | ` /^#(原神\|星铁\|绝区零)?(我的)?(uid)[0-9]{0,2}$/i ` | 由上层或处理函数决定 |
| [apps/user.js:52](https://github.com/TimeRainStarSky/Yunzai-genshin/blob/4a2e1fb8f094b2a8f039ce0768773701718b60b9/apps/user.js#L52) | ` "checkCkStatus" ` | ` /^#\\s*(检查\|我的)*c(oo)?k(ie)?(状态)*$/i ` | 由上层或处理函数决定 |
| [apps/user.js:56](https://github.com/TimeRainStarSky/Yunzai-genshin/blob/4a2e1fb8f094b2a8f039ce0768773701718b60b9/apps/user.js#L56) | ` "bindNoteUser" ` | ` "^#(接受)?绑定(主\|子)?(用户\|账户\|账号)(\\[[a-zA-Z0-9_\\-:\\]+\\]){0,2}$" ` | 由上层或处理函数决定 |
| [apps/user.js:60](https://github.com/TimeRainStarSky/Yunzai-genshin/blob/4a2e1fb8f094b2a8f039ce0768773701718b60b9/apps/user.js#L60) | ` "bindNoteUser" ` | ` "^#(删除绑定\|取消绑定\|解除绑定\|解绑\|删除\|取消)(主\|子)(用户\|账户\|账号)$" ` | 由上层或处理函数决定 |
| [apps/userAdmin.js:14](https://github.com/TimeRainStarSky/Yunzai-genshin/blob/4a2e1fb8f094b2a8f039ce0768773701718b60b9/apps/userAdmin.js#L14) | ` "userAdmin" ` | ` "^#用户统计$" ` | ` "master" ` |
| [apps/userAdmin.js:19](https://github.com/TimeRainStarSky/Yunzai-genshin/blob/4a2e1fb8f094b2a8f039ce0768773701718b60b9/apps/userAdmin.js#L19) | ` "resetCache" ` | ` "^#(刷新\|重置)用户(缓存\|统计\|ck\|Ck\|CK)$" ` | ` "master" ` |
| [apps/userAdmin.js:24](https://github.com/TimeRainStarSky/Yunzai-genshin/blob/4a2e1fb8f094b2a8f039ce0768773701718b60b9/apps/userAdmin.js#L24) | ` "delDisable" ` | ` "^#删除(无效\|失效)(用户\|ck\|Ck\|CK)$" ` | ` "master" ` |

## ark-plugin

固定提交：`b7d8231ddcf606ce4f8d20e9bd743970e4f18cea`。静态声明：32。

| 源文件与行 | 标签或处理函数 | 规则表达式 | 参考权限 |
| --- | --- | --- | --- |
| [apps/admin.js:18](https://github.com/NotIvny/ark-plugin/blob/b7d8231ddcf606ce4f8d20e9bd743970e4f18cea/apps/admin.js#L18) | ` 'sysCfg' ` | ` '^#ark设置(.*)$' ` | 由上层或处理函数决定 |
| [apps/customRank.js:112](https://github.com/NotIvny/ark-plugin/blob/b7d8231ddcf606ce4f8d20e9bd743970e4f18cea/apps/customRank.js#L112) | ` 'rankHelp' ` | ` /^#ark自定义排行帮助$/ ` | 由上层或处理函数决定 |
| [apps/customRank.js:113](https://github.com/NotIvny/ark-plugin/blob/b7d8231ddcf606ce4f8d20e9bd743970e4f18cea/apps/customRank.js#L113) | ` 'rank' ` | ` /^#ark.+(伤害\|圣遗物)?(排行\|排名)(\s.*)?$/ ` | 由上层或处理函数决定 |
| [apps/customRankPanel.js:17](https://github.com/NotIvny/ark-plugin/blob/b7d8231ddcf606ce4f8d20e9bd743970e4f18cea/apps/customRankPanel.js#L17) | ` 'getCustomRankPanel' ` | ` /^#ark获取面板\s*([1-9]\|1[0-9]\|20)$/i ` | 由上层或处理函数决定 |
| [apps/help.js:17](https://github.com/NotIvny/ark-plugin/blob/b7d8231ddcf606ce4f8d20e9bd743970e4f18cea/apps/help.js#L17) | ` 'help' ` | ` '^#ark帮助$' ` | 由上层或处理函数决定 |
| [apps/help.js:21](https://github.com/NotIvny/ark-plugin/blob/b7d8231ddcf606ce4f8d20e9bd743970e4f18cea/apps/help.js#L21) | ` 'version' ` | ` '^#ark版本$' ` | 由上层或处理函数决定 |
| [apps/miaoGroupConfig.js:14](https://github.com/NotIvny/ark-plugin/blob/b7d8231ddcf606ce4f8d20e9bd743970e4f18cea/apps/miaoGroupConfig.js#L14) | ` 'groupConfig' ` | ` '^(.*)$' ` | 由上层或处理函数决定 |
| [apps/priority.js:11](https://github.com/NotIvny/ark-plugin/blob/b7d8231ddcf606ce4f8d20e9bd743970e4f18cea/apps/priority.js#L11) | ` 'refreshPriority' ` | ` '^#刷新优先级(.*)$' ` | 由上层或处理函数决定 |
| [apps/priority.js:15](https://github.com/NotIvny/ark-plugin/blob/b7d8231ddcf606ce4f8d20e9bd743970e4f18cea/apps/priority.js#L15) | ` 'changePriority' ` | ` '#修改优先级$' ` | 由上层或处理函数决定 |
| [apps/priority.js:19](https://github.com/NotIvny/ark-plugin/blob/b7d8231ddcf606ce4f8d20e9bd743970e4f18cea/apps/priority.js#L19) | ` 'resetPriority' ` | ` '#重置优先级$' ` | 由上层或处理函数决定 |
| [apps/priority.js:23](https://github.com/NotIvny/ark-plugin/blob/b7d8231ddcf606ce4f8d20e9bd743970e4f18cea/apps/priority.js#L23) | ` 'getPriority' ` | ` '#查询优先级$' ` | 由上层或处理函数决定 |
| [apps/priority.js:27](https://github.com/NotIvny/ark-plugin/blob/b7d8231ddcf606ce4f8d20e9bd743970e4f18cea/apps/priority.js#L27) | ` 'firstRefresh' ` | ` '(.*)$' ` | 由上层或处理函数决定 |
| [apps/replaceFile.js:17](https://github.com/NotIvny/ark-plugin/blob/b7d8231ddcf606ce4f8d20e9bd743970e4f18cea/apps/replaceFile.js#L17) | ` 'arkCreateBackup' ` | ` '^#ark创建备份$' ` | ` 'master' ` |
| [apps/replaceFile.js:18](https://github.com/NotIvny/ark-plugin/blob/b7d8231ddcf606ce4f8d20e9bd743970e4f18cea/apps/replaceFile.js#L18) | ` 'arkRemoveBackup' ` | ` '^#ark删除备份$' ` | ` 'master' ` |
| [apps/replaceFile.js:19](https://github.com/NotIvny/ark-plugin/blob/b7d8231ddcf606ce4f8d20e9bd743970e4f18cea/apps/replaceFile.js#L19) | ` 'arkRecoverFile' ` | ` '^#ark恢复文件(.*)$' ` | ` 'master' ` |
| [apps/replaceFile.js:20](https://github.com/NotIvny/ark-plugin/blob/b7d8231ddcf606ce4f8d20e9bd743970e4f18cea/apps/replaceFile.js#L20) | ` 'arkReplaceFile' ` | ` '^#ark替换文件(.*)$' ` | ` 'master' ` |
| [apps/replaceFile.js:21](https://github.com/NotIvny/ark-plugin/blob/b7d8231ddcf606ce4f8d20e9bd743970e4f18cea/apps/replaceFile.js#L21) | ` 'arkBackupFile' ` | ` '^#ark备份文件(.*)$' ` | ` 'master' ` |
| [apps/token.js:8](https://github.com/NotIvny/ark-plugin/blob/b7d8231ddcf606ce4f8d20e9bd743970e4f18cea/apps/token.js#L8) | ` 'setToken' ` | ` /^#ark配置token\s*([\s\S]+)$/i ` | 由上层或处理函数决定 |
| [apps/update.js:15](https://github.com/NotIvny/ark-plugin/blob/b7d8231ddcf606ce4f8d20e9bd743970e4f18cea/apps/update.js#L15) | ` 'branchList' ` | ` '^#ark分支列表$' ` | ` 'master' ` |
| [apps/update.js:20](https://github.com/NotIvny/ark-plugin/blob/b7d8231ddcf606ce4f8d20e9bd743970e4f18cea/apps/update.js#L20) | ` 'switchBranch' ` | ` '^#ark切换分支\\s*(.*)$' ` | ` 'master' ` |
| [apps/usage.js:38](https://github.com/NotIvny/ark-plugin/blob/b7d8231ddcf606ce4f8d20e9bd743970e4f18cea/apps/usage.js#L38) | ` 'usage' ` | ` /^#arktoken用量$/i ` | 由上层或处理函数决定 |
| [apps/user.js:101](https://github.com/NotIvny/ark-plugin/blob/b7d8231ddcf606ce4f8d20e9bd743970e4f18cea/apps/user.js#L101) | ` 'getSpecificRank' ` | ` '^#(.*)排名统计$' ` | 由上层或处理函数决定 |
| [apps/user.js:105](https://github.com/NotIvny/ark-plugin/blob/b7d8231ddcf606ce4f8d20e9bd743970e4f18cea/apps/user.js#L105) | ` 'uploadPanelData' ` | ` '^#(星铁\|原神)?(导出面板数据)(.*)' ` | 由上层或处理函数决定 |
| [apps/user.js:109](https://github.com/NotIvny/ark-plugin/blob/b7d8231ddcf606ce4f8d20e9bd743970e4f18cea/apps/user.js#L109) | ` 'arkReforgeRecog' ` | ` '^#(星铁\|原神)?ark(重塑\|重投)识别$' ` | 由上层或处理函数决定 |
| [apps/user.js:113](https://github.com/NotIvny/ark-plugin/blob/b7d8231ddcf606ce4f8d20e9bd743970e4f18cea/apps/user.js#L113) | ` 'downloadPanelData' ` | ` '#(星铁\|原神)?(导入面板数据)(.*)' ` | 由上层或处理函数决定 |
| [apps/user.js:117](https://github.com/NotIvny/ark-plugin/blob/b7d8231ddcf606ce4f8d20e9bd743970e4f18cea/apps/user.js#L117) | ` 'getRank' ` | ` '^#角色排名(.*)$' ` | 由上层或处理函数决定 |
| [apps/user.js:121](https://github.com/NotIvny/ark-plugin/blob/b7d8231ddcf606ce4f8d20e9bd743970e4f18cea/apps/user.js#L121) | ` 'getAllRank' ` | ` '^#(星铁\|原神)?总排名(.*)$' ` | 由上层或处理函数决定 |
| [apps/user.js:125](https://github.com/NotIvny/ark-plugin/blob/b7d8231ddcf606ce4f8d20e9bd743970e4f18cea/apps/user.js#L125) | ` 'stygian' ` | ` '^#(top)?幽境危战排名(\\d+\.\\d+)?$' ` | 由上层或处理函数决定 |
| [apps/user.js:129](https://github.com/NotIvny/ark-plugin/blob/b7d8231ddcf606ce4f8d20e9bd743970e4f18cea/apps/user.js#L129) | ` 'refreshPanel' ` | ` /^#(星铁\|原神)?(全部面板更新\|更新全部面板\|获取游戏角色详情\|更新面板\|面板更新)\s*(\d{9,10})?$/ ` | 由上层或处理函数决定 |
| [apps/user.js:133](https://github.com/NotIvny/ark-plugin/blob/b7d8231ddcf606ce4f8d20e9bd743970e4f18cea/apps/user.js#L133) | ` 'arkGetBindUid' ` | ` /^#ark绑定(星铁\|原神)uid$/ ` | 由上层或处理函数决定 |
| [apps/user.js:137](https://github.com/NotIvny/ark-plugin/blob/b7d8231ddcf606ce4f8d20e9bd743970e4f18cea/apps/user.js#L137) | ` 'arkBindUid' ` | ` /^#ark验证(星铁\|原神)uid$/ ` | 由上层或处理函数决定 |
| [apps/user.js:141](https://github.com/NotIvny/ark-plugin/blob/b7d8231ddcf606ce4f8d20e9bd743970e4f18cea/apps/user.js#L141) | ` 'exportPanel' ` | ` /^#(星铁\|原神)导出面板$/ ` | 由上层或处理函数决定 |

无静态命令对的顶层模块（仍需覆盖）：[apps/HelpTheme.js](https://github.com/NotIvny/ark-plugin/blob/b7d8231ddcf606ce4f8d20e9bd743970e4f18cea/apps/HelpTheme.js)、[apps/miaoHelp.js](https://github.com/NotIvny/ark-plugin/blob/b7d8231ddcf606ce4f8d20e9bd743970e4f18cea/apps/miaoHelp.js)。

## Axiu-Plugin

固定提交：`6f336229dce0a22344e37107a738717d0860f6b7`。静态声明：37。

| 源文件与行 | 标签或处理函数 | 规则表达式 | 参考权限 |
| --- | --- | --- | --- |
| [apps/challenge.js:35](https://github.com/AxiuCN/Axiu-Plugin/blob/6f336229dce0a22344e37107a738717d0860f6b7/apps/challenge.js#L35) | ` 'challengeBoss' ` | ` '^(#?星铁\|[*＊])?(上期\|本期)?(简易)?(末日\|末日幻影)$' ` | 由上层或处理函数决定 |
| [apps/challenge.js:39](https://github.com/AxiuCN/Axiu-Plugin/blob/6f336229dce0a22344e37107a738717d0860f6b7/apps/challenge.js#L39) | ` 'challengeStory' ` | ` '^(#?星铁\|[*＊])?(上期\|本期)?(简易)?(虚构\|虚构叙事)$' ` | 由上层或处理函数决定 |
| [apps/challenge.js:43](https://github.com/AxiuCN/Axiu-Plugin/blob/6f336229dce0a22344e37107a738717d0860f6b7/apps/challenge.js#L43) | ` 'challengeForgottenHall' ` | ` '^(#?星铁\|[*＊])?(上期\|本期)?(简易)?(忘却\|忘却之庭\|混沌\|混沌回忆)$' ` | 由上层或处理函数决定 |
| [apps/challenge.js:47](https://github.com/AxiuCN/Axiu-Plugin/blob/6f336229dce0a22344e37107a738717d0860f6b7/apps/challenge.js#L47) | ` 'challengePeak' ` | ` '^(#?星铁\|[*＊])?(往期\|上期\|本期)?(简易)?(异乡\|异相\|异向\|仲裁\|异相仲裁)$' ` | 由上层或处理函数决定 |
| [apps/challenge.js:51](https://github.com/AxiuCN/Axiu-Plugin/blob/6f336229dce0a22344e37107a738717d0860f6b7/apps/challenge.js#L51) | ` 'challenge' ` | ` '^(#?星铁\|[*＊])?(上期\|本期)?(简易)?(深渊)$' ` | 由上层或处理函数决定 |
| [apps/challenge.js:55](https://github.com/AxiuCN/Axiu-Plugin/blob/6f336229dce0a22344e37107a738717d0860f6b7/apps/challenge.js#L55) | ` 'challengeCurrent' ` | ` '^(#?星铁\|[*＊])?(最新\|当期)(简易)?(深渊)$' ` | 由上层或处理函数决定 |
| [apps/challenge.js:61](https://github.com/AxiuCN/Axiu-Plugin/blob/6f336229dce0a22344e37107a738717d0860f6b7/apps/challenge.js#L61) | ` 'challengeRank' ` | ` '^(#?星铁\|[*＊])?(末日幻影\|末日\|虚构叙事\|虚构\|叙事\|忘却之庭\|忘却\|混沌回忆\|混沌\|异相仲裁\|异相\|仲裁\|异乡)(排名\|排行)' ` | 由上层或处理函数决定 |
| [apps/challenge.js:65](https://github.com/AxiuCN/Axiu-Plugin/blob/6f336229dce0a22344e37107a738717d0860f6b7/apps/challenge.js#L65) | ` 'challengeRankReset' ` | ` '^(#?星铁\|[*＊])?重置(末日幻影\|末日\|虚构叙事\|虚构\|叙事\|忘却之庭\|忘却\|混沌回忆\|混沌\|异相仲裁\|异相\|仲裁\|异乡)(排名\|排行)' ` | ` 'master' ` |
| [apps/challenge.js:70](https://github.com/AxiuCN/Axiu-Plugin/blob/6f336229dce0a22344e37107a738717d0860f6b7/apps/challenge.js#L70) | ` 'challengeRankManage' ` | ` '^(#?星铁\|[*＊])?(开启\|关闭)(挑战)(排名\|排行)' ` | ` 'master' ` |
| [apps/challenge.js:75](https://github.com/AxiuCN/Axiu-Plugin/blob/6f336229dce0a22344e37107a738717d0860f6b7/apps/challenge.js#L75) | ` 'challengeRankRebuild' ` | ` '^(#?星铁\|[*＊])?刷新(末日幻影\|末日\|虚构叙事\|虚构\|叙事\|忘却之庭\|忘却\|混沌回忆\|混沌\|异相仲裁\|异相\|仲裁\|异乡)?(排名\|排行)' ` | ` 'master' ` |
| [apps/challenge.js:83](https://github.com/AxiuCN/Axiu-Plugin/blob/6f336229dce0a22344e37107a738717d0860f6b7/apps/challenge.js#L83) | ` 'gsSpiralAbyssQuery' ` | ` '^#(原神)?(上期\|本期)?(深境\|深渊\|深境螺旋)$' ` | 由上层或处理函数决定 |
| [apps/challenge.js:87](https://github.com/AxiuCN/Axiu-Plugin/blob/6f336229dce0a22344e37107a738717d0860f6b7/apps/challenge.js#L87) | ` 'gsRoleCombatQuery' ` | ` '^#(原神)?(上期\|本期)?(幻想\|剧诗\|幻想真境剧诗)$' ` | 由上层或处理函数决定 |
| [apps/challenge.js:91](https://github.com/AxiuCN/Axiu-Plugin/blob/6f336229dce0a22344e37107a738717d0860f6b7/apps/challenge.js#L91) | ` 'gsHardChallengeQuery' ` | ` '^#(原神)?(上期\|本期)?(幽境\|危战\|幽境危战)(单人\|单挑\|组队\|多人\|合作\|最佳\|数据)?$' ` | 由上层或处理函数决定 |
| [apps/challenge.js:97](https://github.com/AxiuCN/Axiu-Plugin/blob/6f336229dce0a22344e37107a738717d0860f6b7/apps/challenge.js#L97) | ` 'gsAbyssRank' ` | ` '^#(原神)?(深境螺旋\|深境\|深渊\|幻想真境剧诗\|幻想\|剧诗\|幽境危战\|幽境\|危战)(单人\|单挑\|多人\|组队\|合作)?(排名\|排行)' ` | 由上层或处理函数决定 |
| [apps/challenge.js:101](https://github.com/AxiuCN/Axiu-Plugin/blob/6f336229dce0a22344e37107a738717d0860f6b7/apps/challenge.js#L101) | ` 'gsAbyssRankReset' ` | ` '^#(原神)?重置(深境螺旋\|深境\|深渊\|幻想真境剧诗\|幻想\|剧诗\|幽境危战\|幽境\|危战)(单人\|单挑\|多人\|组队\|合作)?(排名\|排行)' ` | ` 'master' ` |
| [apps/challenge.js:106](https://github.com/AxiuCN/Axiu-Plugin/blob/6f336229dce0a22344e37107a738717d0860f6b7/apps/challenge.js#L106) | ` 'gsAbyssRankManage' ` | ` '^#(原神)?(开启\|关闭)(深渊)(排名\|排行)' ` | ` 'master' ` |
| [apps/challenge.js:111](https://github.com/AxiuCN/Axiu-Plugin/blob/6f336229dce0a22344e37107a738717d0860f6b7/apps/challenge.js#L111) | ` 'gsAbyssRankRebuild' ` | ` '^#(原神)?刷新(深境螺旋\|深境\|深渊\|幻想真境剧诗\|幻想\|剧诗\|幽境危战\|幽境\|危战)(单人\|单挑\|多人\|组队\|合作)?(排名\|排行)' ` | ` 'master' ` |
| [apps/gsGachaLog.js:34](https://github.com/AxiuCN/Axiu-Plugin/blob/6f336229dce0a22344e37107a738717d0860f6b7/apps/gsGachaLog.js#L34) | ` 'gachaLog' ` | ` '^#更新抽卡记录$' ` | ` 'all' ` |
| [apps/gsGachaLog.js:41](https://github.com/AxiuCN/Axiu-Plugin/blob/6f336229dce0a22344e37107a738717d0860f6b7/apps/gsGachaLog.js#L41) | ` 'gachaLogAssistant' ` | ` '^#?(获取\|更新)(提瓦特)?小助手(抽卡\|祈愿)?(记录\|历史)( *\|(\\r\|\\n)*)(https.*)?' ` | 由上层或处理函数决定 |
| [apps/gsGachaLog.js:46](https://github.com/AxiuCN/Axiu-Plugin/blob/6f336229dce0a22344e37107a738717d0860f6b7/apps/gsGachaLog.js#L46) | ` 'getGachaUrl' ` | ` '^#获取抽卡链接$' ` | ` 'all' ` |
| [apps/help.js:22](https://github.com/AxiuCN/Axiu-Plugin/blob/6f336229dce0a22344e37107a738717d0860f6b7/apps/help.js#L22) | ` 'help' ` | ` /^#(axiu\|Axiu\|阿修)(帮助\|菜单\|指令\|命令\|help)$/i ` | 由上层或处理函数决定 |
| [apps/mysSignin.js:52](https://github.com/AxiuCN/Axiu-Plugin/blob/6f336229dce0a22344e37107a738717d0860f6b7/apps/mysSignin.js#L52) | ` 'initEnv' ` | ` '^#初始化签到环境$' ` | ` 'master' ` |
| [apps/mysSignin.js:53](https://github.com/AxiuCN/Axiu-Plugin/blob/6f336229dce0a22344e37107a738717d0860f6b7/apps/mysSignin.js#L53) | ` 'register' ` | ` '^#注册自动签到$' ` | ` 'all' ` |
| [apps/mysSignin.js:54](https://github.com/AxiuCN/Axiu-Plugin/blob/6f336229dce0a22344e37107a738717d0860f6b7/apps/mysSignin.js#L54) | ` 'groupRegister' ` | ` '^#注册本群签到$' ` | ` 'all' ` |
| [apps/mysSignin.js:55](https://github.com/AxiuCN/Axiu-Plugin/blob/6f336229dce0a22344e37107a738717d0860f6b7/apps/mysSignin.js#L55) | ` 'registerAllCmd' ` | ` '^#注册所有群签到$' ` | ` 'master' ` |
| [apps/mysSignin.js:56](https://github.com/AxiuCN/Axiu-Plugin/blob/6f336229dce0a22344e37107a738717d0860f6b7/apps/mysSignin.js#L56) | ` 'listProfiles' ` | ` '^#签到名单列表$' ` | ` 'master' ` |
| [apps/mysSignin.js:57](https://github.com/AxiuCN/Axiu-Plugin/blob/6f336229dce0a22344e37107a738717d0860f6b7/apps/mysSignin.js#L57) | ` 'startSignin' ` | ` '^#(开始\|手动\|测试)?签到$' ` | ` 'all' ` |
| [apps/mysSignin.js:58](https://github.com/AxiuCN/Axiu-Plugin/blob/6f336229dce0a22344e37107a738717d0860f6b7/apps/mysSignin.js#L58) | ` 'signinAll' ` | ` '^#全部签到$' ` | ` 'master' ` |
| [apps/mysSignin.js:59](https://github.com/AxiuCN/Axiu-Plugin/blob/6f336229dce0a22344e37107a738717d0860f6b7/apps/mysSignin.js#L59) | ` 'refreshCookieCmd' ` | ` '^#刷新自动签到$' ` | ` 'all' ` |
| [apps/mysSignin.js:60](https://github.com/AxiuCN/Axiu-Plugin/blob/6f336229dce0a22344e37107a738717d0860f6b7/apps/mysSignin.js#L60) | ` 'signinStatus' ` | ` '^#签到状态$' ` | ` 'all' ` |
| [apps/mysSignin.js:61](https://github.com/AxiuCN/Axiu-Plugin/blob/6f336229dce0a22344e37107a738717d0860f6b7/apps/mysSignin.js#L61) | ` 'deleteSigninCmd' ` | ` '^#删除签到$' ` | ` 'all' ` |
| [apps/mysSignin.js:62](https://github.com/AxiuCN/Axiu-Plugin/blob/6f336229dce0a22344e37107a738717d0860f6b7/apps/mysSignin.js#L62) | ` 'deleteStokenCmd' ` | ` '^#删除stoken$' ` | ` 'all' ` |
| [apps/proxySpeak.js:13](https://github.com/AxiuCN/Axiu-Plugin/blob/6f336229dce0a22344e37107a738717d0860f6b7/apps/proxySpeak.js#L13) | ` 'proxySpeak' ` | ` /^#代/ ` | ` 'master' ` |
| [apps/qrLogin.js:23](https://github.com/AxiuCN/Axiu-Plugin/blob/6f336229dce0a22344e37107a738717d0860f6b7/apps/qrLogin.js#L23) | ` 'qrCodeLogin' ` | ` '^#扫码(登录\|登陆\|绑定)$' ` | ` 'all' ` |
| [apps/qrLogin.js:29](https://github.com/AxiuCN/Axiu-Plugin/blob/6f336229dce0a22344e37107a738717d0860f6b7/apps/qrLogin.js#L29) | ` 'updCookie' ` | ` '^#(刷新\|更新\|获取)(ck\|cookie)$' ` | ` 'all' ` |
| [apps/srGachaLog.js:27](https://github.com/AxiuCN/Axiu-Plugin/blob/6f336229dce0a22344e37107a738717d0860f6b7/apps/srGachaLog.js#L27) | ` 'srGachaLog' ` | ` '^#星铁(更新)?(星铁)?(抽卡\|祈愿)?(记录\|历史)$' ` | ` 'all' ` |
| [apps/srGachaLog.js:34](https://github.com/AxiuCN/Axiu-Plugin/blob/6f336229dce0a22344e37107a738717d0860f6b7/apps/srGachaLog.js#L34) | ` 'getSrGachaUrl' ` | ` '^#星铁获取(星铁)?(抽卡\|祈愿)?链接$' ` | ` 'all' ` |

无静态命令对的顶层模块（仍需覆盖）：[apps/autoGroupApprove.js](https://github.com/AxiuCN/Axiu-Plugin/blob/6f336229dce0a22344e37107a738717d0860f6b7/apps/autoGroupApprove.js)、[apps/captchaHandler.js](https://github.com/AxiuCN/Axiu-Plugin/blob/6f336229dce0a22344e37107a738717d0860f6b7/apps/captchaHandler.js)。

## xiaoyao-cvs-plugin

固定提交：`e7ab3e8b276a11680beb47d0c5517f6e0a4c2022`。静态声明：34。

| 源文件与行 | 标签或处理函数 | 规则表达式 | 参考权限 |
| --- | --- | --- | --- |
| [apps/admin.js:28](https://github.com/ctrlcvs/xiaoyao-cvs-plugin/blob/e7ab3e8b276a11680beb47d0c5517f6e0a4c2022/apps/admin.js#L28) | ` "【#管理】更新素材" ` | ` "^#图鉴(强制)?更新$" ` | 由上层或处理函数决定 |
| [apps/admin.js:33](https://github.com/ctrlcvs/xiaoyao-cvs-plugin/blob/e7ab3e8b276a11680beb47d0c5517f6e0a4c2022/apps/admin.js#L33) | ` "【#管理】图鉴更新" ` | ` "^#图鉴插件(强制)?更新" ` | 由上层或处理函数决定 |
| [apps/admin.js:38](https://github.com/ctrlcvs/xiaoyao-cvs-plugin/blob/e7ab3e8b276a11680beb47d0c5517f6e0a4c2022/apps/admin.js#L38) | ` "【#管理】更新素材" ` | ` "^#图鉴模板(强制)?更新$" ` | 由上层或处理函数决定 |
| [apps/admin.js:43](https://github.com/ctrlcvs/xiaoyao-cvs-plugin/blob/e7ab3e8b276a11680beb47d0c5517f6e0a4c2022/apps/admin.js#L43) | ` "【#管理】系统设置" ` | ` sysCfgReg ` | 由上层或处理函数决定 |
| [apps/index.js:83](https://github.com/ctrlcvs/xiaoyao-cvs-plugin/blob/e7ab3e8b276a11680beb47d0c5517f6e0a4c2022/apps/index.js#L83) | ` "【#帮助】 图鉴版本介绍" ` | ` "^#图鉴版本$" ` | 由上层或处理函数决定 |
| [apps/index.js:87](https://github.com/ctrlcvs/xiaoyao-cvs-plugin/blob/e7ab3e8b276a11680beb47d0c5517f6e0a4c2022/apps/index.js#L87) | ` "查看插件的功能" ` | ` "^#?(图鉴)?(命令\|帮助\|菜单\|help\|说明\|功能\|指令\|使用说明)$" ` | 由上层或处理函数决定 |
| [apps/index.js:91](https://github.com/ctrlcvs/xiaoyao-cvs-plugin/blob/e7ab3e8b276a11680beb47d0c5517f6e0a4c2022/apps/index.js#L91) | ` "角色、食物、怪物、武器信息图鉴" ` | ` "^(#(.*)\|.*图鉴)$" ` | 由上层或处理函数决定 |
| [apps/index.js:95](https://github.com/ctrlcvs/xiaoyao-cvs-plugin/blob/e7ab3e8b276a11680beb47d0c5517f6e0a4c2022/apps/index.js#L95) | ` "sr 星穹铁道武器信息图鉴" ` | ` "^((#\|\\*)(.*)\|.*图鉴)$" ` | 由上层或处理函数决定 |
| [apps/index.js:99](https://github.com/ctrlcvs/xiaoyao-cvs-plugin/blob/e7ab3e8b276a11680beb47d0c5517f6e0a4c2022/apps/index.js#L99) | ` "体力" ` | ` "^#*(多\|全\|全部)*(体力\|树脂\|查询体力\|便笺\|便签)$" ` | 由上层或处理函数决定 |
| [apps/index.js:103](https://github.com/ctrlcvs/xiaoyao-cvs-plugin/blob/e7ab3e8b276a11680beb47d0c5517f6e0a4c2022/apps/index.js#L103) | ` "体力推送" ` | ` "^#*((开启\|关闭)体力推送\|体力设置群(推送(开启\|关闭)\|(阈值\|上限)(\\d*)))$" ` | 由上层或处理函数决定 |
| [apps/index.js:107](https://github.com/ctrlcvs/xiaoyao-cvs-plugin/blob/e7ab3e8b276a11680beb47d0c5517f6e0a4c2022/apps/index.js#L107) | ` "体力模板设置" ` | ` "^#(体力模板(设置(.*)\|列表(.*))\|(我的体力模板列表\|体力模板移除(.*)))$" ` | 由上层或处理函数决定 |
| [apps/index.js:112](https://github.com/ctrlcvs/xiaoyao-cvs-plugin/blob/e7ab3e8b276a11680beb47d0c5517f6e0a4c2022/apps/index.js#L112) | ` "体力" ` | ` "#poke#" ` | 由上层或处理函数决定 |
| [apps/index.js:116](https://github.com/ctrlcvs/xiaoyao-cvs-plugin/blob/e7ab3e8b276a11680beb47d0c5517f6e0a4c2022/apps/index.js#L116) | ` "动态" ` | ` '#?(动态\|幻影)' ` | 由上层或处理函数决定 |
| [apps/map.js:15](https://github.com/ctrlcvs/xiaoyao-cvs-plugin/blob/e7ab3e8b276a11680beb47d0c5517f6e0a4c2022/apps/map.js#L15) | ` "地图资源查询 #**在哪里" ` | ` "^#(刷新\|更新)?(.*)(在(哪\|那)里*)$" ` | 由上层或处理函数决定 |
| [apps/map.js:19](https://github.com/ctrlcvs/xiaoyao-cvs-plugin/blob/e7ab3e8b276a11680beb47d0c5517f6e0a4c2022/apps/map.js#L19) | ` "清空地图下载数据" ` | ` "^#(清空\|清除)地图(缓存)?数据$" ` | 由上层或处理函数决定 |
| [apps/mhyTopUpLogin.js:13](https://github.com/ctrlcvs/xiaoyao-cvs-plugin/blob/e7ab3e8b276a11680beb47d0c5517f6e0a4c2022/apps/mhyTopUpLogin.js#L13) | ` "扫码登录" ` | `` `^#(扫码\|二维码\|辅助)(登录\|绑定\|登陆)$` `` | 由上层或处理函数决定 |
| [apps/mhyTopUpLogin.js:17](https://github.com/ctrlcvs/xiaoyao-cvs-plugin/blob/e7ab3e8b276a11680beb47d0c5517f6e0a4c2022/apps/mhyTopUpLogin.js#L17) | ` "账号密码登录" ` | `` `^#(账号\|密码)(密码)?(登录\|绑定\|登陆)$` `` | 由上层或处理函数决定 |
| [apps/mhyTopUpLogin.js:21](https://github.com/ctrlcvs/xiaoyao-cvs-plugin/blob/e7ab3e8b276a11680beb47d0c5517f6e0a4c2022/apps/mhyTopUpLogin.js#L21) | ` "账号密码登录" ` | `` `^账号(.*)密码(.*)$` `` | 由上层或处理函数决定 |
| [apps/mhyTopUpLogin.js:25](https://github.com/ctrlcvs/xiaoyao-cvs-plugin/blob/e7ab3e8b276a11680beb47d0c5517f6e0a4c2022/apps/mhyTopUpLogin.js#L25) | ` '原神充值（离线）' ` | ` '^#?((原神(微信)?充值(微信)?(.*))\|((商品\|充值)列表)\|((订单\|查询)(订单\|查询)(.*)))$' ` | 由上层或处理函数决定 |
| [apps/sign.js:10](https://github.com/ctrlcvs/xiaoyao-cvs-plugin/blob/e7ab3e8b276a11680beb47d0c5517f6e0a4c2022/apps/sign.js#L10) | ` "米社规则签到" ` | `` `^#*(${lodash.map(ForumData,v=> v.otherName.join('\|')).join('\|')}\|游戏)签到$` `` | 由上层或处理函数决定 |
| [apps/sign.js:14](https://github.com/ctrlcvs/xiaoyao-cvs-plugin/blob/e7ab3e8b276a11680beb47d0c5517f6e0a4c2022/apps/sign.js#L14) | ` "米游社米游币签到（理论上会签到全部所以区分开了）" ` | `` `^#*(米游社\|mys\|社区)(原神\|崩坏3\|崩坏2\|未定事件簿\|大别野\|崩坏星穹铁道\|绝区零\|全部)签到$` `` | 由上层或处理函数决定 |
| [apps/sign.js:18](https://github.com/ctrlcvs/xiaoyao-cvs-plugin/blob/e7ab3e8b276a11680beb47d0c5517f6e0a4c2022/apps/sign.js#L18) | ` "云原神签到" ` | ` "^#*云原神签到$" ` | 由上层或处理函数决定 |
| [apps/sign.js:22](https://github.com/ctrlcvs/xiaoyao-cvs-plugin/blob/e7ab3e8b276a11680beb47d0c5517f6e0a4c2022/apps/sign.js#L22) | ` "米游币、云原神查询" ` | `` `^#*(米游币\|米币\|云原神)查询$` `` | 由上层或处理函数决定 |
| [apps/sign.js:26](https://github.com/ctrlcvs/xiaoyao-cvs-plugin/blob/e7ab3e8b276a11680beb47d0c5517f6e0a4c2022/apps/sign.js#L26) | ` "cookies获取帮助" ` | ` "^#*(米游社\|cookies\|米游币\|stoken\|Stoken\|云原神\|云)(帮助\|教程\|绑定)$" ` | 由上层或处理函数决定 |
| [apps/sign.js:30](https://github.com/ctrlcvs/xiaoyao-cvs-plugin/blob/e7ab3e8b276a11680beb47d0c5517f6e0a4c2022/apps/sign.js#L30) | ` "米游币、云原神查询" ` | `` `^#((米游币\|云原神\|米社(原神\|崩坏3\|崩坏2\|未定事件簿)*))全部签到$` `` | 由上层或处理函数决定 |
| [apps/user.js:17](https://github.com/ctrlcvs/xiaoyao-cvs-plugin/blob/e7ab3e8b276a11680beb47d0c5517f6e0a4c2022/apps/user.js#L17) | ` "用户个人信息查询" ` | ` "^#*(ck\|stoken\|cookie\|cookies\|签到)查询$" ` | 由上层或处理函数决定 |
| [apps/user.js:21](https://github.com/ctrlcvs/xiaoyao-cvs-plugin/blob/e7ab3e8b276a11680beb47d0c5517f6e0a4c2022/apps/user.js#L21) | ` "更新抽卡记录" ` | ` "^#*(更新\|获取\|导出)抽卡记录$" ` | 由上层或处理函数决定 |
| [apps/user.js:25](https://github.com/ctrlcvs/xiaoyao-cvs-plugin/blob/e7ab3e8b276a11680beb47d0c5517f6e0a4c2022/apps/user.js#L25) | ` "刷新充值记录" ` | ` "^#*(刷新\|获取\|导出)(充值\|氪金)记录$" ` | 由上层或处理函数决定 |
| [apps/user.js:29](https://github.com/ctrlcvs/xiaoyao-cvs-plugin/blob/e7ab3e8b276a11680beb47d0c5517f6e0a4c2022/apps/user.js#L29) | ` "查询绑定数据" ` | ` "^#*我的(stoken\|云ck)$" ` | 由上层或处理函数决定 |
| [apps/user.js:33](https://github.com/ctrlcvs/xiaoyao-cvs-plugin/blob/e7ab3e8b276a11680beb47d0c5517f6e0a4c2022/apps/user.js#L33) | ` "绑定stoken" ` | ` "^(.*)stoken=(.*)$" ` | 由上层或处理函数决定 |
| [apps/user.js:37](https://github.com/ctrlcvs/xiaoyao-cvs-plugin/blob/e7ab3e8b276a11680beb47d0c5517f6e0a4c2022/apps/user.js#L37) | ` "绑定ck自动获取sk" ` | ` "^(.*)login_ticket=(.*)$" ` | 由上层或处理函数决定 |
| [apps/user.js:41](https://github.com/ctrlcvs/xiaoyao-cvs-plugin/blob/e7ab3e8b276a11680beb47d0c5517f6e0a4c2022/apps/user.js#L41) | ` "云原神签到token获取" ` | ` "^(.*)ct(.*)$" ` | 由上层或处理函数决定 |
| [apps/user.js:45](https://github.com/ctrlcvs/xiaoyao-cvs-plugin/blob/e7ab3e8b276a11680beb47d0c5517f6e0a4c2022/apps/user.js#L45) | ` "删除云原神、stoken数据" ` | ` "^#*删除(我的)*((stoken\|sk)\|(云原神\|云ck))$" ` | 由上层或处理函数决定 |
| [apps/user.js:49](https://github.com/ctrlcvs/xiaoyao-cvs-plugin/blob/e7ab3e8b276a11680beb47d0c5517f6e0a4c2022/apps/user.js#L49) | ` "刷新cookie" ` | ` "^#*(刷新\|更新\|获取)(ck\|cookie)$" ` | 由上层或处理函数决定 |

无静态命令对的顶层模块（仍需覆盖）：[apps/help.js](https://github.com/ctrlcvs/xiaoyao-cvs-plugin/blob/e7ab3e8b276a11680beb47d0c5517f6e0a4c2022/apps/help.js)、[apps/Note.js](https://github.com/ctrlcvs/xiaoyao-cvs-plugin/blob/e7ab3e8b276a11680beb47d0c5517f6e0a4c2022/apps/Note.js)、[apps/srGallery.js](https://github.com/ctrlcvs/xiaoyao-cvs-plugin/blob/e7ab3e8b276a11680beb47d0c5517f6e0a4c2022/apps/srGallery.js)、[apps/xiaoyao_image.js](https://github.com/ctrlcvs/xiaoyao-cvs-plugin/blob/e7ab3e8b276a11680beb47d0c5517f6e0a4c2022/apps/xiaoyao_image.js)。

## Atlas

固定提交：`016e49357666e0823791abdf28fbb3b2efe68225`。静态声明：5。

| 源文件与行 | 标签或处理函数 | 规则表达式 | 参考权限 |
| --- | --- | --- | --- |
| [apps/admin.js:16](https://github.com/Nwflower/Atlas/blob/016e49357666e0823791abdf28fbb3b2efe68225/apps/admin.js#L16) | ` 'update' ` | ` '^[#/]*(github)?(原神\|星铁\|绝区零\|洛克\|rc)?图鉴(强行)?(强制)?升级$' ` | 由上层或处理函数决定 |
| [apps/admin.js:20](https://github.com/Nwflower/Atlas/blob/016e49357666e0823791abdf28fbb3b2efe68225/apps/admin.js#L20) | ` 'updatePlugin' ` | ` '^[#/]*图鉴插件(强行)?(强制)?升级$' ` | 由上层或处理函数决定 |
| [apps/atlas.js:19](https://github.com/Nwflower/Atlas/blob/016e49357666e0823791abdf28fbb3b2efe68225/apps/atlas.js#L19) | ` 'atlas' ` | ` '^[#/]?.+(图鉴)*' ` | 由上层或处理函数决定 |
| [apps/atlasHelp.js:12](https://github.com/Nwflower/Atlas/blob/016e49357666e0823791abdf28fbb3b2efe68225/apps/atlasHelp.js#L12) | ` 'atlasHelp' ` | ` '^[#/](图鉴\|wiki\|百科\|Atlas)(\\s*)(帮助\|菜单\|功能\|help)' ` | 由上层或处理函数决定 |
| [apps/EnemyValue.js:19](https://github.com/Nwflower/Atlas/blob/016e49357666e0823791abdf28fbb3b2efe68225/apps/EnemyValue.js#L19) | ` "query" ` | ` "^[#/](原魔).*(生命值\|攻击力).*" ` | 由上层或处理函数决定 |

