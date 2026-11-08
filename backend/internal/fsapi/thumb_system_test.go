package fsapi

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/nativeshell"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/desktop"
)

type ownThumbs struct{ data []byte }

func (o ownThumbs) thumb(context.Context, string, os.FileInfo, string) ([]byte, error) {
	if o.data == nil {
		return nil, errNoThumb
	}
	return o.data, nil
}

type noApp struct{}

func (noApp) AutoUpdate() bool         { return false }
func (noApp) SetAutoUpdate(bool) error { return nil }
func (noApp) UpdateReady() string      { return "" }
func (noApp) Restart()                 {}

func TestTheSystemDrawsOnlyWhatTheAppCannot(t *testing.T) {
	asked := 0
	system := func(_ context.Context, _ string, px int) ([]byte, error) {
		asked++
		if px != 640 {
			t.Errorf("asked for %d pixels", px)
		}
		return []byte("system"), nil
	}
	ctx := context.Background()

	if got, _ := (systemThumbs{own: ownThumbs{}, system: system}).thumb(ctx, "a.docx", nil, "large"); got != nil || asked != 0 {
		t.Fatalf("the container asked the system: %q", got)
	}

	desktop.Use(noApp{}, desktop.Options{})
	t.Cleanup(func() { desktop.Use(nil, desktop.Options{}) })
	if got, _ := (systemThumbs{own: ownThumbs{data: []byte("own")}, system: system}).thumb(ctx, "a.jpg", nil, "large"); string(got) != "own" || asked != 0 {
		t.Fatalf("got %q", got)
	}
	if got, _ := (systemThumbs{own: ownThumbs{}, system: system}).thumb(ctx, "a.docx", nil, "large"); string(got) != "system" || asked != 1 {
		t.Fatalf("got %q", got)
	}
	none := func(context.Context, string, int) ([]byte, error) { return nil, nativeshell.ErrNoPicture }
	if _, err := (systemThumbs{own: ownThumbs{}, system: none}).thumb(ctx, "a.bin", nil, "large"); !errors.Is(err, errNoThumb) {
		t.Fatalf("got %v", err)
	}
}
