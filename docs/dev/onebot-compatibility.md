# OneBot11 兼容矩阵

本文记录 RayleaBot 在标准 OneBot11、NapCat 和 LuckyLillia 三类实现端上支持的事件、消息段、读取能力与 provider 扩展，供插件开发和适配器维护查阅。管理面不展示这张矩阵，也不提供对应接口；支持范围变化时直接更新本文。

## 核心事件

| 事件 | 标识 | 标准 OneBot11 | NapCat | LuckyLillia | 说明 |
| --- | --- | --- | --- | --- | --- |
| 私聊消息 | `message.private` | 支持 | 支持 | 支持 | 投递给插件 |
| 群消息 | `message.group` | 支持 | 支持 | 支持 | 投递给插件 |
| 群发送回执 | `message_sent.group` | 支持 | 支持 | 支持 | 投递给插件 |
| 好友请求 | `request.friend` | 支持 | 支持 | 支持 | 投递给插件 |
| 群请求 | `request.group` | 支持 | 支持 | 支持 | 投递给插件 |
| 闪传文件事件 | `notice.flash_file` | 支持 | 支持 | 支持 | 使用正式 `notice.flash_file` 事件类型 |
| 心跳事件 | `meta.heartbeat` | 支持 | 支持 | 支持 | 更新连接状态并投递给插件 |
| 生命周期事件 | `meta.lifecycle` | 支持 | 支持 | 支持 | 更新连接状态并投递给插件 |

## 消息段

| 消息段 | 标识 | 标准 OneBot11 | NapCat | LuckyLillia | 说明 |
| --- | --- | --- | --- | --- | --- |
| 文本 | `text` | 支持 | 支持 | 支持 | 接收与发送 |
| 图片 | `image` | 支持 | 支持 | 支持 | 接收与发送 |
| 回复 | `reply` | 支持 | 支持 | 支持 | 接收与发送 |
| 闪传文件 | `flash_file` | 支持 | 支持 | 支持 | 接收与发送 |
| 键盘 | `keyboard` | 不支持 | 支持 | 不支持 | NapCat provider 扩展 |

## 读取能力

以下读取能力在三类实现端均受支持。

| 能力 | 标识 |
| --- | --- |
| 读取单条消息 | `message.get` |
| 读取历史消息 | `message.history.get` |
| 读取转发消息详情 | `message.forward.get` |
| 读取文件详情 | `file.get` |
| 读取群文件下载地址 | `file.group.url.get` |
| 读取私聊文件下载地址 | `file.private.url.get` |
| 群公告列表 | `group.announcement.list` |
| 群精华列表 | `group.essence.list` |
| 群荣誉 | `group.honor.get` |
| 消息表情回应列表 | `reaction.list` |

## Provider 扩展

Provider 扩展只由对应实现端提供。

| 能力 | 标识 | 标准 OneBot11 | NapCat | LuckyLillia | 类型 |
| --- | --- | --- | --- | --- | --- |
| 群消息表情回应 | `notice.group_message_emoji_like` | 不支持 | 支持 | 不支持 | 事件 |
| 设置消息表情回应 | `provider.napcat.message_emoji.like.set` | 不支持 | 支持 | 不支持 | 动作 |
| 群签到 | `provider.napcat.group.sign.set` | 不支持 | 支持 | 不支持 | 动作 |
| 好友分组 | `provider.luckylillia.friend_groups.get` | 不支持 | 不支持 | 支持 | 动作 |
