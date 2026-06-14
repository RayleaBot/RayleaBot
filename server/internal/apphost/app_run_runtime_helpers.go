package apphost

func (s *appRuntimeState) redactString(value string) string {
	if s == nil || s.redactText == nil {
		return value
	}
	return s.redactText(value)
}
