package config

import "net/url"

func Load(configPath, schemaPath string) (Config, Summary, error) {
	var cfg Config

	document, cfg, err := loadCanonicalDocument(configPath, schemaPath, true)
	if err != nil {
		return cfg, Summary{}, err
	}

	return cfg, buildSummary(configPath, schemaPath, cfg, document), nil
}

func Init(configPath, schemaPath string) (Config, Summary, error) {
	return Normalize(configPath, schemaPath)
}

func Normalize(configPath, schemaPath string) (Config, Summary, error) {
	return normalizeCanonicalDocument(configPath, schemaPath)
}

// Validate reads the config the way Load does but leaves the file alone: a
// document written by an older build is migrated in memory to be judged against
// the current schema, and the migrated form is not persisted.
func Validate(configPath, schemaPath string) (Config, Summary, error) {
	document, cfg, err := loadCanonicalDocument(configPath, schemaPath, false)
	if err != nil {
		return cfg, Summary{}, err
	}
	return cfg, buildSummary(configPath, schemaPath, cfg, document), nil
}

type Summary struct {
	ConfigPath       string
	SchemaPath       string
	ServerHost       string
	ServerPort       int
	DatabaseEngine   string
	DatabasePath     string
	WebExposureMode  string
	LoggingLevel     string
	SuperAdminCount  int
	OneBotConfigured bool
	OneBotEndpoint   string
}

func buildSummary(configPath, schemaPath string, cfg Config, _ map[string]any) Summary {
	var endpoint string
	if _, settings, ok := cfg.PrimaryOneBot11(); ok {
		endpoint = firstConfiguredOneBotEndpoint(settings)
	}
	if schemaPath == "" {
		schemaPath = ConfigUserSchemaID
	}
	return Summary{
		ConfigPath:       configPath,
		SchemaPath:       schemaPath,
		ServerHost:       cfg.Server.Host,
		ServerPort:       cfg.Server.Port,
		DatabaseEngine:   cfg.Database.Engine,
		DatabasePath:     cfg.Database.Path,
		WebExposureMode:  cfg.Web.ExposureMode,
		LoggingLevel:     cfg.Log.Level,
		SuperAdminCount:  len(cfg.Admin.SuperAdmins),
		OneBotConfigured: endpoint != "",
		OneBotEndpoint:   sanitizeOneBotEndpoint(endpoint),
	}
}

func firstConfiguredOneBotEndpoint(cfg OneBotConfig) string {
	for _, endpoint := range []string{
		cfg.ForwardWS.URL,
		cfg.ReverseWS.URL,
		cfg.HTTPAPI.URL,
		cfg.Webhook.URL,
	} {
		if endpoint != "" {
			return endpoint
		}
	}
	return ""
}

func sanitizeOneBotEndpoint(raw string) string {
	if raw == "" {
		return ""
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}

	if parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}

	return parsed.Scheme + "://" + parsed.Host
}
