package architecture_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestProtectedManagementRoutesDeclareOpenAPISecurity(t *testing.T) {
	root := testServerRoot(t)
	data, err := os.ReadFile(filepath.Join(root, "..", "contracts", "web-api.openapi.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var contract struct {
		Paths map[string]map[string]struct {
			Security []map[string][]string `yaml:"security"`
		} `yaml:"paths"`
	}
	if err := yaml.Unmarshal(data, &contract); err != nil {
		t.Fatal(err)
	}
	parameter := regexp.MustCompile(`\{[^}]+\}`)
	declared := map[string]bool{}
	for path, operations := range contract.Paths {
		for method, operation := range operations {
			declared[method+" "+parameter.ReplaceAllString(path, "{parameter}")] = len(operation.Security) > 0
		}
	}
	checked := 0
	walkGoFiles(t, filepath.Join(root, "internal", "management"), func(path string) {
		if strings.HasSuffix(path, "_test.go") {
			return
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || function.Name.Name != "RegisterProtectedRoutes" {
				continue
			}
			ast.Inspect(function.Body, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if !ok || len(call.Args) == 0 {
					return true
				}
				method := strings.ToLower(selectorIdentName(call.Fun))
				switch method {
				case "get", "post", "put", "delete", "patch":
				default:
					return true
				}
				route, ok := stringLiteralValue(call.Args[0])
				if !ok || !strings.HasPrefix(route, "/api/") {
					return true
				}
				checked++
				if !declared[method+" "+parameter.ReplaceAllString(route, "{parameter}")] {
					t.Errorf("protected route %s %s has no OpenAPI security declaration", method, route)
				}
				return true
			})
		}
	})
	if checked == 0 {
		t.Fatal("no protected management routes were inspected")
	}
}
