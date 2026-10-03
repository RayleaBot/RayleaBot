package render

import "sync"

type templateCompilation struct {
	compiled *CompiledTemplate
	owner    TemplateSourceInfo
}

// Compilations belong to the service and never leave it through catalog APIs.
// Each lookup uses a bundle built from freshly read content, not file metadata
// or the database's stored source_digest.
type templateCompiler struct {
	mu      sync.Mutex
	entries map[string]templateCompilation
}

func (c *templateCompiler) compileSystem(bundle SourceBundle) (*CompiledTemplate, []TemplateValidationIssue, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.compileLocked(bundle, TemplateSourceInfo{Type: "system"})
}

func (c *templateCompiler) compileLocked(bundle SourceBundle, owner TemplateSourceInfo) (*CompiledTemplate, []TemplateValidationIssue, error) {
	id := bundle.Manifest.ID
	if entry, ok := c.entries[id]; ok && entry.compiled.Bundle.Digest == bundle.Digest {
		entry.owner = owner
		c.entries[id] = entry
		return entry.compiled, nil, nil
	}
	compiled, issues, err := CompileBundle(bundle)
	if err != nil || len(issues) != 0 {
		delete(c.entries, id)
		return compiled, issues, err
	}
	if c.entries == nil {
		c.entries = make(map[string]templateCompilation)
	}
	c.entries[id] = templateCompilation{compiled: compiled, owner: owner}
	return compiled, nil, nil
}

func (c *templateCompiler) removeExceptLocked(ownerType, pluginID string, keep map[string]struct{}) {
	for id, entry := range c.entries {
		if entry.owner.Type != ownerType || (pluginID != "" && entry.owner.PluginID != pluginID) {
			continue
		}
		if _, ok := keep[id]; !ok {
			delete(c.entries, id)
		}
	}
}
