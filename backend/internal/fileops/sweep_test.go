package fileops_test

import (
	"context"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/fileops"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/fsindex/fsindextest"
)

func TestTheSweepRemovesPartFilesLeftBehindAndNothingElse(t *testing.T) {
	e := newEnv(t)
	long := time.Now().Add(-48 * time.Hour)
	const (
		leftByJob = "/mnt/user/fotos/.a.jpg.cc-part"
		leftByDav = "/mnt/user/fotos/.b.jpg.0f3a.cc-part"
		fresh     = "/mnt/user/fotos/.c.jpg.cc-part"
		openOne   = "/mnt/user/fotos/.up.bin.cc-part"
		plain     = "/mnt/user/fotos/d.cc-part"
	)
	if _, code := e.startUpload("anna", fileops.UploadRequest{Path: "/mnt/user/fotos/up.bin", Size: 5, Rule: "replace"}); code != http.StatusCreated {
		t.Fatalf("upload: %d", code)
	}
	if err := os.Chtimes(e.real(openOne), long, long); err != nil {
		t.Fatal(err)
	}
	e.write(leftByJob, "x", long)
	e.write(leftByDav, "x", long)
	e.write(fresh, "x", time.Now())
	e.write(plain, "x", long)

	db := fsindextest.New(t)
	fsindextest.Scan(t, db, "Shares", e.root)
	fileops.For(e.deps).SweepParts(context.Background())

	for _, p := range []string{leftByJob, leftByDav} {
		if !e.gone(p) {
			t.Errorf("%s is still there", p)
		}
	}
	for _, p := range []string{fresh, openOne, plain} {
		if e.gone(p) {
			t.Errorf("%s went away", p)
		}
	}
}
