package contractcheck

import (
	"encoding/json"
	"fmt"
	"net/url"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

var openapiMethods = []string{"get", "put", "post", "delete", "patch", "head", "options", "trace"}

type operation struct {
	route, method string
	body          object
}

func (op operation) pointer() string { return "/paths/" + pointerEscape(op.route) + "/" + op.method }
func openapiOperations(api object) map[string]operation {
	operations := map[string]operation{}
	paths := requireObject(api["paths"], "OpenAPI paths")
	for _, route := range keys(paths) {
		item := requireObject(paths[route], "OpenAPI route "+route)
		for _, method := range keys(item) {
			if !slices.Contains(openapiMethods, method) {
				continue
			}
			body := requireObject(item[method], "OpenAPI "+strings.ToUpper(method)+" "+route)
			id := str(body["operationId"])
			if strings.TrimSpace(id) == "" {
				fail("OpenAPI %s %s requires operationId", strings.ToUpper(method), route)
			}
			if _, ok := operations[id]; ok {
				fail("OpenAPI duplicate operationId: %s", id)
			}
			operations[id] = operation{route, method, body}
		}
	}
	return operations
}
func requestURL(target string) *url.URL {
	u, err := url.Parse(target)
	if err != nil {
		fail("invalid request target %q: %v", target, err)
	}
	return u
}
func routeParts(path string) []string { return strings.Split(strings.Trim(path, "/"), "/") }
func pathVariable(part string) bool {
	return strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}")
}
func matchingOpenapiPath(paths object, target string, declaredOrder ...[]string) string {
	path := requestURL(target).EscapedPath()
	if has(paths, path) {
		return path
	}
	actual := routeParts(path)
	candidates := keys(paths)
	if len(declaredOrder) > 0 && len(declaredOrder[0]) > 0 {
		candidates = declaredOrder[0]
	}
	for _, candidate := range candidates {
		parts := routeParts(candidate)
		if len(parts) != len(actual) {
			continue
		}
		matches := true
		for i, part := range parts {
			if !(pathVariable(part) && actual[i] != "") && part != actual[i] {
				matches = false
				break
			}
		}
		if matches {
			return candidate
		}
	}
	return ""
}
func requestPathParameters(route, target string) map[string]string {
	parts := routeParts(route)
	actual := routeParts(requestURL(target).EscapedPath())
	values := map[string]string{}
	if len(parts) != len(actual) {
		fail("request path differs from contract route %s", route)
	}
	for i, part := range parts {
		if pathVariable(part) {
			value, err := url.PathUnescape(actual[i])
			if err != nil {
				fail("invalid path parameter: %v", err)
			}
			values[part[1:len(part)-1]] = value
		}
	}
	return values
}
func headerValue(headers any, name string) string {
	for _, key := range keys(obj(headers)) {
		if strings.EqualFold(key, name) {
			if value, ok := obj(headers)[key].(string); ok {
				return mediaName(value)
			}
		}
	}
	return ""
}
func mediaName(value string) string {
	value, _, _ = strings.Cut(value, ";")
	return strings.ToLower(strings.TrimSpace(value))
}

var integerParameter = regexp.MustCompile(`^-?(?:0|[1-9][0-9]*)$`)

func coerceParameter(value string, schemaType any) (any, error) {
	types := arr(schemaType)
	if types == nil {
		types = []any{schemaType}
	}
	if contains(types, "integer") {
		if !integerParameter.MatchString(value) {
			return nil, fmt.Errorf("must be an integer")
		}
		return json.Number(value), nil
	}
	if contains(types, "number") {
		n, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
		if err != nil {
			return nil, fmt.Errorf("must be a number")
		}
		return n, nil
	}
	if contains(types, "boolean") {
		if value == "true" {
			return true, nil
		}
		if value == "false" {
			return false, nil
		}
		return nil, fmt.Errorf("must be true or false")
	}
	return value, nil
}
func parameterInstance(api, parameter object, schema any, pointer string, values []string) (any, error) {
	resolved, pointer := resolveLocal(api, schema, pointer)
	m := obj(resolved)
	if m == nil {
		return nil, fmt.Errorf("schema must be an object")
	}
	if m["type"] == "array" {
		items, _ := resolveLocal(api, m["items"], pointer+"/items")
		itemType := obj(items)["type"]
		var result []any
		for _, value := range values {
			parts := []string{value}
			if parameter["explode"] == false {
				parts = strings.Split(value, ",")
			}
			for _, part := range parts {
				v, err := coerceParameter(part, itemType)
				if err != nil {
					return nil, err
				}
				result = append(result, v)
			}
		}
		return result, nil
	}
	if len(values) != 1 {
		return nil, fmt.Errorf("must not be repeated")
	}
	return coerceParameter(values[0], m["type"])
}
func (c *checker) requestParameterErrors(api object, op operation, request object) []string {
	target := str(request["path"])
	query := requestQuery(requestURL(target).RawQuery)
	pathValues := requestPathParameters(op.route, target)
	route := requireObject(obj(api["paths"])[op.route], "OpenAPI route "+op.route)
	sources := []struct {
		value   any
		pointer string
	}{{route["parameters"], "/paths/" + pointerEscape(op.route) + "/parameters"}, {op.body["parameters"], op.pointer() + "/parameters"}}
	var errors []string
	for _, source := range sources {
		if source.value == nil {
			continue
		}
		parameters, ok := source.value.([]any)
		if !ok {
			fail("%s#%s: parameters must be an array", webAPI, source.pointer)
		}
		for i, value := range parameters {
			parameter, pointer := resolveLocal(api, value, fmt.Sprintf("%s/%d", source.pointer, i))
			p := obj(parameter)
			if p == nil {
				fail("%s#%s: parameter must be an object", webAPI, pointer)
			}
			name, nameOK := p["name"].(string)
			location, locOK := p["in"].(string)
			schema := obj(p["schema"])
			if !nameOK || !locOK || schema == nil {
				fail("%s#%s: parameter requires name, in, and schema", webAPI, pointer)
			}
			var values []string
			switch location {
			case "query":
				values = query[name]
			case "path":
				if value, ok := pathValues[name]; ok {
					values = []string{value}
				}
			case "header":
				for key, raw := range obj(request["headers"]) {
					if strings.EqualFold(key, name) {
						if value, ok := raw.(string); ok {
							values = []string{mediaName(value)}
						}
						break
					}
				}
			default:
				continue
			}
			label := "/parameters/" + pointerEscape(location) + "/" + pointerEscape(name)
			if values == nil {
				if p["required"] == true {
					errors = append(errors, label+": required parameter is missing")
				}
				continue
			}
			instance, err := parameterInstance(api, p, schema, pointer+"/schema", values)
			if err != nil {
				errors = append(errors, label+": "+err.Error())
				continue
			}
			errors = append(errors, prefixErrors(label+": ", c.schemaErrorsAt(webAPI, pointer+"/schema", instance))...)
		}
	}
	return errors
}
func selectMediaType(content object, preferred string) string {
	if len(content) == 0 {
		return ""
	}
	if preferred != "" && has(content, preferred) {
		return preferred
	}
	if has(content, "application/json") {
		return "application/json"
	}
	if len(content) == 1 {
		return keys(content)[0]
	}
	return ""
}
func responseEntry(responses object, status int) (any, string, bool) {
	key := strconv.Itoa(status)
	if v, ok := responses[key]; ok {
		return v, key, true
	}
	if v, ok := responses["default"]; ok {
		return v, "default", true
	}
	return nil, "", false
}
func (c *checker) messageBodyErrors(api object, op operation, direction string, message object) []string {
	var entry any
	var pointer string
	if direction == "request" {
		entry, pointer = resolveLocal(api, op.body["requestBody"], op.pointer()+"/requestBody")
		if obj(entry) == nil {
			if has(message, "body") {
				return []string{"request body is not declared"}
			}
			return nil
		}
		if !has(message, "body") {
			if truth(obj(entry)["required"]) {
				return []string{"required request body is missing"}
			}
			return nil
		}
	} else {
		status, ok := integer(message["status"])
		if !ok || status < 100 || status > 599 {
			return []string{"response.status must be an HTTP status integer"}
		}
		response, key, ok := responseEntry(requireObject(op.body["responses"], "OpenAPI responses"), status)
		if !ok {
			return []string{fmt.Sprintf("response status %d is not declared", status)}
		}
		entry, pointer = resolveLocal(api, response, op.pointer()+"/responses/"+pointerEscape(key))
		if obj(entry) == nil {
			return []string{"response must resolve to an object"}
		}
		if !has(message, "body") {
			if has(obj(obj(entry)["content"]), "application/json") {
				return []string{"JSON response body is missing"}
			}
			return nil
		}
	}
	content := obj(obj(entry)["content"])
	preferred := str(message["content_type"])
	if preferred == "" {
		preferred = headerValue(message["headers"], "content-type")
	}
	if preferred != "" {
		preferred = mediaName(preferred)
		if !has(content, preferred) {
			return []string{fmt.Sprintf("%s media type %q is not declared", direction, preferred)}
		}
	}
	media := selectMediaType(content, preferred)
	if media == "" || !has(obj(content[media]), "schema") {
		return []string{direction + " body media type has no schema"}
	}
	schemaPointer := pointer + "/content/" + pointerEscape(media) + "/schema"
	errors := c.schemaErrorsAt(webAPI, schemaPointer, message["body"])
	errors = append(errors, c.registryCodeErrors(pointerGet(api, schemaPointer), message["body"], fileURI(filepath.Join(c.root, filepath.FromSlash(webAPI))), "")...)
	return errors
}
func (c *checker) httpExampleErrors(api, mapping object, instance any) []string {
	operations := openapiOperations(api)
	id, ok := mapping["operationId"].(string)
	op, found := operations[id]
	if !ok || !found {
		return []string{fmt.Sprintf("unknown OpenAPI operationId %v", mapping["operationId"])}
	}
	direction := str(mapping["direction"])
	if direction != "request" && direction != "response" {
		return []string{"direction must be request or response"}
	}
	if mapping["representation"] == "request" {
		if direction != "request" || mapping["media_type"] != nil || mapping["status"] != nil {
			return []string{"request representation requires request direction, null media_type and null status"}
		}
		request := obj(instance)
		if request == nil || strings.ToLower(str(request["method"])) != op.method {
			return []string{"request method differs from the mapped operation"}
		}
		if matchingOpenapiPath(obj(api["paths"]), str(request["path"]), c.openapiRoutes) != op.route {
			return []string{"request path differs from the mapped operation"}
		}
		return append(c.requestParameterErrors(api, op, request), c.messageBodyErrors(api, op, direction, request)...)
	}
	if mapping["representation"] != "body" {
		return []string{"representation must be body or request"}
	}
	media := str(mapping["media_type"])
	if strings.TrimSpace(media) == "" {
		return []string{"body examples require an explicit media_type"}
	}
	if direction == "request" && mapping["status"] != nil {
		return []string{"request example status must be null"}
	}
	return c.messageBodyErrors(api, op, direction, object{"body": instance, "content_type": media, "status": mapping["status"]})
}
func (c *checker) httpExamples(api object) {
	index := requireObject(c.load("examples/http/index.yaml"), "HTTP examples index")
	mappings := requireObject(index["examples"], "HTTP examples mappings")
	files := map[string]bool{}
	for _, path := range c.files("examples/http", false) {
		if filepath.Ext(path) == ".json" {
			files[filepath.Base(path)] = true
		}
	}
	var missing, stale []string
	for _, name := range keys(files) {
		if !has(mappings, name) {
			missing = append(missing, name)
		}
	}
	for _, name := range keys(mappings) {
		if !files[name] {
			stale = append(stale, name)
		}
	}
	if len(missing)+len(stale) > 0 {
		fail("HTTP example mappings drift: missing=%v; stale=%v", missing, stale)
	}
	for _, name := range keys(mappings) {
		mapping := requireObject(mappings[name], "HTTP example mapping "+name)
		requireFields(mapping, []string{"operationId", "direction", "status", "media_type", "representation"}, name)
		errors := c.httpExampleErrors(api, mapping, c.load("examples/http/"+name))
		if len(errors) > 0 {
			fail("examples/http/%s: %v", name, errors[:min(3, len(errors))])
		}
	}
}
func (c *checker) openapiFixtures(api object) {
	paths := requireObject(api["paths"], "web-api paths")
	schemas := requireObject(obj(api["components"])["schemas"], "web-api schemas")
	for _, name := range keys(schemas) {
		if err := c.checkSchema(schemas[name]); err != nil {
			fail("%s#/components/schemas/%s: invalid Draft 2020-12 schema: %v", webAPI, pointerEscape(name), err)
		}
	}
	for _, path := range c.dataFiles("fixtures/web-api", false) {
		doc := requireObject(c.load(path), path)
		request := requireObject(doc["request"], path+" request")
		response := requireObject(doc["response"], path+" response")
		method := strings.ToLower(str(request["method"]))
		route := matchingOpenapiPath(paths, str(request["path"]), c.openapiRoutes)
		if route == "" {
			fail("%s: request path is not declared in OpenAPI", path)
		}
		body := obj(obj(paths[route])[method])
		if body == nil {
			fail("%s: method %s is not declared for %s", path, strings.ToUpper(method), route)
		}
		op := operation{route, method, body}
		requestErrors := append(c.requestParameterErrors(api, op, request), c.messageBodyErrors(api, op, "request", request)...)
		responseErrors := append(c.messageBodyErrors(api, op, "response", response), c.httpErrorCatalogErrors(response, c.catalog)...)
		if fixtureExpectedValid(path, doc) {
			requireFixtureOutcome(path, true, responseErrors)
			if doc["case"] != "invalid" && len(requestErrors) > 0 {
				fail("%s: request body validation failed: %v", path, requestErrors[:min(3, len(requestErrors))])
			}
		} else if len(requestErrors)+len(responseErrors) == 0 {
			fail("%s: invalid fixture did not fail request or response validation", path)
		}
	}
}
func openapiBasic(api object) {
	if api["openapi"] != "3.1.0" {
		fail("contracts/web-api.openapi.yaml must use OpenAPI 3.1.0")
	}
	info := requireObject(api["info"], "web-api info")
	version, ok := info["version"].(string)
	if !ok || !regexp.MustCompile(`^\d+\.\d+\.\d+$`).MatchString(version) {
		fail("contracts/web-api.openapi.yaml info.version must be a semantic version")
	}
	paths := requireObject(api["paths"], "web-api paths")
	if len(paths) == 0 {
		fail("web-api paths must not be empty")
	}
	requireObject(requireObject(api["components"], "web-api components")["schemas"], "web-api components.schemas")
	for _, path := range []string{"/healthz", "/readyz", "/api/session/login", "/api/logs"} {
		if !has(paths, path) {
			fail("web-api missing required entry path: %s", path)
		}
	}
}
