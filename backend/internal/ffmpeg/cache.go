package ffmpeg

import (
	"time"

	"github.com/gtsteffaniak/go-cache/cache"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/utils"
)

var (
	MediaCache           = cache.NewCache[[]utils.SubtitleTrack](24 * time.Hour) // subtitle track lists from ffprobe
	SubtitleContentCache = cache.NewCache[string](24 * time.Hour)                // extracted embedded subtitle content
	MetadataCache        = cache.NewCache[float64](24 * time.Hour)               // media duration from ffprobe
)
