//go:build desktop

package fsapi

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/nativeshell"
)

type ownThumbs struct {
	data []byte
	err  error
}

func (o ownThumbs) thumb(context.Context, string, os.FileInfo, string) ([]byte, error) {
	if o.err != nil {
		return nil, o.err
	}
	if o.data == nil {
		return nil, errNoThumb
	}
	return o.data, nil
}

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

func TestTheSystemStepsInWhenTheAppsOwnFails(t *testing.T) {
	broken := errors.New("ffmpeg lacks the scale filter")
	ctx := context.Background()
	system := func(context.Context, string, int) ([]byte, error) { return []byte("system"), nil }
	if got, err := (systemThumbs{own: ownThumbs{err: broken}, system: system}).thumb(ctx, "a.mp4", nil, "small"); err != nil || string(got) != "system" {
		t.Fatalf("got %q %v", got, err)
	}
	none := func(context.Context, string, int) ([]byte, error) { return nil, nativeshell.ErrNoPicture }
	if _, err := (systemThumbs{own: ownThumbs{err: broken}, system: none}).thumb(ctx, "a.mp4", nil, "small"); !errors.Is(err, broken) {
		t.Fatalf("the app's own failure got lost: %v", err)
	}
}
