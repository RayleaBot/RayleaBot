// Package ordered preserves contract field order in generated JSON and Go code.
package ordered

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"
)

type Field struct {
	Name  string
	Value any
}
type Object []Field

func (o *Object) Get(name string) any {
	if o != nil {
		for _, f := range *o {
			if f.Name == name {
				return f.Value
			}
		}
	}
	return nil
}
func (o *Object) Has(name string) bool {
	for _, f := range *o {
		if f.Name == name {
			return true
		}
	}
	return false
}
func (o *Object) Set(name string, value any) {
	for i := range *o {
		if (*o)[i].Name == name {
			(*o)[i].Value = value
			return
		}
	}
	*o = append(*o, Field{name, value})
}
func (o *Object) Copy() *Object { c := append(Object{}, (*o)...); return &c }
func (o *Object) MarshalJSON() ([]byte, error) {
	var b bytes.Buffer
	b.WriteByte('{')
	for i, f := range *o {
		if i > 0 {
			b.WriteByte(',')
		}
		k, _ := json.Marshal(f.Name)
		v, e := JSON(f.Value, false)
		if e != nil {
			return nil, e
		}
		b.Write(k)
		b.WriteByte(':')
		b.Write(bytes.TrimSpace(v))
	}
	b.WriteByte('}')
	return b.Bytes(), nil
}
func Obj(v any) *Object {
	if o, ok := v.(*Object); ok {
		return o
	}
	return &Object{}
}
func Str(v any) string {
	if v == nil {
		return ""
	}
	return fmt.Sprint(v)
}
func List(v any) []any { a, _ := v.([]any); return a }
func Strings(v any) []string {
	var a []string
	for _, x := range List(v) {
		a = append(a, Str(x))
	}
	return a
}
func At(v any, pointer string) (any, error) {
	if pointer == "" {
		return v, nil
	}
	for _, s := range strings.Split(strings.TrimPrefix(pointer, "/"), "/") {
		s = strings.ReplaceAll(strings.ReplaceAll(s, "~1", "/"), "~0", "~")
		switch n := v.(type) {
		case *Object:
			if !n.Has(s) {
				return nil, fmt.Errorf("missing contract pointer %s", pointer)
			}
			v = n.Get(s)
		case []any:
			i, e := strconv.Atoi(s)
			if e != nil || i < 0 || i >= len(n) {
				return nil, fmt.Errorf("invalid contract pointer %s", pointer)
			}
			v = n[i]
		default:
			return nil, fmt.Errorf("invalid contract pointer %s", pointer)
		}
	}
	return v, nil
}
func Read(path string) (*Object, error) {
	b, e := os.ReadFile(path)
	if e != nil {
		return nil, e
	}
	v, e := Parse(b)
	if e != nil {
		return nil, e
	}
	o, ok := v.(*Object)
	if !ok {
		return nil, fmt.Errorf("expected object: %s", path)
	}
	return o, nil
}
func Parse(b []byte) (any, error) {
	var n yaml.Node
	if e := yaml.Unmarshal(b, &n); e != nil {
		return nil, e
	}
	if len(n.Content) == 0 {
		return nil, fmt.Errorf("empty document")
	}
	return fromYAML(n.Content[0])
}
func fromYAML(n *yaml.Node) (any, error) {
	switch n.Kind {
	case yaml.AliasNode:
		return fromYAML(n.Alias)
	case yaml.MappingNode:
		o := &Object{}
		for i := 0; i < len(n.Content); i += 2 {
			v, e := fromYAML(n.Content[i+1])
			if e != nil {
				return nil, e
			}
			o.Set(n.Content[i].Value, v)
		}
		return o, nil
	case yaml.SequenceNode:
		a := []any{}
		for _, c := range n.Content {
			v, e := fromYAML(c)
			if e != nil {
				return nil, e
			}
			a = append(a, v)
		}
		return a, nil
	default:
		var v any
		e := n.Decode(&v)
		return v, e
	}
}
func JSON(v any, pretty bool) ([]byte, error) {
	var b bytes.Buffer
	e := json.NewEncoder(&b)
	e.SetEscapeHTML(false)
	if pretty {
		e.SetIndent("", "  ")
	}
	err := e.Encode(v)
	return b.Bytes(), err
}
