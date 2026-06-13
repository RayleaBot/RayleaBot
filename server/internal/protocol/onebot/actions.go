package onebot

type ActionSpec struct {
	Kind           string
	Capability     string
	Provider       string
	APIName        string
	CollectionKey  string
	RequiredFields []string
	NoParams       bool
}

var actionSpecs = []ActionSpec{
	staticAction("message.get", "get_msg", "message_id"),
	staticAction("message.delete", "delete_msg", "message_id"),
	{Kind: "message.history.get", CollectionKey: "messages"},
	{Kind: "message.forward.get", CollectionKey: "messages"},
	{Kind: "message.forward.send"},
	{Kind: "message.read.mark"},
	staticAction("friend.request.handle", "set_friend_add_request", "flag"),
	noParamAction("friend.list", "get_friend_list", "friends"),
	staticAction("friend.remark.set", "set_friend_remark", "user_id"),
	staticAction("user.info.get", "get_stranger_info", "user_id"),
	staticAction("user.like.send", "send_like", "user_id"),
	noParamAction("group.list", "get_group_list", "groups"),
	staticAction("group.info.get", "get_group_info", "group_id"),
	staticAction("group.member.get", "get_group_member_info", "group_id", "user_id"),
	staticCollectionAction("group.member.list", "get_group_member_list", "members", "group_id"),
	staticAction("group.request.handle", "set_group_add_request", "flag"),
	staticAction("group.leave", "set_group_leave", "group_id"),
	staticAction("group.admin.set", "set_group_admin"),
	{Kind: "group.ban.set"},
	staticAction("group.card.set", "set_group_card"),
	staticAction("group.title.set", "set_group_special_title"),
	staticAction("group.name.set", "set_group_name"),
	staticCollectionAction("group.announcement.list", "_get_group_notice", "announcements"),
	staticAction("group.announcement.create", "_send_group_notice"),
	staticAction("group.announcement.delete", "_del_group_notice"),
	staticCollectionAction("group.essence.list", "get_essence_msg_list", "messages"),
	staticAction("group.essence.set", "set_essence_msg"),
	staticAction("group.essence.unset", "delete_essence_msg"),
	staticAction("group.honor.get", "get_group_honor_info"),
	staticAction("group.todo.set", "set_group_todo"),
	staticAction("file.get", "get_file"),
	staticAction("file.download", "download_file"),
	staticAction("file.group.upload", "upload_group_file"),
	staticAction("file.private.upload", "upload_private_file"),
	staticAction("file.group.url.get", "get_group_file_url"),
	staticAction("file.private.url.get", "get_private_file_url"),
	staticAction("file.group.fs.info", "get_group_file_system_info"),
	{Kind: "file.group.fs.list"},
	staticAction("file.group.fs.mkdir", "create_group_file_folder"),
	{Kind: "file.group.fs.delete"},
	staticAction("reaction.set", "set_msg_emoji_like"),
	staticCollectionAction("reaction.list", "get_msg_emoji_like_list", "reactions"),
	staticAction("poke.send", "send_poke"),
	providerAction("provider.napcat.message_emoji.like.set", "napcat", "set_msg_emoji_like"),
	providerAction("provider.napcat.group.sign.set", "napcat", "set_group_sign"),
	providerCollectionAction("provider.luckylillia.friend_groups.get", "luckylillia", "get_grouped_friend_list", "groups"),
}

var actionSpecByKind = buildActionSpecByKind()

func Actions() []ActionSpec {
	items := make([]ActionSpec, 0, len(actionSpecs))
	for _, spec := range actionSpecs {
		items = append(items, normalizeSpec(spec))
	}
	return items
}

func LookupAction(kind string) (ActionSpec, bool) {
	spec, ok := actionSpecByKind[kind]
	if !ok {
		return ActionSpec{}, false
	}
	return cloneSpec(spec), true
}

func IsGenericAction(kind string) bool {
	spec, ok := LookupAction(kind)
	return ok && spec.Provider == ""
}

func IsProviderExtensionAction(kind string) bool {
	spec, ok := LookupAction(kind)
	return ok && spec.Provider != ""
}

func buildActionSpecByKind() map[string]ActionSpec {
	registry := make(map[string]ActionSpec, len(actionSpecs))
	for _, spec := range actionSpecs {
		spec = normalizeSpec(spec)
		registry[spec.Kind] = spec
	}
	return registry
}

func normalizeSpec(spec ActionSpec) ActionSpec {
	if spec.Capability == "" {
		spec.Capability = spec.Kind
	}
	return cloneSpec(spec)
}

func cloneSpec(spec ActionSpec) ActionSpec {
	spec.RequiredFields = append([]string(nil), spec.RequiredFields...)
	return spec
}

func staticAction(kind, apiName string, required ...string) ActionSpec {
	return ActionSpec{
		Kind:           kind,
		APIName:        apiName,
		RequiredFields: append([]string(nil), required...),
	}
}

func staticCollectionAction(kind, apiName, collectionKey string, required ...string) ActionSpec {
	spec := staticAction(kind, apiName, required...)
	spec.CollectionKey = collectionKey
	return spec
}

func noParamAction(kind, apiName, collectionKey string) ActionSpec {
	return ActionSpec{
		Kind:          kind,
		APIName:       apiName,
		CollectionKey: collectionKey,
		NoParams:      true,
	}
}

func providerAction(kind, provider, apiName string) ActionSpec {
	return ActionSpec{
		Kind:     kind,
		Provider: provider,
		APIName:  apiName,
	}
}

func providerCollectionAction(kind, provider, apiName, collectionKey string) ActionSpec {
	spec := providerAction(kind, provider, apiName)
	spec.CollectionKey = collectionKey
	return spec
}
