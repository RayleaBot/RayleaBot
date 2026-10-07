// Package depsmanifest reads managed resource manifests without depending on
// release tooling or server internals. Schema validation belongs to the caller.
package depsmanifest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"unicode/utf8"
)

func Read(path string) (map[string]any, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Parse(b)
}
func Parse(b []byte) (map[string]any, error) {
	if !utf8.Valid(b) {
		return nil, fmt.Errorf("deps manifest is not UTF-8")
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	var manifest map[string]any
	if err := d.Decode(&manifest); err != nil {
		return nil, err
	}
	if manifest == nil {
		return nil, fmt.Errorf("deps manifest must be an object")
	}
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		return nil, fmt.Errorf("expected one JSON document")
	}
	return manifest, nil
}

// SemanticErrors supplements the contract schema with uniqueness and trusted
// source URL rules. Malformed shapes are left to the schema validator.
func SemanticErrors(manifest any) []string {
	m, _ := manifest.(map[string]any)
	resources, _ := m["resources"].([]any)
	var errors []string
	ids := map[string]bool{}
	kinds := map[[2]string]bool{}
	for i, value := range resources {
		r, ok := value.(map[string]any)
		if !ok {
			continue
		}
		id, idOK := r["id"].(string)
		platform, pOK := r["platform"].(string)
		kind, kOK := r["kind"].(string)
		if idOK && pOK && kOK {
			key := [2]string{platform, kind}
			if ids[id] || kinds[key] {
				errors = append(errors, fmt.Sprintf("/resources/%d: resource IDs and platform/kind pairs must be unique", i))
			}
			ids[id] = true
			kinds[key] = true
		}
		sources, ok := r["sources"].([]any)
		if !ok {
			continue
		}
		type sourceList struct {
			prefix string
			values []any
		}
		lists := []sourceList{{fmt.Sprintf("/resources/%d/sources", i), sources}}
		if archive, ok := r["ffprobe_archive"].(map[string]any); ok {
			if a, ok := archive["sources"].([]any); ok {
				lists = append(lists, sourceList{fmt.Sprintf("/resources/%d/ffprobe_archive/sources", i), a})
			}
		}
		seen := map[string]bool{}
		for _, list := range lists {
			for j, value := range list.values {
				source, ok := value.(map[string]any)
				if !ok {
					continue
				}
				raw, ok := source["url"].(string)
				if !ok {
					continue
				}
				u, err := url.Parse(raw)
				if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.Fragment != "" {
					errors = append(errors, fmt.Sprintf("%s/%d/url: source requires HTTPS, hostname, and no credentials or fragment", list.prefix, j))
				}
				if seen[raw] {
					errors = append(errors, fmt.Sprintf("%s/%d/url: source URLs must be unique", list.prefix, j))
				}
				seen[raw] = true
			}
		}
	}
	return errors
}
