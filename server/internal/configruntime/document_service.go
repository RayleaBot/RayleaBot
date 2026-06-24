package configruntime

import (
	"context"

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
	request = restoreRedactedConfigSecrets(request, ConfigDocumentFromTyped(s.config()))
	if _, _, _, err := internalconfig.NormalizeDocument(summary.ConfigPath, summary.SchemaPath, request); err != nil {
		return UpdateResult{}, err
	}
	storedRequest, err := StoreConfigSecrets(context.Background(), s.secrets, request)
	if err != nil {
		return UpdateResult{}, err
	}
	newCfg, newSummary, err := internalconfig.SaveDocument(summary.ConfigPath, summary.SchemaPath, storedRequest)
	if err != nil {
		return UpdateResult{}, err
	}
	newCfg, err = ResolveConfigSecretRefs(context.Background(), s.secrets, newCfg)
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
