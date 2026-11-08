package ffmpeg

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync/atomic"

	goffmpeg "github.com/gtsteffaniak/go-ffmpeg"
	"github.com/gtsteffaniak/go-ffmpeg/capabilities"
	"github.com/gtsteffaniak/go-ffmpeg/ops"
	"github.com/gtsteffaniak/go-logger/logger"
)

// Service wraps go-ffmpeg for media operations.
type Service struct {
	inner    *goffmpeg.Service
	cacheDir string
}

// FFmpegService is kept for existing callers.
type FFmpegService = Service

var global atomic.Pointer[Service]

// probing is the end of a look for ffmpeg that runs in the background, nil
// while none does.
var probing atomic.Pointer[chan struct{}]

// Probing marks Initialize as running in the background, so Wait holds
// until the returned function says it has run.
func Probing() (done func()) {
	ch := make(chan struct{})
	probing.Store(&ch)
	return func() {
		probing.Store(nil)
		close(ch)
	}
}

// Checking reports whether a look for ffmpeg still runs.
func Checking() bool { return probing.Load() != nil }

// Wait returns the service once a look that runs has ended, or nil when ctx
// ends first or there is no ffmpeg.
func Wait(ctx context.Context) *Service {
	if p := probing.Load(); p != nil {
		select {
		case <-*p:
		case <-ctx.Done():
			return nil
		}
	}
	return Get()
}

// InitOptions configures startup initialization.
type InitOptions struct {
	FFmpegPath           string
	MaxConcurrent        int // deprecated: use Concurrency
	Concurrency          goffmpeg.Concurrency
	CacheDir             string
	SkipHWTests          bool
	HardwareAcceleration bool
}

// Initialize creates the global ffmpeg service and runs capability detection.
func Initialize(ctx context.Context, opts InitOptions) error {
	if opts.CacheDir == "" {
		opts.CacheDir = os.TempDir()
	}

	if opts.HardwareAcceleration {
		logger.Info("Detecting ffmpeg hardware codec support...")
	}

	cfg := goffmpeg.Config{
		FFmpegPath:  opts.FFmpegPath,
		Concurrency: opts.Concurrency,
		Logger:      goffmpeg.NopLogger(),
		SkipHWTests: opts.SkipHWTests,
	}
	if opts.Concurrency.MaxProbe == 0 && opts.Concurrency.MaxDecode == 0 && opts.Concurrency.MaxEncode == 0 {
		if opts.MaxConcurrent < 1 {
			opts.MaxConcurrent = 4
		}
		cfg.MaxConcurrent = opts.MaxConcurrent
	}

	svc, err := goffmpeg.New(ctx, cfg)
	if err != nil {
		global.Store(nil)
		return err
	}

	global.Store(&Service{
		inner:    svc,
		cacheDir: opts.CacheDir,
	})

	logCapabilities(svc, opts.HardwareAcceleration)
	return nil
}

// Get returns the initialized service, or nil when ffmpeg is unavailable.
func Get() *Service {
	return global.Load()
}

// Enabled reports whether ffmpeg initialized successfully.
func Enabled() bool {
	s := global.Load()
	return s != nil && s.inner != nil
}

// FFmpegPath returns the resolved ffmpeg binary path.
func (s *Service) FFmpegPath() string {
	if s == nil || s.inner == nil {
		return ""
	}
	return s.inner.FFmpegPath()
}

// FFprobePath returns the resolved ffprobe binary path.
func (s *Service) FFprobePath() string {
	if s == nil || s.inner == nil {
		return ""
	}
	return s.inner.FFprobePath()
}

func (s *Service) Acquire(ctx context.Context) error {
	if s == nil || s.inner == nil {
		return fmt.Errorf("ffmpeg service not available")
	}
	return s.inner.Acquire(ctx)
}

func (s *Service) Release() {
	if s == nil || s.inner == nil {
		return
	}
	s.inner.Release()
}

// VideoPreview extracts a JPEG preview frame to w.
// go-ffmpeg Service methods acquire concurrency slots automatically; do not Acquire before calling.
func (s *Service) VideoPreview(ctx context.Context, w io.Writer, opts ops.PreviewOptions) error {
	if s == nil || s.inner == nil {
		return fmt.Errorf("ffmpeg service not available")
	}
	return s.inner.VideoPreview(ctx, w, opts)
}

func logCapabilities(svc *goffmpeg.Service, detectHardware bool) {
	caps := svc.Capabilities()
	if caps == nil {
		return
	}

	logger.Debugf("Media Enabled: version %s @ %s", caps.FFmpegVersion, caps.FFmpegPath)

	if !detectHardware {
		return
	}

	hw := hardwareCodecSummary(svc)
	if hw == "" {
		logger.Warning("no ffmpeg hardware codec support found")
		return
	}
	logger.Debugf("supported ffmpeg hardware codecs: %s", hw)
}

func hardwareCodecSummary(svc *goffmpeg.Service) string {
	seen := make(map[string]struct{})
	var parts []string

	add := func(entry string) {
		if entry == "" {
			return
		}
		if _, ok := seen[entry]; ok {
			return
		}
		seen[entry] = struct{}{}
		parts = append(parts, entry)
	}

	for _, opt := range svc.AvailableEncodeOptions() {
		if opt.Accel == capabilities.AccelNone {
			continue
		}
		add(fmt.Sprintf("%s encode via %s (%s)", opt.Codec, opt.Encoder, capabilities.AccelLabel(opt.Accel)))
	}
	for _, opt := range svc.AvailableDecodeOptions() {
		if opt.Accel == capabilities.AccelNone {
			continue
		}
		add(fmt.Sprintf("%s decode via %s (%s)", opt.Codec, opt.Decoder, capabilities.AccelLabel(opt.Accel)))
	}

	return strings.Join(parts, ", ")
}
