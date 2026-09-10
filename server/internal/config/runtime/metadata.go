package runtime

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

// ConfigFieldMetadataForPath accepts either a schema shape or a concrete
// document path; a path naming collection entries resolves to the shape they
// share, because entries do not carry per-entry metadata.
func ConfigFieldMetadataForPath(path string) (ConfigFieldMetadata, bool) {
	if metadata, ok := configFieldMetadata[path]; ok {
		return metadata, true
	}
	metadata, ok := configFieldMetadata[ConfigShapePath(path)]
	return metadata, ok
}

// ConfigShapePath rewrites a concrete document path into the schema shape it
// belongs to by replacing each collection entry's key with the wildcard.
func ConfigShapePath(path string) string {
	if path == "" {
		return ""
	}
	segments := strings.Split(path, ".")
	shape := make([]string, 0, len(segments))
	for index := 0; index < len(segments); index++ {
		shape = append(shape, segments[index])
		if _, ok := ConfigCollectionKey(strings.Join(shape, ".")); !ok {
			continue
		}
		// The segment after a collection names one of its entries rather than a
		// field, so the shape carries the wildcard in its place.
		if index+1 < len(segments) {
			index++
			shape = append(shape, ConfigCollectionWildcard)
		}
	}
	return strings.Join(shape, ".")
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
	return s.collectWithReferences(path, raw, make(map[string]bool))
}

func (s configSchemaMetadataState) collectWithReferences(path string, raw json.RawMessage, references map[string]bool) error {
	var node configSchemaNode
	if err := json.Unmarshal(raw, &node); err != nil {
		return fmt.Errorf("parse config schema node %s: %w", path, err)
	}
	if node.Ref != "" {
		if references[node.Ref] {
			return fmt.Errorf("cyclic config schema reference %s at %s", node.Ref, path)
		}
		references[node.Ref] = true
		defer delete(references, node.Ref)
		resolved, err := s.resolveRef(node.Ref)
		if err != nil {
			return fmt.Errorf("resolve config schema ref %s for %s: %w", node.Ref, path, err)
		}
		var base, siblings map[string]json.RawMessage
		if err := json.Unmarshal(resolved, &base); err != nil {
			return fmt.Errorf("parse referenced schema %s: %w", node.Ref, err)
		}
		if err := json.Unmarshal(raw, &siblings); err != nil {
			return fmt.Errorf("parse schema siblings %s: %w", path, err)
		}
		for key, value := range siblings {
			if key != "$ref" {
				base[key] = value
			}
		}
		merged, err := json.Marshal(base)
		if err != nil {
			return fmt.Errorf("merge schema metadata %s: %w", path, err)
		}
		return s.collectWithReferences(path, merged, references)
	}
	if len(node.Properties) > 0 {
		for key, child := range node.Properties {
			if err := s.collectWithReferences(joinConfigPath(path, key), child, references); err != nil {
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
		if strings.TrimSpace(node.ApplyPolicy) == "" {
			return fmt.Errorf("config collection %s missing x-apply-policy", path)
		}
		// Membership of the collection is a configurable fact of its own, so the
		// collection path carries the policy for adding, removing or reordering
		// entries while the entry fields keep theirs.
		s.metadata[path] = ConfigFieldMetadata{ApplyPolicy: ConfigApplyPolicy(strings.TrimSpace(node.ApplyPolicy))}
		s.collections[path] = strings.TrimSpace(node.Collection)
		return s.collectWithReferences(joinConfigPath(path, ConfigCollectionWildcard), node.Items, references)
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
