package config

func Load(configPath, schemaPath string) (Config, Summary, error) {
	var cfg Config

	document, cfg, err := loadCanonicalDocument(configPath, schemaPath)
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

func Validate(configPath, schemaPath string) (Config, Summary, error) {
	return Load(configPath, schemaPath)
}
