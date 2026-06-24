package configruntime

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/schemaassets"
)

type ConfigFieldMetadata struct {
	ApplyPolicy ConfigApplyPolicy
	Secret      bool
	Redaction   string
}

var configFieldMetadata = mustLoadConfigFieldMetadata(schemaassets.ConfigUserSchemaJSON)

func ConfigFieldMetadataForPath(path string) (ConfigFieldMetadata, bool) {
	metadata, ok := configFieldMetadata[path]
	return metadata, ok
}

func ConfigFieldMetadataPaths() []string {
	paths := make([]string, 0, len(configFieldMetadata))
	for path := range configFieldMetadata {
		paths = append(paths, path)
	}
	slices.Sort(paths)
	return paths
}

func ConfigSecretFieldPaths() []string {
	paths := make([]string, 0)
	for path, metadata := range configFieldMetadata {
		if metadata.Secret {
			paths = append(paths, path)
		}
	}
	slices.Sort(paths)
	return paths
}

func ConfigApplyPolicyForPath(path string) (ConfigApplyPolicy, bool) {
	metadata, ok := ConfigFieldMetadataForPath(path)
	return metadata.ApplyPolicy, ok
}

func mustLoadConfigFieldMetadata(payload []byte) map[string]ConfigFieldMetadata {
	metadata, err := loadConfigFieldMetadata(payload)
	if err != nil {
		panic(err)
	}
	return metadata
}

func loadConfigFieldMetadata(payload []byte) (map[string]ConfigFieldMetadata, error) {
	var root configSchemaNode
	if err := json.Unmarshal(payload, &root); err != nil {
		return nil, fmt.Errorf("parse config schema metadata: %w", err)
	}
	state := configSchemaMetadataState{
		defs:     root.Defs,
		metadata: map[string]ConfigFieldMetadata{},
	}
	for key, raw := range root.Properties {
		if err := state.collect(key, raw); err != nil {
			return nil, err
		}
	}
	return state.metadata, nil
}

type configSchemaNode struct {
	Ref         string                     `json:"$ref"`
	Defs        map[string]json.RawMessage `json:"$defs"`
	Properties  map[string]json.RawMessage `json:"properties"`
	ApplyPolicy string                     `json:"x-apply-policy"`
	Secret      bool                       `json:"x-secret"`
	Redaction   string                     `json:"x-redaction"`
}

type configSchemaMetadataState struct {
	defs     map[string]json.RawMessage
	metadata map[string]ConfigFieldMetadata
}

func (s configSchemaMetadataState) collect(path string, raw json.RawMessage) error {
	var node configSchemaNode
	if err := json.Unmarshal(raw, &node); err != nil {
		return fmt.Errorf("parse config schema node %s: %w", path, err)
	}
	if node.Ref != "" {
		resolved, err := s.resolveRef(node.Ref)
		if err != nil {
			return fmt.Errorf("resolve config schema ref %s for %s: %w", node.Ref, path, err)
		}
		return s.collect(path, resolved)
	}
	if len(node.Properties) > 0 {
		for key, child := range node.Properties {
			if err := s.collect(joinConfigPath(path, key), child); err != nil {
				return err
			}
		}
		return nil
	}
	if strings.TrimSpace(node.ApplyPolicy) == "" {
		return fmt.Errorf("config field %s missing x-apply-policy", path)
	}
	metadata := ConfigFieldMetadata{
		ApplyPolicy: ConfigApplyPolicy(strings.TrimSpace(node.ApplyPolicy)),
		Secret:      node.Secret,
		Redaction:   strings.TrimSpace(node.Redaction),
	}
	if metadata.Secret && metadata.Redaction == "" {
		return fmt.Errorf("config secret field %s missing x-redaction", path)
	}
	s.metadata[path] = metadata
	return nil
}

func (s configSchemaMetadataState) resolveRef(ref string) (json.RawMessage, error) {
	name, ok := strings.CutPrefix(ref, "#/$defs/")
	if !ok || strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("unsupported ref %q", ref)
	}
	raw, ok := s.defs[name]
	if !ok {
		return nil, fmt.Errorf("missing definition %q", name)
	}
	return raw, nil
}
