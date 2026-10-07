package pluginwiregen

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/RayleaBot/RayleaBot/tools/internal/generated"
	"github.com/RayleaBot/RayleaBot/tools/internal/ordered"
	"github.com/RayleaBot/RayleaBot/tools/internal/repo"
)

func TestCommittedOutputsAndContractChanges(t *testing.T) {
	first, e := Generate(repo.Root())
	if e != nil {
		t.Fatal(e)
	}
	drift, e := generated.Sync(repo.Root(), first, marker, dataDirs, true)
	if e != nil || len(drift) != 0 {
		t.Fatalf("committed output drift: %v %v", drift, e)
	}
	root := t.TempDir()
	for _, dir := range []string{"contracts", "fixtures/plugin-protocol", "scripts/testdata"} {
		if e = os.CopyFS(filepath.Join(root, dir), os.DirFS(filepath.Join(repo.Root(), dir))); e != nil {
			t.Fatal(e)
		}
	}
	schemaPath := filepath.Join(root, "contracts/plugin-protocol.schema.json")
	schema, e := ordered.Read(schemaPath)
	if e != nil {
		t.Fatal(e)
	}
	value, e := ordered.At(schema, "/$defs/init/allOf/1/properties/protocol_version")
	if e != nil {
		t.Fatal(e)
	}
	ordered.Obj(value).Set("const", "99")
	for _, name := range []string{"onebot_action_kind", "provider_extension_action_kind"} {
		v, e := ordered.At(schema, "/$defs/"+name)
		if e != nil {
			t.Fatal(e)
		}
		n := ordered.Obj(v)
		n.Set("enum", append(ordered.List(n.Get("enum")), "fixture.action"))
	}
	b, e := ordered.JSON(schema, true)
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(schemaPath, b, 0600); e != nil {
		t.Fatal(e)
	}
	releasePath := filepath.Join(root, "contracts/release-manifest.schema.json")
	release, e := ordered.Read(releasePath)
	if e != nil {
		t.Fatal(e)
	}
	v, e := ordered.At(release, "/$defs/artifactId")
	if e != nil {
		t.Fatal(e)
	}
	n := ordered.Obj(v)
	n.Set("enum", append(ordered.List(n.Get("enum")), "fixture-x64-full"))
	b, e = ordered.JSON(release, true)
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(releasePath, b, 0600); e != nil {
		t.Fatal(e)
	}
	changed, e := Generate(root)
	if e != nil {
		t.Fatal(e)
	}
	for _, p := range []string{"server/internal/plugins/pluginwire/protocol.generated.go", "sdk/go/internal/pluginwire/protocol.generated.go", "server/internal/platform/contractversions/versions.generated.go", "launcher/internal/contractversions/versions.generated.go"} {
		if bytes.Equal(first[p], changed[p]) || !bytes.Contains(changed[p], []byte(`"99"`)) {
			t.Fatalf("contract version not propagated to %s", p)
		}
	}
	if !bytes.Equal(changed["server/internal/plugins/pluginwire/protocol.generated.go"], changed["sdk/go/internal/pluginwire/protocol.generated.go"]) {
		t.Fatal("SDK and host models differ")
	}
	for _, p := range []string{"server/internal/releaseupdate/artifacts.generated.go", "launcher/internal/desktop/artifacts.generated.go"} {
		if !bytes.Contains(changed[p], []byte(`"fixture-x64-full"`)) {
			t.Fatalf("artifact missing in %s", p)
		}
	}
	if !bytes.Contains(changed["server/internal/plugins/pluginwire/action_kinds.generated.go"], []byte(`"fixture.action"`)) {
		t.Fatal("action enum not propagated")
	}
}
