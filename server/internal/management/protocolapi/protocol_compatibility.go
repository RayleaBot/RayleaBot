package protocolapi

func (s *ProtocolService) currentOneBot11ProtocolCompatibility() (oneBot11ProtocolCompatibilityResponse, error) {
	return oneBot11CompatibilityMatrix(), nil
}

func oneBot11CompatibilityMatrix() oneBot11ProtocolCompatibilityResponse {
	categories := []protocolCompatibilityCategoryResponse{
		{
			Key:   "events",
			Title: "核心事件",
			Items: []protocolCompatibilityItemResponse{
				compatibilityItem("message.private", "私聊消息", supportedEverywhere(), "私聊消息事件进入正式插件事件主链。"),
				compatibilityItem("message.group", "群消息", supportedEverywhere(), "群消息事件进入正式插件事件主链。"),
				compatibilityItem("message_sent.group", "群发送回执", supportedEverywhere(), "群发送回执事件进入正式插件事件主链。"),
				compatibilityItem("request.friend", "好友请求", supportedEverywhere(), "好友请求事件进入正式插件事件主链。"),
				compatibilityItem("request.group", "群请求", supportedEverywhere(), "群请求事件进入正式插件事件主链。"),
				compatibilityItem("notice.flash_file", "闪传文件事件", supportedEverywhere(), "闪传文件事件使用正式 notice.flash_file 事件类型。"),
				compatibilityItem("meta.heartbeat", "心跳事件", supportedEverywhere(), "心跳事件进入正式插件事件主链，同时保持传输状态信号职责。"),
				compatibilityItem("meta.lifecycle", "生命周期事件", supportedEverywhere(), "生命周期事件进入正式插件事件主链，同时保持传输状态信号职责。"),
			},
		},
		{
			Key:   "message_segments",
			Title: "消息段",
			Items: []protocolCompatibilityItemResponse{
				compatibilityItem("text", "文本", supportedEverywhere(), "文本消息段进入正式入站与出站消息段集合。"),
				compatibilityItem("image", "图片", supportedEverywhere(), "图片消息段进入正式入站与出站消息段集合。"),
				compatibilityItem("reply", "回复", supportedEverywhere(), "回复消息段进入正式入站与出站消息段集合。"),
				compatibilityItem("flash_file", "闪传文件", supportedEverywhere(), "闪传文件消息段进入正式入站与出站消息段集合。"),
				compatibilityItem("keyboard", "键盘", protocolCompatibilitySupportResponse{
					Standard:    "unsupported",
					NapCat:      "supported",
					LuckyLillia: "unsupported",
				}, "键盘消息段作为 provider 扩展能力，由 NapCat 提供支持。"),
			},
		},
		{
			Key:   "read_capabilities",
			Title: "读取能力",
			Items: []protocolCompatibilityItemResponse{
				compatibilityItem("message.get", "读取单条消息", supportedEverywhere(), "平台提供单条消息读取能力。"),
				compatibilityItem("message.history.get", "读取历史消息", supportedEverywhere(), "平台提供历史消息读取能力。"),
				compatibilityItem("message.forward.get", "读取转发消息", supportedEverywhere(), "平台提供转发消息详情读取能力。"),
				compatibilityItem("file.get", "读取文件详情", supportedEverywhere(), "平台提供文件详情读取能力。"),
				compatibilityItem("file.group.url.get", "读取群文件下载地址", supportedEverywhere(), "平台提供群文件下载地址读取能力。"),
				compatibilityItem("file.private.url.get", "读取私聊文件下载地址", supportedEverywhere(), "平台提供私聊文件下载地址读取能力。"),
				compatibilityItem("group.announcement.list", "群公告列表", supportedEverywhere(), "平台提供群公告列表读取能力。"),
				compatibilityItem("group.essence.list", "群精华列表", supportedEverywhere(), "平台提供群精华列表读取能力。"),
				compatibilityItem("group.honor.get", "群荣誉", supportedEverywhere(), "平台提供群荣誉读取能力。"),
				compatibilityItem("reaction.list", "表情回应列表", supportedEverywhere(), "平台提供消息表情回应读取能力。"),
			},
		},
		{
			Key:   "provider_extensions",
			Title: "Provider 扩展",
			Items: []protocolCompatibilityItemResponse{
				compatibilityItem("notice.group_message_emoji_like", "群消息表情回应事件", protocolCompatibilitySupportResponse{
					Standard:    "unsupported",
					NapCat:      "supported",
					LuckyLillia: "unsupported",
				}, "群消息表情回应事件属于 NapCat provider 扩展。"),
				compatibilityItem("provider.napcat.message_emoji.like.set", "NapCat 设置消息表情回应", protocolCompatibilitySupportResponse{
					Standard:    "unsupported",
					NapCat:      "supported",
					LuckyLillia: "unsupported",
				}, "NapCat 提供设置消息表情回应扩展动作。"),
				compatibilityItem("provider.napcat.group.sign.set", "NapCat 群签到", protocolCompatibilitySupportResponse{
					Standard:    "unsupported",
					NapCat:      "supported",
					LuckyLillia: "unsupported",
				}, "NapCat 提供群签到扩展动作。"),
				compatibilityItem("provider.luckylillia.friend_groups.get", "LuckyLillia 好友分组", protocolCompatibilitySupportResponse{
					Standard:    "unsupported",
					NapCat:      "unsupported",
					LuckyLillia: "supported",
				}, "LuckyLillia 提供好友分组扩展动作。"),
			},
		},
	}

	return oneBot11ProtocolCompatibilityResponse{
		Protocol:   "onebot11",
		Categories: categories,
	}
}

func supportedEverywhere() protocolCompatibilitySupportResponse {
	return protocolCompatibilitySupportResponse{
		Standard:    "supported",
		NapCat:      "supported",
		LuckyLillia: "supported",
	}
}

func compatibilityItem(key string, label string, support protocolCompatibilitySupportResponse, summary string) protocolCompatibilityItemResponse {
	return protocolCompatibilityItemResponse{
		Key:     key,
		Label:   label,
		Support: support,
		Summary: summary,
	}
}
