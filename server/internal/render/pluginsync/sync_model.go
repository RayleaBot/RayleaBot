package pluginsync

import (
	renderrepo "github.com/RayleaBot/RayleaBot/server/internal/render/repository"
	rendertemplates "github.com/RayleaBot/RayleaBot/server/internal/render/templates"
)

type Source struct {
	PluginID     string
	LocalID      string
	Dir          string
	ResourceRoot string
}

type PreparedTemplate struct {
	PluginID     string
	LocalID      string
	TemplateID   string
	Dir          string
	ResourceRoot string
	SourceInfo   renderrepo.TemplateSourceInfo
	Seed         rendertemplates.Seed
}

type PreparedSync struct {
	Templates       []PreparedTemplate
	KeepByPlugin    map[string][]string
	ActivePluginIDs []string
}
