package config

import "strings"

// SecretReferenceFor identifies the secret stored for one configuration field.
func SecretReferenceFor(path []string) string {
	return "secret://" + strings.Join(path, "/")
}

func SecretStoreKeyFor(path []string) string {
	return "config." + strings.Join(path, ".")
}
