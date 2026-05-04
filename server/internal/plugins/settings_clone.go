package plugins

func CloneSettings(values map[string]any) map[string]any {
	cloned := cloneMap(values)
	if cloned == nil {
		return map[string]any{}
	}
	return cloned
}

func CloneSettingValue(value any) any {
	return cloneValue(value)
}
