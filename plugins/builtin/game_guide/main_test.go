package main

import "testing"

func TestFindCharacterByNameAliasAndSlug(t *testing.T) {
	item := catalog.Characters[0]
	for _, candidate := range append([]string{item.Name, item.Slug}, item.Aliases...) {
		if candidate == "" {
			continue
		}
		got, ok := findCharacter(candidate)
		if !ok || got.Name != item.Name {
			t.Fatalf("findCharacter(%q) = %#v, %v", candidate, got, ok)
		}
	}
}

func TestNormalizeIgnoresGuideSeparators(t *testing.T) {
	if normalize("The Herta") != normalize("the-herta") {
		t.Fatal("expected equivalent normalized names")
	}
}
