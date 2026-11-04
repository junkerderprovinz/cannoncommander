package meta

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
)

// Page widths in pixels. The viewer asks for the width it shows, a phone
// for less, a 4K screen for more.
const (
	DefaultPageWidth = 1200
	MaxPageWidth     = 4096
)

// ErrNoPage means the document has no page with that number or cannot be
// drawn at all.
var ErrNoPage = errors.New("the document has no such page")

// ErrNoRenderer means MuPDF is not installed.
var ErrNoRenderer = errors.New("MuPDF is not installed")

// Page draws one page of a document as a PNG, counted from 1.
func (s *Service) Page(ctx context.Context, real string, page, width int) ([]byte, error) {
	mutool, ok := s.tools.find("mutool")
	if !ok {
		return nil, ErrNoRenderer
	}
	// MuPDF draws the last page for a number past the end instead of
	// failing, so the count is checked first.
	info, err := s.docInfo(ctx, real)
	if err != nil {
		return nil, errors.Join(ErrNoPage, err)
	}
	if info.Locked || page > int(info.Pages) {
		return nil, ErrNoPage
	}
	dir, err := os.MkdirTemp("", "cannoncommander-page-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	out := filepath.Join(dir, "page.png")
	select {
	case s.slots <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	defer func() { <-s.slots }()
	_, err = run(ctx, mutool, "draw", "-F", "png", "-w", strconv.Itoa(width), "-o", out, real, strconv.Itoa(page))
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	data, readErr := os.ReadFile(out)
	if readErr != nil || len(data) == 0 {
		if err == nil {
			err = readErr
		}
		return nil, errors.Join(ErrNoPage, err)
	}
	return data, nil
}
