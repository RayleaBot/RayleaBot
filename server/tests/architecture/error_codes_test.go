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

	"gopkg.in/yaml.v3"
)

type errorCodeCatalog struct {
	Codes map[string]struct {
		Code string `yaml:"code"`
	} `yaml:"codes"`
}

type reportedErrorCode struct {
	path string
	line int
	code string
}

func TestManagementErrorCodesAreDeclaredInContract(t *testing.T) {
	serverRoot := testServerRoot(t)
	declaredCodes := loadDeclaredErrorCodes(t, serverRoot)
	managementRoot := filepath.Join(serverRoot, "internal", "management")
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
			if _, ok := declaredCodes[reported.code]; !ok {
				t.Errorf("%s:%d reports error code %q, but contracts/error-codes.yaml does not declare it", relPath(t, serverRoot, reported.path), reported.line, reported.code)
			}
		}
	})
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
	return codes
}

func managementPackageStringConstants(t *testing.T, serverRoot, managementRoot string) map[string]map[string]string {
	t.Helper()

	packageConstants := map[string]map[string]string{}
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
					value, ok := stringLiteralValue(valueSpec.Values[index])
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
		case "writeAppError", "writeAuthError", "writeError":
			return 3, true
		default:
			return 0, false
		}
	case *ast.SelectorExpr:
		if selectorIdentName(typed.X) == "httpapi" && typed.Sel.Name == "WriteError" {
			return 3, true
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
