package preview

import (
	"bytes"
	"context"
	"fmt"
	"os"

	"github.com/gtsteffaniak/go-ffmpeg/ops"
	"github.com/gtsteffaniak/go-logger/logger"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/ffmpeg"
)

// GenerateVideoPreview generates a single preview image from a video using ffmpeg.
// videoPath: path to the input video file.
// previewSize: small, large, xlarge, or original.
// percentageSeek: percentage of video duration to seek to (0–100).
// Returns: JPEG image bytes.
func (s *Service) GenerateVideoPreview(ctx context.Context, videoPath, previewSize string, percentageSeek int) ([]byte, error) {
	svc := ffmpeg.Get()
	if svc == nil {
		logger.Errorf("FFmpeg service not available for file '%s'", videoPath)
		return nil, fmt.Errorf("FFmpeg service not available")
	}

	// Validate file exists before processing
	if _, err := os.Stat(videoPath); err != nil {
		logger.Errorf("Video file does not exist or is not accessible: '%s': %v", videoPath, err)
		return nil, fmt.Errorf("video file not accessible: %w", err)
	}

	params, err := ffmpegPreviewParamsForSize(previewSize)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	err = svc.VideoPreview(ctx, &buf, ops.PreviewOptions{
		Input:       videoPath,
		SeekPercent: float64(percentageSeek),
		Width:       params.width,
		Height:      params.height,
		ScaleMode:   params.scaleMode,
		Quality:     params.quality,
	})
	if err != nil {
		return nil, err
	}

	previewBytes := buf.Bytes()
	if len(previewBytes) == 0 {
		logger.Errorf("FFmpeg service returned empty result for '%s'", videoPath)
		return nil, fmt.Errorf("video preview generation returned empty result")
	}

	return previewBytes, nil
}
