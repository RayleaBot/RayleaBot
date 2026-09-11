package runtime

import (
	"reflect"
	"slices"
	"strings"

	internalconfig "github.com/RayleaBot/RayleaBot/server/internal/config"
)

// retainConfigFields keeps the listed document paths at their current values.
// A path names struct fields by their JSON names and collection entries by the
// key the config schema declares, which is how apply effects address changes.
// A value absent from the current document is not copied, and a collection
// entry missing from the desired configuration is not created, because it would
// have no identity.
func retainConfigFields(current, desired internalconfig.Config, paths []string) internalconfig.Config {
	source := reflect.ValueOf(current)
	target := reflect.ValueOf(&desired).Elem()
	for _, path := range paths {
		retainConfigPath(target, source, nil, strings.Split(path, "."))
	}
	return desired
}

func retainConfigPath(target, source reflect.Value, parent, path []string) {
	switch target.Kind() {
	case reflect.Pointer:
		if source.IsNil() {
			return
		}
		if target.IsNil() {
			target.Set(reflect.New(target.Type().Elem()))
		}
		retainConfigPath(target.Elem(), source.Elem(), parent, path)
	case reflect.Struct:
		index, omitEmpty, ok := configJSONField(target.Type(), path[0])
		if !ok {
			return
		}
		targetField, sourceField := target.Field(index), source.Field(index)
		if len(path) > 1 {
			retainConfigPath(targetField, sourceField, append(parent[:len(parent):len(parent)], path[0]), path[1:])
			return
		}
		if omitEmpty && emptyJSONValue(sourceField) {
			return
		}
		targetField.Set(cloneConfigValue(sourceField))
	case reflect.Slice:
		// A path ending at an entry names no field to keep.
		if len(path) == 1 {
			return
		}
		key := collectionKeyFor(parent)
		targetEntry, targetFound := configCollectionEntry(target, key, path[0])
		sourceEntry, sourceFound := configCollectionEntry(source, key, path[0])
		if targetFound && sourceFound {
			retainConfigPath(targetEntry, sourceEntry, append(parent[:len(parent):len(parent)], path[0]), path[1:])
		}
	}
}

func configJSONField(structType reflect.Type, name string) (int, bool, bool) {
	for index := range structType.NumField() {
		field := structType.Field(index)
		tagName, options, _ := strings.Cut(field.Tag.Get("json"), ",")
		if field.IsExported() && tagName == name {
			return index, slices.Contains(strings.Split(options, ","), "omitempty"), true
		}
	}
	return 0, false, false
}

func configCollectionEntry(entries reflect.Value, key, id string) (reflect.Value, bool) {
	for index := range entries.Len() {
		entry := reflect.Indirect(entries.Index(index))
		if entry.Kind() != reflect.Struct {
			continue
		}
		field, _, ok := configJSONField(entry.Type(), key)
		if ok && entry.Field(field).Kind() == reflect.String && entry.Field(field).String() == id {
			return entry, true
		}
	}
	return reflect.Value{}, false
}

// emptyJSONValue reports whether encoding/json omits the value under omitempty.
func emptyJSONValue(value reflect.Value) bool {
	switch value.Kind() {
	case reflect.Array, reflect.Map, reflect.Slice, reflect.String:
		return value.Len() == 0
	case reflect.Pointer, reflect.Interface:
		return value.IsNil()
	default:
		return value.IsZero()
	}
}

// cloneConfigValue copies nested slices, maps and pointers so a retained value
// does not share storage with the current configuration.
func cloneConfigValue(value reflect.Value) reflect.Value {
	switch value.Kind() {
	case reflect.Pointer:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		clone := reflect.New(value.Type().Elem())
		clone.Elem().Set(cloneConfigValue(value.Elem()))
		return clone
	case reflect.Slice:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		clone := reflect.MakeSlice(value.Type(), value.Len(), value.Len())
		for index := range value.Len() {
			clone.Index(index).Set(cloneConfigValue(value.Index(index)))
		}
		return clone
	case reflect.Map:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		clone := reflect.MakeMapWithSize(value.Type(), value.Len())
		for entries := value.MapRange(); entries.Next(); {
			clone.SetMapIndex(entries.Key(), cloneConfigValue(entries.Value()))
		}
		return clone
	case reflect.Struct:
		clone := reflect.New(value.Type()).Elem()
		clone.Set(value)
		for index := range value.NumField() {
			if clone.Field(index).CanSet() {
				clone.Field(index).Set(cloneConfigValue(value.Field(index)))
			}
		}
		return clone
	default:
		return value
	}
}
