package config

func Load(configPath, schemaPath string) (Config, Summary, error) {
	var cfg Config

	document, cfg, err := loadCanonicalDocument(configPath, schemaPath)
	if err != nil {
		return cfg, Summary{}, err
	}

	return cfg, buildSummary(configPath, schemaPath, cfg, document), nil
}
