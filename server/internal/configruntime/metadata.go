package configruntime

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
)

type ConfigFieldMetadata struct {
	ApplyPolicy ConfigApplyPolicy
	Secret      bool
	Redaction   string
}

// configCollectionKeys maps a collection path to the field naming its entries.
// It is populated while the metadata is built.
var configCollectionKeys = map[string]string{}

var configFieldMetadata = mustLoadConfigFieldMetadata(config.ConfigUserSchemaJSON)

// ConfigCollectionKey reports the field that names entries of the collection at
// this path, if it is one.
func ConfigCollectionKey(path string) (string, bool) {
	key, ok := configCollectionKeys[path]
	return key, ok
}

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
		defs:        root.Defs,
		metadata:    map[string]ConfigFieldMetadata{},
		collections: map[string]string{},
	}
	for key, raw := range root.Properties {
		if err := state.collect(key, raw); err != nil {
			return nil, err
		}
	}
	configCollectionKeys = state.collections
	return state.metadata, nil
}

type configSchemaNode struct {
	Ref         string                     `json:"$ref"`
	Defs        map[string]json.RawMessage `json:"$defs"`
	Properties  map[string]json.RawMessage `json:"properties"`
	Items       json.RawMessage            `json:"items"`
	Collection  string                     `json:"x-collection-key"`
	ApplyPolicy string                     `json:"x-apply-policy"`
	Secret      bool                       `json:"x-secret"`
	Redaction   string                     `json:"x-redaction"`
}

// ConfigCollectionWildcard stands for one entry of a keyed collection in a
// metadata path. Paths containing it describe a shape; resolving them against a
// document substitutes each entry's key.
const ConfigCollectionWildcard = "*"

type configSchemaMetadataState struct {
	defs        map[string]json.RawMessage
	metadata    map[string]ConfigFieldMetadata
	collections map[string]string
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
	// A keyed collection contributes one shape, not one path per entry: its
	// entries only exist in a document, so the wildcard is resolved there.
	if strings.TrimSpace(node.Collection) != "" {
		if len(node.Items) == 0 {
			return fmt.Errorf("config collection %s declares x-collection-key without items", path)
		}
		s.collections[path] = strings.TrimSpace(node.Collection)
		return s.collect(joinConfigPath(path, ConfigCollectionWildcard), node.Items)
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
