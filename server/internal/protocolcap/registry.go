package protocolcap

type Support struct {
	Standard    string
	NapCat      string
	LuckyLillia string
}

type Item struct {
	Key     string
	Label   string
	Support Support
	Summary string
}

type Category struct {
	Key   string
	Title string
	Items []Item
}

type Matrix struct {
	Protocol   string
	Categories []Category
}

func OneBot11CompatibilityMatrix() Matrix {
	return Matrix{
		Protocol: "onebot11",
		Categories: []Category{
			{
				Key:   "events",
				Title: "核心事件",
				Items: []Item{
					item("message.private", "私聊消息", supportedEverywhere(), "私聊消息事件进入正式插件事件主链。"),
					item("message.group", "群消息", supportedEverywhere(), "群消息事件进入正式插件事件主链。"),
					item("message_sent.group", "群发送回执", supportedEverywhere(), "群发送回执事件进入正式插件事件主链。"),
					item("request.friend", "好友请求", supportedEverywhere(), "好友请求事件进入正式插件事件主链。"),
					item("request.group", "群请求", supportedEverywhere(), "群请求事件进入正式插件事件主链。"),
					item("notice.flash_file", "闪传文件事件", supportedEverywhere(), "闪传文件事件使用正式 notice.flash_file 事件类型。"),
					item("meta.heartbeat", "心跳事件", supportedEverywhere(), "心跳事件进入正式插件事件主链，同时保持传输状态信号职责。"),
					item("meta.lifecycle", "生命周期事件", supportedEverywhere(), "生命周期事件进入正式插件事件主链，同时保持传输状态信号职责。"),
				},
			},
			{
				Key:   "message_segments",
				Title: "消息段",
				Items: []Item{
					item("text", "文本", supportedEverywhere(), "文本消息段进入正式入站与出站消息段集合。"),
					item("image", "图片", supportedEverywhere(), "图片消息段进入正式入站与出站消息段集合。"),
					item("reply", "回复", supportedEverywhere(), "回复消息段进入正式入站与出站消息段集合。"),
					item("flash_file", "闪传文件", supportedEverywhere(), "闪传文件消息段进入正式入站与出站消息段集合。"),
					item("keyboard", "键盘", Support{
						Standard:    "unsupported",
						NapCat:      "supported",
						LuckyLillia: "unsupported",
					}, "键盘消息段作为 provider 扩展能力，由 NapCat 提供支持。"),
				},
			},
			{
				Key:   "read_capabilities",
				Title: "读取能力",
				Items: []Item{
					item("message.get", "读取单条消息", supportedEverywhere(), "平台提供单条消息读取能力。"),
					item("message.history.get", "读取历史消息", supportedEverywhere(), "平台提供历史消息读取能力。"),
					item("message.forward.get", "读取转发消息", supportedEverywhere(), "平台提供转发消息详情读取能力。"),
					item("file.get", "读取文件详情", supportedEverywhere(), "平台提供文件详情读取能力。"),
					item("file.group.url.get", "读取群文件下载地址", supportedEverywhere(), "平台提供群文件下载地址读取能力。"),
					item("file.private.url.get", "读取私聊文件下载地址", supportedEverywhere(), "平台提供私聊文件下载地址读取能力。"),
					item("group.announcement.list", "群公告列表", supportedEverywhere(), "平台提供群公告列表读取能力。"),
					item("group.essence.list", "群精华列表", supportedEverywhere(), "平台提供群精华列表读取能力。"),
					item("group.honor.get", "群荣誉", supportedEverywhere(), "平台提供群荣誉读取能力。"),
					item("reaction.list", "表情回应列表", supportedEverywhere(), "平台提供消息表情回应读取能力。"),
				},
			},
			{
				Key:   "provider_extensions",
				Title: "Provider 扩展",
				Items: []Item{
					item("notice.group_message_emoji_like", "群消息表情回应事件", Support{
						Standard:    "unsupported",
						NapCat:      "supported",
						LuckyLillia: "unsupported",
					}, "群消息表情回应事件属于 NapCat provider 扩展。"),
					item("provider.napcat.message_emoji.like.set", "NapCat 设置消息表情回应", Support{
						Standard:    "unsupported",
						NapCat:      "supported",
						LuckyLillia: "unsupported",
					}, "NapCat 提供设置消息表情回应扩展动作。"),
					item("provider.napcat.group.sign.set", "NapCat 群签到", Support{
						Standard:    "unsupported",
						NapCat:      "supported",
						LuckyLillia: "unsupported",
					}, "NapCat 提供群签到扩展动作。"),
					item("provider.luckylillia.friend_groups.get", "LuckyLillia 好友分组", Support{
						Standard:    "unsupported",
						NapCat:      "unsupported",
						LuckyLillia: "supported",
					}, "LuckyLillia 提供好友分组扩展动作。"),
				},
			},
		},
	}
}

func supportedEverywhere() Support {
	return Support{
		Standard:    "supported",
		NapCat:      "supported",
		LuckyLillia: "supported",
	}
}

func item(key string, label string, support Support, summary string) Item {
	return Item{
		Key:     key,
		Label:   label,
		Support: support,
		Summary: summary,
	}
}
