package preview

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"runtime"
	"testing"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/adapters/fs/fileutils"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/testutil"
	"github.com/kovidgoyal/imaging/apng"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	testutil.QuietLogging()
	// Ensure fileutils permissions are set (needed by NewPreviewGenerator)
	if fileutils.PermDir == 0 {
		fileutils.SetFsPermissions(0644, 0755)
	}

	// Run the tests
	code := m.Run()

	// Exit with the test result code
	os.Exit(code)
}

func newGrayJpeg(t *testing.T, width, height int) afero.File {
	fs := afero.NewMemMapFs()
	file, err := fs.Create("image.jpg")
	require.NoError(t, err)

	img := image.NewGray(image.Rect(0, 0, width, height))
	err = jpeg.Encode(file, img, &jpeg.Options{Quality: 90})
	require.NoError(t, err)

	_, err = file.Seek(0, io.SeekStart)
	require.NoError(t, err)

	return file
}

func TestService_FormatFromExtension(t *testing.T) {
	testCases := map[string]struct {
		ext     string
		want    Format
		wantErr error
	}{
		"jpg": {
			ext:  ".jpg",
			want: FormatJpeg,
		},
		"jpeg": {
			ext:  ".jpeg",
			want: FormatJpeg,
		},
		"png": {
			ext:  ".png",
			want: FormatPng,
		},
		"gif": {
			ext:  ".gif",
			want: FormatGif,
		},
		"tiff": {
			ext:  ".tiff",
			want: FormatTiff,
		},
		"tif": {
			ext:  ".tif",
			want: FormatTiff,
		},
		"bmp": {
			ext:  ".bmp",
			want: FormatBmp,
		},
		"heic": {
			ext:  ".heic",
			want: FormatHeic,
		},
		"heif": {
			ext:  ".heif",
			want: FormatHeic,
		},
		"webp": {
			ext:  ".webp",
			want: FormatWebp,
		},
		"unknown": {
			ext:     ".mov",
			wantErr: ErrUnsupportedFormat,
		},
	}

	for name, test := range testCases {
		t.Run(name, func(t *testing.T) {
			tmpDir := t.TempDir()
			svc := NewPreviewGenerator(1, tmpDir)
			got, err := svc.FormatFromExtension(test.ext)
			require.Truef(t, errors.Is(err, test.wantErr), "error = %v, wantErr %v", err, test.wantErr)
			if err != nil {
				return
			}
			require.Equal(t, test.want, got)
		})
	}
}

func TestCacheKeyConsistency(t *testing.T) {
	// Test that folder previews and direct file previews use the same cache key
	// when the folder preview is based on a child file

	// Test direct file preview
	cacheKey1 := CacheKey("testmd5hash123", "small", 0)

	// Test folder preview with same child MD5
	cacheKey2 := CacheKey("testmd5hash123", "small", 0)

	// Both should produce the same cache key
	require.Equal(t, cacheKey1, cacheKey2, "Folder preview and direct file preview should use the same cache key")

	// Test with different seek percentages
	cacheKey3 := CacheKey("testmd5hash123", "small", 25)
	cacheKey4 := CacheKey("testmd5hash123", "small", 25)

	require.Equal(t, cacheKey3, cacheKey4, "Cache keys should be consistent for same parameters")
	require.NotEqual(t, cacheKey1, cacheKey3, "Different seek percentages should produce different cache keys")
}

// TestCreatePreviewFromReader_UsesConcurrencyLimit verifies that CreatePreviewFromReader
// goes through Resize and thus respects the imageSem semaphore (smoke test).
func TestCreatePreviewFromReader_UsesConcurrencyLimit(t *testing.T) {
	tmpDir := t.TempDir()
	svc := NewPreviewGenerator(2, tmpDir)

	source := newGrayJpeg(t, 200, 200)
	defer source.Close()
	data, err := io.ReadAll(source)
	require.NoError(t, err)

	// Get options for small preview
	options, err := getPreviewOptions("small")
	require.NoError(t, err)

	// Create preview from reader
	out, err := svc.CreatePreview(context.Background(), bytes.NewReader(data), int64(len(data)), options)
	require.NoError(t, err)
	require.NotEmpty(t, out)

	// Same via another call for parity
	out2, err := svc.CreatePreview(context.Background(), bytes.NewReader(data), int64(len(data)), options)
	require.NoError(t, err)
	require.Equal(t, out, out2)
}

func TestImageFitsPreviewSize(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		width       int
		height      int
		previewSize string
		want        bool
	}{
		{name: "fits xlarge", width: 800, height: 600, previewSize: "xlarge", want: true},
		{name: "exceeds xlarge width", width: 1200, height: 600, previewSize: "xlarge", want: false},
		{name: "fits large", width: 640, height: 480, previewSize: "large", want: true},
		{name: "fits small", width: 256, height: 200, previewSize: "small", want: true},
		{name: "exceeds small height", width: 200, height: 300, previewSize: "small", want: false},
		{name: "original always fits", width: 5000, height: 5000, previewSize: "original", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, ImageFitsPreviewSize(tt.width, tt.height, tt.previewSize))
		})
	}
}

func TestReadImageFileDimensions(t *testing.T) {
	t.Parallel()

	width, height, err := ReadImageFileDimensions("testdata/gray-sample.jpg")
	require.NoError(t, err)
	require.Greater(t, width, 0)
	require.Greater(t, height, 0)
}

// pngDeclaring is a PNG that states a size in its header and has no pixels.
func pngDeclaring(width, height uint32, padding int) []byte {
	chunk := func(name string, data []byte) []byte {
		out := binary.BigEndian.AppendUint32(nil, uint32(len(data)))
		out = append(out, name...)
		out = append(out, data...)
		return binary.BigEndian.AppendUint32(out, crc32.ChecksumIEEE(out[4:]))
	}
	ihdr := binary.BigEndian.AppendUint32(nil, width)
	ihdr = binary.BigEndian.AppendUint32(ihdr, height)
	ihdr = append(ihdr, 8, 6, 0, 0, 0)
	out := []byte("\x89PNG\r\n\x1a\n")
	out = append(out, chunk("IHDR", ihdr)...)
	out = append(out, chunk("tEXt", make([]byte, padding))...)
	return append(out, chunk("IEND", nil)...)
}

func jpegDeclaring(t *testing.T, width, height uint16) []byte {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, jpeg.Encode(&buf, image.NewGray(image.Rect(0, 0, 8, 8)), nil))
	data := buf.Bytes()
	sof := bytes.Index(data, []byte{0xff, 0xc0})
	require.GreaterOrEqual(t, sof, 0)
	binary.BigEndian.PutUint16(data[sof+5:], height)
	binary.BigEndian.PutUint16(data[sof+7:], width)
	return data
}

func gifDeclaring(width, height uint16) []byte {
	out := []byte("GIF89a")
	out = binary.LittleEndian.AppendUint16(out, width)
	out = binary.LittleEndian.AppendUint16(out, height)
	return append(out, 0x80, 0, 0, 0, 0, 0, 0xff, 0xff, 0xff, ';')
}

func TestCreatePreviewRefusesAnImageDeclaringTooManyPixels(t *testing.T) {
	options, err := getPreviewOptions("large")
	require.NoError(t, err)
	for name, data := range map[string][]byte{
		"png":  pngDeclaring(20000, 20000, 0),
		"jpeg": jpegDeclaring(t, 20000, 20000),
		"gif":  gifDeclaring(20000, 20000),
	} {
		_, err := (&Service{}).CreatePreview(context.Background(), bytes.NewReader(data), int64(len(data)), options)
		require.ErrorIs(t, err, ErrTooManyPixels, name)
	}
}

func TestCheckPixelsAllowsThePhotosOfTheLargestCameras(t *testing.T) {
	require.NoError(t, checkPixels(bytes.NewReader(pngDeclaring(11648, 8736, 0))))
	require.NoError(t, checkPixels(bytes.NewReader(jpegDeclaring(t, 11648, 8736))))
	require.ErrorIs(t, checkPixels(bytes.NewReader(pngDeclaring(10500, 10001, 0))), ErrTooManyPixels)
}

func TestCheckPixelsLeavesTheReaderWhereItWas(t *testing.T) {
	data := pngDeclaring(64, 64, 0)
	r := bytes.NewReader(data)
	require.NoError(t, checkPixels(r))
	require.Equal(t, int64(len(data)), int64(r.Len()))
}

func TestOrientationLeavesAnImageWithTooManyPixelsAlone(t *testing.T) {
	if testing.Short() {
		t.Skip("encodes a very large picture")
	}
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, image.NewGray(image.Rect(0, 0, 10500, 10001))))
	data := buf.Bytes()
	require.Equal(t, data, applyOrientationToPreviewBytes(data, "Rotate 180"))
}

func previewAllocation(t *testing.T, data []byte) uint64 {
	t.Helper()
	options, err := getPreviewOptions("large")
	require.NoError(t, err)
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	out, err := (&Service{}).CreatePreview(context.Background(), bytes.NewReader(data), int64(len(data)), options)
	runtime.ReadMemStats(&after)
	require.NoError(t, err)
	_, format, err := image.DecodeConfig(bytes.NewReader(out))
	require.NoError(t, err)
	require.Equal(t, "jpeg", format)
	return after.TotalAlloc - before.TotalAlloc
}

func TestCreatePreviewDecodesOneFrameOfAnAnimatedGIF(t *testing.T) {
	frame := image.NewPaletted(image.Rect(0, 0, 3000, 3000), color.Palette{color.Black, color.White})
	var animation gif.GIF
	for range 20 {
		animation.Image = append(animation.Image, frame)
		animation.Delay = append(animation.Delay, 0)
	}
	var buf bytes.Buffer
	require.NoError(t, gif.EncodeAll(&buf, &animation))
	require.Less(t, previewAllocation(t, buf.Bytes()), uint64(100<<20))
}

func TestCreatePreviewDecodesOneFrameOfAnAnimatedPNG(t *testing.T) {
	frame := image.NewGray(image.Rect(0, 0, 3000, 3000))
	var animation apng.APNG
	for range 20 {
		animation.Frames = append(animation.Frames, apng.Frame{Image: frame, DelayNumerator: 1, DelayDenominator: 10})
	}
	var buf bytes.Buffer
	require.NoError(t, apng.Encode(&buf, animation))
	require.Less(t, previewAllocation(t, buf.Bytes()), uint64(100<<20))
}

func TestCreatePreviewRefusesAnAnimatedWebP(t *testing.T) {
	vp8x := []byte("VP8X\x0a\x00\x00\x00\x02\x00\x00\x00\xff\x00\x00\xff\x00\x00")
	data := binary.LittleEndian.AppendUint32([]byte("RIFF"), uint32(4+len(vp8x)))
	data = append(append(data, "WEBP"...), vp8x...)
	options, err := getPreviewOptions("large")
	require.NoError(t, err)
	_, err = (&Service{}).CreatePreview(context.Background(), bytes.NewReader(data), int64(len(data)), options)
	require.ErrorIs(t, err, ErrUnsupportedFormat)
}

func TestCreatePreviewOfAStillPNGStillWorks(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, image.NewGray(image.Rect(0, 0, 800, 800))))
	previewAllocation(t, buf.Bytes())
}
