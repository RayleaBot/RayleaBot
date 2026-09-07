package config

// Adapter type names, matching the schema's closed vocabulary.
const (
	AdapterTypeOneBot11   = "onebot11"
	AdapterTypeQQOfficial = "qqofficial"
)

// Identifiers the schema-3 migration assigns to the adapters that used to be
// singleton blocks. They also name the ingress routes those adapters keep.
const (
	DefaultOneBot11AdapterID   = "onebot11"
	DefaultQQOfficialAdapterID = "qq-official"
)

// AdapterByID returns the configured instance with this identifier.
func (c Config) AdapterByID(id string) (AdapterInstance, bool) {
	for _, adapter := range c.Adapters {
		if adapter.ID == id {
			return adapter, true
		}
	}
	return AdapterInstance{}, false
}

// AdaptersOfType returns every configured instance speaking one protocol, in
// configuration order.
func (c Config) AdaptersOfType(adapterType string) []AdapterInstance {
	matched := make([]AdapterInstance, 0, len(c.Adapters))
	for _, adapter := range c.Adapters {
		if adapter.Type == adapterType {
			matched = append(matched, adapter)
		}
	}
	return matched
}

// OneBot11Settings returns the OneBot settings for one instance. The second
// result is false when the instance is absent or speaks another protocol, so a
// caller never reads the wrong adapter's settings.
func (c Config) OneBot11Settings(id string) (OneBotConfig, bool) {
	adapter, ok := c.AdapterByID(id)
	if !ok || adapter.Type != AdapterTypeOneBot11 || adapter.OneBot11 == nil {
		return OneBotConfig{}, false
	}
	return *adapter.OneBot11, true
}

// OneBot11RuntimeSettings applies the instance switch while preserving the
// configured transport values for editing and for a later re-enable.
func (c Config) OneBot11RuntimeSettings(id string) (OneBotConfig, bool) {
	settings, ok := c.OneBot11Settings(id)
	if !ok {
		return OneBotConfig{}, false
	}
	instance, _ := c.AdapterByID(id)
	if !instance.Enabled {
		settings.ReverseWS.Enabled = false
		settings.ForwardWS.Enabled = false
		settings.HTTPAPI.Enabled = false
		settings.Webhook.Enabled = false
	}
	return settings, true
}

// QQOfficialSettings returns the QQ settings for one instance.
func (c Config) QQOfficialSettings(id string) (QQOfficialConfig, bool) {
	adapter, ok := c.AdapterByID(id)
	if !ok || adapter.Type != AdapterTypeQQOfficial || adapter.QQOfficial == nil {
		return QQOfficialConfig{}, false
	}
	return *adapter.QQOfficial, true
}

// PrimaryOneBot11 returns the first configured OneBot adapter. Parts of the
// management surface still speak about "the" OneBot connection; they use this
// and are explicit that it is the first one.
func (c Config) PrimaryOneBot11() (AdapterInstance, OneBotConfig, bool) {
	for _, adapter := range c.AdaptersOfType(AdapterTypeOneBot11) {
		if adapter.OneBot11 != nil {
			return adapter, *adapter.OneBot11, true
		}
	}
	return AdapterInstance{}, OneBotConfig{}, false
}
