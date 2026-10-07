package toolversions

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/RayleaBot/RayleaBot/tools/internal/repo"
	"go.yaml.in/yaml/v3"
)

func TestReadersUseChangedSource(t *testing.T) {
	root := repo.Root()
	source, err := os.ReadFile(filepath.Join(root, ".tool-versions"))
	if err != nil {
		t.Fatal(err)
	}
	versions, err := Parse(string(source))
	if err != nil {
		t.Fatal(err)
	}
	changed := strings.ReplaceAll(strings.ReplaceAll(string(source), "\r\n", "\n"), "golang "+versions["golang"], "golang 9.8.7")
	path := filepath.Join(t.TempDir(), ".tool-versions")
	nodeScript := `import {readToolVersions} from './scripts/tool-versions.mjs'; import {pathToFileURL} from 'node:url'; console.log(JSON.stringify(readToolVersions(pathToFileURL(process.argv[1]))));`
	readers := map[string][]string{"node": {"node", "--input-type=module", "-e", nodeScript, path}, "shell": {"bash", filepath.Join(root, "scripts/read-tool-versions.sh"), path}}
	for name, args := range readers {
		if _, err := exec.LookPath(args[0]); err != nil {
			t.Logf("%s reader unavailable: %v", name, err)
			delete(readers, name)
		}
	}
	for _, newline := range []string{"\n", "\r\n"} {
		text := strings.ReplaceAll("# fixture\n\n"+changed, "\n", newline)
		if err := os.WriteFile(path, []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
		expected, err := Read(filepath.Dir(path))
		if err != nil {
			t.Fatal(err)
		}
		if expected["golang"] != "9.8.7" {
			t.Fatal(expected)
		}
		for name, args := range readers {
			cmd := exec.Command(args[0], args[1:]...)
			cmd.Dir = root
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("%s: %v %s", name, err, output)
			}
			actual := map[string]string{}
			if name == "node" {
				if err := json.Unmarshal(output, &actual); err != nil {
					t.Fatal(err)
				}
			} else {
				for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
					k, v, _ := strings.Cut(strings.TrimSpace(line), "=")
					actual[k] = v
				}
			}
			if !reflect.DeepEqual(actual, expected) {
				t.Fatalf("%s: %v != %v", name, actual, expected)
			}
		}
	}
	for _, broken := range []string{changed + "\ngolang 1.2.3\n", strings.ReplaceAll(changed, "nodejs ", "missing "), strings.ReplaceAll(changed, "9.8.7", "latest")} {
		if _, err := Parse(broken); err == nil {
			t.Fatal("accepted invalid declaration")
		}
		if err := os.WriteFile(path, []byte(broken), 0600); err != nil {
			t.Fatal(err)
		}
		for name, args := range readers {
			cmd := exec.Command(args[0], args[1:]...)
			cmd.Dir = root
			if err := cmd.Run(); err == nil {
				t.Fatalf("%s accepted invalid declaration", name)
			}
		}
	}

}

func TestWorkflowVersionSources(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join(repo.Root(), ".github/workflows/*.yml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var workflow struct {
			Jobs map[string]struct {
				Steps []struct {
					Uses string
					With map[string]string
				}
			}
		}
		if err := yaml.Unmarshal(data, &workflow); err != nil {
			t.Fatal(err)
		}
		for name, job := range workflow.Jobs {
			checkedOut, available := false, false
			for _, step := range job.Steps {
				if strings.HasPrefix(step.Uses, "actions/checkout@") {
					checkedOut = true
				}
				if step.Uses == "./.github/actions/tool-versions" {
					if !checkedOut {
						t.Fatalf("%s/%s reads versions before checkout", path, name)
					}
					available = true
				}
				for _, rule := range []struct{ action, field, tool string }{{"actions/setup-node@", "node-version", "nodejs"}, {"pnpm/action-setup@", "version", "pnpm"}} {
					if value, ok := step.With[rule.field]; ok && strings.HasPrefix(step.Uses, rule.action) {
						if !available || value != "${{ steps.toolchain.outputs."+rule.tool+" }}" {
							t.Fatalf("%s/%s: %s version source drift", path, name, rule.action)
						}
					}
				}
			}
		}
	}
}
