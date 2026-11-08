package main

import (
	"maps"
	"slices"
	"strings"
	"testing"
)

func TestEveryLanguageNamesWhatStays(t *testing.T) {
	if got, want := slices.Sorted(maps.Keys(stays)), slices.Sorted(maps.Keys(fileManager.Label)); !slices.Equal(got, want) {
		t.Fatalf("stays has %v, the choice %v", got, want)
	}
	for lang, s := range stays {
		if s.Item == "" || !strings.HasPrefix(s.Note, s.Item) || len(s.Note) <= len(s.Item) {
			t.Errorf("%s: %+v", lang, s)
		}
	}
}
