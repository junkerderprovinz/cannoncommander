package meta

import (
	"context"
	"os"
	"strings"

	audiotag "github.com/dhowden/tag"
)

func readTags(_ context.Context, _ *Service, f File, rec Record) error {
	fh, err := os.Open(f.Real)
	if err != nil {
		return err
	}
	defer fh.Close()
	m, err := audiotag.ReadFrom(fh)
	if err != nil {
		if err == audiotag.ErrNoTagsFound {
			return nil
		}
		return err
	}
	rec["artist"] = strings.TrimSpace(m.Artist())
	rec["album"] = strings.TrimSpace(m.Album())
	rec["title"] = strings.TrimSpace(m.Title())
	rec["albumartist"] = strings.TrimSpace(m.AlbumArtist())
	rec["genre"] = strings.TrimSpace(m.Genre())
	rec["composer"] = strings.TrimSpace(m.Composer())
	if n, _ := m.Track(); n > 0 {
		rec["track"] = int64(n)
	}
	if y := m.Year(); y > 0 {
		rec["year"] = int64(y)
	}
	return nil
}
