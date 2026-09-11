package architecture_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/errorcodes"
	"gopkg.in/yaml.v3"
)

type errorCodeCatalog struct {
	Codes map[string]struct {
		Code string `yaml:"code"`
	} `yaml:"codes"`
	Diagnostics map[string]struct{ Description string } `yaml:"diagnostics"`
}

type reportedErrorCode struct {
	path string
	line int
	code string
}

func TestHTTPErrorCodesAreDeclaredForTheirTransport(t *testing.T) {
	serverRoot := testServerRoot(t)
	managementRoot := filepath.Join(serverRoot, "internal")
	packageConstants := managementPackageStringConstants(t, serverRoot, managementRoot)

	walkGoFiles(t, managementRoot, func(path string) {
		if strings.HasSuffix(path, "_test.go") || isGeneratedGoFile(path) {
			return
		}

		fileSet := token.NewFileSet()
		parsed, err := parser.ParseFile(fileSet, path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", relPath(t, serverRoot, path), err)
		}
		constants := packageConstants[filepath.Dir(path)]
		for _, reported := range managementReportedErrorCodes(fileSet, parsed, path, constants) {
			if _, ok := errorcodes.HTTP(reported.code); !ok {
				t.Errorf("%s:%d reports code %q outside its declared HTTP scope", relPath(t, serverRoot, reported.path), reported.line, reported.code)
			}
		}
	})
}

func TestStructuredErrorAndDiagnosticCodesAreRegistered(t *testing.T) {
	serverRoot := testServerRoot(t)
	declared := loadDeclaredErrorCodes(t, serverRoot)
	for _, root := range []string{filepath.Join(serverRoot, "internal"), filepath.Join(serverRoot, "..", "sdk", "go")} {
		constants := managementPackageStringConstants(t, serverRoot, root)
		walkGoFiles(t, root, func(path string) {
			if strings.HasSuffix(path, "_test.go") || isGeneratedGoFile(path) {
				return
			}
			set := token.NewFileSet()
			file, err := parser.ParseFile(set, path, nil, 0)
			if err != nil {
				t.Fatal(err)
			}
			for _, reported := range structuredReportedCodes(set, file, path, constants[filepath.Dir(path)], root != filepath.Join(serverRoot, "internal")) {
				if _, ok := declared[reported.code]; !ok {
					t.Errorf("%s:%d uses unregistered error/diagnostic code %q", relPath(t, serverRoot, path), reported.line, reported.code)
				}
			}
		})
	}
}

func structuredReportedCodes(set *token.FileSet, file *ast.File, path string, constants map[string]string, sdk bool) []reportedErrorCode {
	var reported []reportedErrorCode
	var collect func(ast.Expr)
	collect = func(expr ast.Expr) {
		if code, ok := errorCodeExpressionValue(expr, constants); ok && strings.Contains(code, ".") {
			reported = append(reported, reportedErrorCodeFromExpr(set, path, expr, code))
			return
		}
		if literal, ok := expr.(*ast.CompositeLit); ok {
			for _, element := range literal.Elts {
				collect(element)
			}
		}
		if call, ok := expr.(*ast.CallExpr); ok && selectorIdentName(call.Fun) == "append" {
			for _, element := range call.Args[1:] {
				collect(element)
			}
		}
	}
	isCode := func(name string) bool { return name == "Code" || name == "ErrorCode" || name == "ReasonCodes" }
	ast.Inspect(file, func(node ast.Node) bool {
		switch value := node.(type) {
		case *ast.KeyValueExpr:
			if isCode(selectorIdentName(value.Key)) {
				collect(value.Value)
			}
		case *ast.AssignStmt:
			for index, target := range value.Lhs {
				if isCode(selectorIdentName(target)) && index < len(value.Rhs) {
					collect(value.Rhs[index])
				}
			}
		case *ast.CallExpr:
			if !sdk {
				break
			}
			index := -1
			switch selectorIdentName(value.Fun) {
			case "sendError":
				index = 1
			case "Fail":
				index = 0
			}
			if index >= 0 && len(value.Args) > index {
				collect(value.Args[index])
			}
		}
		return true
	})
	return reported
}

func TestStructuredCodeGuardIncludesAssignmentsSlicesAndSDKFailures(t *testing.T) {
	set := token.NewFileSet()
	file, err := parser.ParseFile(set, "sample.go", `package sample
func f() {
    report := Report{ReasonCodes: []string{"recovery.blocked"}, Code: "plugin.shutdown"}
    report.ReasonCodes = []string{errorcodes.DiagnosticRecoveryDegraded}
    report.ReasonCodes = append(report.ReasonCodes, "unregistered.reason")
    report.ErrorCode = "unregistered.assignment"
    state.sendError("request", "unregistered.sdk", "message")
    event.Fail("unregistered.failure", "message")
    report.Summary = "unregistered.not_a_code"
}`, 0)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, item := range structuredReportedCodes(set, file, "sample.go", map[string]string{
		"errorcodes.DiagnosticRecoveryDegraded": "recovery.degraded",
	}, true) {
		got[item.code] = true
	}
	for _, code := range []string{"recovery.blocked", "plugin.shutdown", "recovery.degraded", "unregistered.reason", "unregistered.assignment", "unregistered.sdk", "unregistered.failure"} {
		if !got[code] {
			t.Errorf("guard missed %s", code)
		}
	}
	if got["unregistered.not_a_code"] {
		t.Error("ordinary text was treated as an error code")
	}
}

func loadDeclaredErrorCodes(t *testing.T, serverRoot string) map[string]struct{} {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(serverRoot, "..", "contracts", "error-codes.yaml"))
	if err != nil {
		t.Fatalf("read error code contract: %v", err)
	}
	var catalog errorCodeCatalog
	if err := yaml.Unmarshal(data, &catalog); err != nil {
		t.Fatalf("decode error code contract: %v", err)
	}
	if len(catalog.Codes) == 0 {
		t.Fatalf("error code contract declares no codes")
	}
	codes := make(map[string]struct{}, len(catalog.Codes))
	for key, value := range catalog.Codes {
		code := strings.TrimSpace(value.Code)
		if code == "" {
			code = key
		}
		codes[code] = struct{}{}
	}
	for code := range catalog.Diagnostics {
		codes[code] = struct{}{}
	}
	return codes
}

func managementPackageStringConstants(t *testing.T, serverRoot, managementRoot string) map[string]map[string]string {
	t.Helper()

	packageConstants := map[string]map[string]string{}
	generated := map[string]string{}
	file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(serverRoot, "internal", "errorcodes", "catalog.generated.go"), nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, decl := range file.Decls {
		group, ok := decl.(*ast.GenDecl)
		if !ok || group.Tok != token.CONST {
			continue
		}
		for _, spec := range group.Specs {
			values := spec.(*ast.ValueSpec)
			for i, value := range values.Values {
				if text, ok := stringLiteralValue(value); ok {
					generated["errorcodes."+values.Names[i].Name] = text
				}
			}
		}
	}
	walkGoFiles(t, managementRoot, func(path string) {
		if strings.HasSuffix(path, "_test.go") || isGeneratedGoFile(path) {
			return
		}
		fileSet := token.NewFileSet()
		parsed, err := parser.ParseFile(fileSet, path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", relPath(t, serverRoot, path), err)
		}
		dir := filepath.Dir(path)
		constants := packageConstants[dir]
		if constants == nil {
			constants = map[string]string{}
			for name, code := range generated {
				constants[name] = code
			}
			packageConstants[dir] = constants
		}
		for _, decl := range parsed.Decls {
			genDecl, ok := decl.(*ast.GenDecl)
			if !ok || genDecl.Tok != token.CONST {
				continue
			}
			for _, spec := range genDecl.Specs {
				valueSpec, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for index, name := range valueSpec.Names {
					if index >= len(valueSpec.Values) {
						continue
					}
					value, ok := errorCodeExpressionValue(valueSpec.Values[index], constants)
					if ok {
						constants[name.Name] = value
					}
				}
			}
		}
	})
	return packageConstants
}

func managementReportedErrorCodes(fileSet *token.FileSet, parsed *ast.File, path string, constants map[string]string) []reportedErrorCode {
	reported := []reportedErrorCode{}
	ast.Inspect(parsed, func(node ast.Node) bool {
		switch typed := node.(type) {
		case *ast.CallExpr:
			if index, ok := errorWriterCodeArgIndex(typed.Fun); ok && len(typed.Args) > index {
				if code, ok := errorCodeExpressionValue(typed.Args[index], constants); ok {
					reported = append(reported, reportedErrorCodeFromExpr(fileSet, path, typed.Args[index], code))
				}
				return true
			}
			if isHTTPAPIDomainErrorWriter(typed.Fun) && len(typed.Args) >= 3 {
				reported = append(reported, domainErrorLiteralCodes(fileSet, path, typed.Args[2], constants)...)
			}
		case *ast.CompositeLit:
			reported = append(reported, systemHTTPErrorLiteralCodes(fileSet, path, typed, constants)...)
		}
		return true
	})
	return reported
}

func errorWriterCodeArgIndex(fun ast.Expr) (int, bool) {
	switch typed := fun.(type) {
	case *ast.Ident:
		switch typed.Name {
		case "writeAuthError", "writeCoreAuthError", "writeError":
			return 2, true
		default:
			return 0, false
		}
	case *ast.SelectorExpr:
		if selectorIdentName(typed.X) == "httpapi" && typed.Sel.Name == "WriteError" {
			return 2, true
		}
	}
	return 0, false
}

func isHTTPAPIDomainErrorWriter(fun ast.Expr) bool {
	selector, ok := fun.(*ast.SelectorExpr)
	return ok && selectorIdentName(selector.X) == "httpapi" && selector.Sel.Name == "WriteDomainError"
}

func domainErrorLiteralCodes(fileSet *token.FileSet, path string, expr ast.Expr, constants map[string]string) []reportedErrorCode {
	literal := unwrapAddressedCompositeLiteral(expr)
	if literal == nil || !isDomainErrorType(literal.Type) {
		return nil
	}
	return keyedStringCodes(fileSet, path, literal, constants, "Code")
}

func systemHTTPErrorLiteralCodes(fileSet *token.FileSet, path string, literal *ast.CompositeLit, constants map[string]string) []reportedErrorCode {
	if !isSystemHTTPErrorType(literal.Type) {
		return nil
	}
	return keyedStringCodes(fileSet, path, literal, constants, "code")
}

func keyedStringCodes(fileSet *token.FileSet, path string, literal *ast.CompositeLit, constants map[string]string, key string) []reportedErrorCode {
	reported := []reportedErrorCode{}
	for _, element := range literal.Elts {
		pair, ok := element.(*ast.KeyValueExpr)
		if !ok || selectorIdentName(pair.Key) != key {
			continue
		}
		if code, ok := errorCodeExpressionValue(pair.Value, constants); ok {
			reported = append(reported, reportedErrorCodeFromExpr(fileSet, path, pair.Value, code))
		}
	}
	return reported
}

func errorCodeExpressionValue(expr ast.Expr, constants map[string]string) (string, bool) {
	if value, ok := stringLiteralValue(expr); ok {
		return value, true
	}
	if selector, ok := expr.(*ast.SelectorExpr); ok {
		value, ok := constants[selectorIdentName(selector.X)+"."+selector.Sel.Name]
		return value, ok
	}
	ident, ok := expr.(*ast.Ident)
	if !ok {
		return "", false
	}
	value, ok := constants[ident.Name]
	return value, ok
}

func stringLiteralValue(expr ast.Expr) (string, bool) {
	literal, ok := expr.(*ast.BasicLit)
	if !ok || literal.Kind != token.STRING {
		return "", false
	}
	value, err := strconv.Unquote(literal.Value)
	if err != nil {
		return "", false
	}
	return value, true
}

func reportedErrorCodeFromExpr(fileSet *token.FileSet, path string, expr ast.Expr, code string) reportedErrorCode {
	return reportedErrorCode{
		path: path,
		line: fileSet.Position(expr.Pos()).Line,
		code: code,
	}
}

func unwrapAddressedCompositeLiteral(expr ast.Expr) *ast.CompositeLit {
	if unary, ok := expr.(*ast.UnaryExpr); ok && unary.Op == token.AND {
		expr = unary.X
	}
	literal, _ := expr.(*ast.CompositeLit)
	return literal
}

func isDomainErrorType(expr ast.Expr) bool {
	selector, ok := expr.(*ast.SelectorExpr)
	return ok && selectorIdentName(selector.X) == "httpapi" && selector.Sel.Name == "DomainError"
}

func isSystemHTTPErrorType(expr ast.Expr) bool {
	ident, ok := expr.(*ast.Ident)
	return ok && ident.Name == "SystemHTTPError"
}

func selectorIdentName(expr ast.Expr) string {
	switch typed := expr.(type) {
	case *ast.Ident:
		return typed.Name
	case *ast.SelectorExpr:
		return typed.Sel.Name
	default:
		return ""
	}
}
