package fsapi

import (
	"context"
	"errors"
	"os"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/nativeshell"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/desktop"
)

// systemThumbs draws with the app's own thumbnailer first and, in the
// desktop app, asks the computer's thumbnail handlers for what that one
// has no picture of, such as Office files, or videos on a Windows without
// ffmpeg.
type systemThumbs struct {
	own    thumbnailer
	system func(ctx context.Context, real string, px int) ([]byte, error)
}

// withSystemThumbs adds the computer's thumbnails behind own.
func withSystemThumbs(own thumbnailer) thumbnailer {
	return systemThumbs{own: own, system: nativeshell.Thumbnail}
}

// systemEdges are the sizes asked of the system, those of the app's own.
var systemEdges = map[string]int{"small": 256, "large": 640}

func (s systemThumbs) thumb(ctx context.Context, real string, info os.FileInfo, size string) ([]byte, error) {
	data, err := s.own.thumb(ctx, real, info, size)
	if !errors.Is(err, errNoThumb) || !desktop.Active() {
		return data, err
	}
	data, err = s.system(ctx, real, systemEdges[size])
	if errors.Is(err, nativeshell.ErrNoPicture) || errors.Is(err, nativeshell.ErrUnsupported) {
		return nil, errNoThumb
	}
	return data, err
}
