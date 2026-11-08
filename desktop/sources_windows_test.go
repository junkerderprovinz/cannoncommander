package main

import (
	"reflect"
	"testing"
)

func TestEveryDriveWithADiskIsASource(t *testing.T) {
	// C, D and E are there; E is a card reader without a card.
	mask := func() (uint32, error) { return 1<<2 | 1<<3 | 1<<4, nil }
	readable := map[string]bool{`C:\`: true, `D:\`: true}
	got := drives(mask, func(p string) bool { return readable[p] })
	want := []source{{Name: "C:", Path: `C:\`}, {Name: "D:", Path: `D:\`}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v", got)
	}
}
