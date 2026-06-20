package service

import (
	renderartifact "github.com/RayleaBot/RayleaBot/server/internal/render/artifact"
	rendertemplates "github.com/RayleaBot/RayleaBot/server/internal/render/templates"
)

func (s *Service) LookupArtifact(artifactID string) (renderartifact.Artifact, error) {
	if s == nil {
		return renderartifact.Artifact{}, &rendertemplates.Error{Code: "platform.resource_missing", Message: "render service is not available"}
	}
	return s.artifactStore.lookup(artifactID)
}
