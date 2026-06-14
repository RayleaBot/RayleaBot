package configruntime

import (
	internalconfig "github.com/RayleaBot/RayleaBot/server/internal/config"
	managementhttp "github.com/RayleaBot/RayleaBot/server/internal/management/http"
)

func (s *Service) CurrentConfigDocument() managementhttp.ConfigResponse {
	document, redactedFields := sanitizeConfigDocument(ConfigDocumentFromTyped(s.config()))
	return managementhttp.ConfigResponse{
		Config:         document,
		RedactedFields: redactedFields,
	}
}

func (s *Service) UpdateConfigDocument(request map[string]any) (managementhttp.ConfigUpdateResponse, error) {
	summary := s.summary()
	newCfg, newSummary, err := internalconfig.SaveDocument(summary.ConfigPath, summary.SchemaPath, request)
	if err != nil {
		return managementhttp.ConfigUpdateResponse{}, err
	}

	applyEffects := s.ApplyHotReloadableFields(newCfg)
	if s.setSummary != nil {
		s.setSummary(newSummary)
	}

	document, redactedFields := sanitizeConfigDocument(ConfigDocumentFromTyped(newCfg))
	return managementhttp.ConfigUpdateResponse{
		Config:          document,
		RedactedFields:  redactedFields,
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
