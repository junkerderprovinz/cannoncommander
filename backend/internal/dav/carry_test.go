package dav_test

import (
	"net/http"
	"testing"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
)

func TestDeleteTakesAFileHiddenByNameAlong(t *testing.T) {
	e := newEnv(t)
	e.write("fotos/album/bild.jpg", "bild")
	e.write("fotos/album/Thumbs.db", "klein")
	e.rules["Thumbs.db"] = ccfs.Hidden
	e.want(e.ask("anna", "DELETE", "/mnt/user/fotos/album", "", nil), http.StatusNoContent)
	if e.exists("fotos/album") {
		t.Error("the folder is still there")
	}
}

func TestMoveTakesAFileHiddenByNameAlong(t *testing.T) {
	e := newEnv(t)
	e.write("fotos/album/bild.jpg", "bild")
	e.write("fotos/album/Thumbs.db", "klein")
	e.rules["Thumbs.db"] = ccfs.Hidden
	e.want(e.ask("anna", "MOVE", "/mnt/user/fotos/album", "", e.dest("/mnt/user/fotos/anders")), http.StatusCreated)
	if !e.exists("fotos/anders/Thumbs.db") {
		t.Error("the hidden file did not come along")
	}
}

func TestAFileHiddenByNameCannotBeTakenByItselfOverDav(t *testing.T) {
	e := newEnv(t)
	e.write("fotos/album/Thumbs.db", "klein")
	e.rules["Thumbs.db"] = ccfs.Hidden
	e.want(e.ask("anna", "DELETE", "/mnt/user/fotos/album/Thumbs.db", "", nil), http.StatusForbidden)
	e.want(e.ask("anna", "MOVE", "/mnt/user/fotos/album/Thumbs.db", "", e.dest("/mnt/user/fotos/t.db")), http.StatusForbidden)
}

func TestAPathRuleOrAReadRuleKeepsAFolderInPlace(t *testing.T) {
	for name, effect := range map[string]ccfs.Effect{"hidden": ccfs.Hidden, "read only": ccfs.ReadOnly} {
		t.Run(name, func(t *testing.T) {
			e := newEnv(t)
			e.write("fotos/album/privat/x.jpg", "x")
			e.rules["/mnt/user/fotos/album/privat"] = effect
			e.want(e.ask("anna", "DELETE", "/mnt/user/fotos/album", "", nil), http.StatusForbidden)
			e.want(e.ask("anna", "MOVE", "/mnt/user/fotos/album", "", e.dest("/mnt/user/fotos/anders")), http.StatusForbidden)
			if !e.exists("fotos/album/privat/x.jpg") {
				t.Error("the protected file went away")
			}
		})
	}
}
