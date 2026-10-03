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
	// Index every function in the package so route registration delegated to
	// helpers such as registerSystemProtectedRoutes or registerPluginReadRoutes
	// is followed instead of silently skipped.
	functions := map[string][]*ast.FuncDecl{}
	var registrars []*ast.FuncDecl
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
			if !ok || function.Body == nil {
				continue
			}
			functions[function.Name.Name] = append(functions[function.Name.Name], function)
			if function.Name.Name == "RegisterProtectedRoutes" {
				registrars = append(registrars, function)
			}
		}
	})

	checked := 0
	visited := map[*ast.FuncDecl]bool{}
	var inspectRoutes func(function *ast.FuncDecl)
	inspectRoutes = func(function *ast.FuncDecl) {
		if visited[function] {
			return
		}
		visited[function] = true
		ast.Inspect(function.Body, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			name := selectorIdentName(call.Fun)
			for _, callee := range functions[name] {
				inspectRoutes(callee)
			}
			if len(call.Args) == 0 {
				return true
			}
			method := strings.ToLower(name)
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
	for _, registrar := range registrars {
		inspectRoutes(registrar)
	}
	if checked == 0 {
		t.Fatal("no protected management routes were inspected")
	}
}
