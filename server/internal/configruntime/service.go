package configruntime

import (
	internalconfig "github.com/RayleaBot/RayleaBot/server/internal/config"
)

func (s *Service) CurrentConfigDocument() Document {
	document, redactedFields := sanitizeConfigDocument(ConfigDocumentFromTyped(s.config()))
	return Document{
		Config:         document,
		RedactedFields: redactedFields,
	}
}

func (s *Service) UpdateConfigDocument(request map[string]any) (UpdateResult, error) {
	summary := s.summary()
	newCfg, newSummary, err := internalconfig.SaveDocument(summary.ConfigPath, summary.SchemaPath, request)
	if err != nil {
		return UpdateResult{}, err
	}

	applyEffects := s.ApplyHotReloadableFields(newCfg)
	if s.setSummary != nil {
		s.setSummary(newSummary)
	}

	document, redactedFields := sanitizeConfigDocument(ConfigDocumentFromTyped(newCfg))
	return UpdateResult{
		Document: Document{
			Config:         document,
			RedactedFields: redactedFields,
		},
		RestartRequired: applyEffects.RestartRequired(),
		ApplyEffects:    applyEffects,
	}, nil
}

func ConfigDocumentFromTyped(cfg internalconfig.Config) map[string]any {
	return internalconfig.CanonicalDocumentFromTyped(cfg)
}

func (s *Service) config() internalconfig.Config {
	if s == nil || s.currentConfig == nil {
		return internalconfig.Config{}
	}
	return s.currentConfig()
}

func (s *Service) summary() internalconfig.Summary {
	if s == nil || s.currentSummary == nil {
		return internalconfig.Summary{}
	}
	return s.currentSummary()
}

func sanitizeConfigDocument(document map[string]any) (map[string]any, []string) {
	cloned := internalconfig.CloneDocument(document)
	if cloned == nil {
		return nil, nil
	}

	return cloned, []string{}
}
