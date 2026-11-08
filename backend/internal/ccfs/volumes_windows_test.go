package ccfs

import (
	"errors"
	"testing"
)

func TestMountForPutsSharesBelowNet(t *testing.T) {
	cases := map[string]string{
		`\\nas\daten`:            "/net/nas/daten",
		`\\nas\daten\filme\2024`: "/net/nas/daten/filme/2024",
		`C:\`:                    "/C/",
		`D:\data`:                "/D/data",
	}
	for in, want := range cases {
		if got := MountFor(in); got != want {
			t.Errorf("MountFor(%s) = %s, want %s", in, got, want)
		}
	}
}

func TestAPulledDriveFromTheSettingsWaitsUntilItIsBack(t *testing.T) {
	r, err := New(nil,
		Source{Name: "C:", Mount: "/C", Root: `C:\`},
		Source{Name: "E:", Mount: "/E", Root: `E:\`},
	)
	if err != nil {
		t.Fatal(err)
	}
	r.SetVolumes([]Volume{{Name: "C:", Root: `C:\`, Drive: Drive{Kind: "fixed"}}})
	if _, err := r.Resolve("/E/fotos"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("the pulled drive still resolves: %v", err)
	}
	r.SetVolumes([]Volume{{Name: "C:", Root: `C:\`}, {Name: "E:", Root: `E:\`, Drive: Drive{Kind: "removable", Eject: true}}})
	loc, err := r.Resolve("/E/fotos")
	if err != nil || loc.Real != `E:\fotos` || !loc.Source.Drive.Eject {
		t.Fatalf("got %+v, %v", loc, err)
	}
}
