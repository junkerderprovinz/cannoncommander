//go:build desktop

package clipapi

import (
	"testing"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
)

func TestFilesOnSharesAndDrivesFindTheirSource(t *testing.T) {
	sources := []ccfs.Source{{Mount: "/net/nas/daten", Root: `\\nas\daten`}, {Mount: "/E", Root: `E:\`}}
	cases := []struct{ real, api string }{
		{`\\nas\daten\filme\a.mkv`, "/net/nas/daten/filme/a.mkv"},
		{`\\NAS\Daten\b.txt`, "/net/nas/daten/b.txt"},
		{`E:\`, "/E"},
		{`E:\Fotos\c.jpg`, "/E/Fotos/c.jpg"},
	}
	for _, c := range cases {
		if api, ok := apiPath(sources, c.real); !ok || api != c.api {
			t.Errorf("%s: got %q %v, want %q", c.real, api, ok, c.api)
		}
	}
	if _, ok := apiPath(sources, `\\nas\andere\d.txt`); ok {
		t.Error("a share that is no source found one")
	}
}

func TestAShareIsOneVolumeAndADriveAnother(t *testing.T) {
	if !sameVolume(`\\nas\daten\a.txt`, `\\NAS\daten\filme`) || sameVolume(`\\nas\daten\a.txt`, `C:\Users`) || sameVolume(`E:\a`, `C:\a`) {
		t.Fatal("volumes told apart wrongly")
	}
}
