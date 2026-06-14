package app

import (
	internalconfig "github.com/RayleaBot/RayleaBot/server/internal/config"
	managementhttp "github.com/RayleaBot/RayleaBot/server/internal/management/http"
)

func (s *configHTTPServiceImpl) CurrentConfigDocument() managementhttp.ConfigResponse {
	document, redactedFields := sanitizeConfigDocument(configDocumentFromTyped(s.state.Config))
	return managementhttp.ConfigResponse{
		Config:         document,
		RedactedFields: redactedFields,
	}
}

func (s *configHTTPServiceImpl) UpdateConfigDocument(request map[string]any) (managementhttp.ConfigUpdateResponse, error) {
	newCfg, newSummary, err := internalconfig.SaveDocument(s.state.Summary.ConfigPath, s.state.Summary.SchemaPath, request)
	if err != nil {
		return managementhttp.ConfigUpdateResponse{}, err
	}

	applyEffects := s.ApplyHotReloadableFields(newCfg)
	s.state.Summary = newSummary

	document, redactedFields := sanitizeConfigDocument(configDocumentFromTyped(newCfg))
	return managementhttp.ConfigUpdateResponse{
		Config:          document,
		RedactedFields:  redactedFields,
		RestartRequired: applyEffects.RestartRequired(),
		ApplyEffects:    applyEffects,
	}, nil
}

func configDocumentFromTyped(cfg internalconfig.Config) map[string]any {
	return internalconfig.CanonicalDocumentFromTyped(cfg)
}

func sanitizeConfigDocument(document map[string]any) (map[string]any, []string) {
	cloned := internalconfig.CloneDocument(document)
	if cloned == nil {
		return nil, nil
	}

	return cloned, []string{}
}
