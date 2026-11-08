package main

import (
	"os"
	"slices"
	"testing"

	"github.com/junkerderprovinz/cannoncommander/backend/pkg/desktop"
)

func TestTheSystemDriveIsAFixedDriveThatStays(t *testing.T) {
	system := os.Getenv("SystemDrive")
	drives := listDrives()
	at := slices.IndexFunc(drives, func(v desktop.Volume) bool { return v.ID == system })
	if at < 0 {
		t.Fatalf("%s is missing from %+v", system, drives)
	}
	if v := drives[at]; v.Root != system+`\` || v.Kind != "fixed" || v.Eject {
		t.Fatalf("got %+v", v)
	}
}

func TestALetterWithoutADriveIsNoDrive(t *testing.T) {
	for _, letter := range []string{"Q", "W", "Y"} {
		if _, err := os.Stat(letter + `:\`); err != nil {
			if v := describeDrive(letter); v != nil {
				t.Fatalf("%s: got %+v", letter, v)
			}
			return
		}
	}
	t.Skip("every letter tried is in use")
}
