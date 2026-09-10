package deps

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sync"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
)

//go:embed contracts/deps-manifest.schema.json
var manifestSchemaJSON []byte

var manifestSchema = sync.OnceValues(func() (*jsonschema.Schema, error) {
	var document any
	if err := json.Unmarshal(manifestSchemaJSON, &document); err != nil {
		return nil, err
	}
	compiler := jsonschema.NewCompiler()
	compiler.AssertFormat()
	const id = "https://rayleabot.local/contracts/deps-manifest.schema.json"
	if err := compiler.AddResource(id, document); err != nil {
		return nil, err
	}
	return compiler.Compile(id)
})

func validateManifestJSON(payload []byte) error {
	var document any
	if err := json.Unmarshal(payload, &document); err != nil {
		return err
	}
	schema, err := manifestSchema()
	if err != nil {
		return err
	}
	if err := schema.Validate(document); err != nil {
		return err
	}
	object := document.(map[string]any)
	ids, kinds := map[string]bool{}, map[string]bool{}
	for _, raw := range object["resources"].([]any) {
		resource := raw.(map[string]any)
		id := resource["id"].(string)
		kind := resource["platform"].(string) + ":" + resource["kind"].(string)
		if ids[id] || kinds[kind] {
			return errors.New("managed resource IDs and platform/kind pairs must be unique")
		}
		ids[id], kinds[kind] = true, true
		seen := map[string]bool{}
		for _, rawSource := range resource["sources"].([]any) {
			value := rawSource.(map[string]any)["url"].(string)
			parsed, err := url.Parse(value)
			if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" || parsed.User != nil || parsed.Fragment != "" {
				return fmt.Errorf("resource %s has an invalid HTTPS source", id)
			}
			if seen[value] {
				return fmt.Errorf("resource %s has duplicate source URLs", id)
			}
			seen[value] = true
		}
	}
	return nil
}
