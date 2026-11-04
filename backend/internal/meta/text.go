package meta

import (
	"bytes"
	"context"
	"io"
	"os"
	"unicode/utf8"
)

// textLimit is the largest text file whose lines and words are counted.
// Beyond it only the encoding is told, a log of many gigabytes would keep
// the list waiting.
const textLimit = 256 << 20

func readText(ctx context.Context, _ *Service, f File, rec Record) error {
	fh, err := os.Open(f.Real)
	if err != nil {
		return err
	}
	defer fh.Close()
	buf := make([]byte, 256<<10)
	n, err := io.ReadFull(fh, buf)
	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		return err
	}
	rec["encoding"] = encodingOf(buf[:n], int64(n) == f.Size)
	if f.Size > textLimit {
		return nil
	}
	var c counter
	c.add(buf[:n])
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		n, err := fh.Read(buf)
		c.add(buf[:n])
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}
	rec["lines"], rec["words"] = c.result()
	return nil
}

// encodingOf names the encoding of the start of a text. whole tells that
// the sample is the entire file, so a character cut at its end is real.
func encodingOf(b []byte, whole bool) string {
	switch {
	case bytes.HasPrefix(b, []byte{0xEF, 0xBB, 0xBF}):
		return "UTF-8 mit BOM"
	case bytes.HasPrefix(b, []byte{0xFF, 0xFE}):
		return "UTF-16 LE"
	case bytes.HasPrefix(b, []byte{0xFE, 0xFF}):
		return "UTF-16 BE"
	}
	ascii := true
	for _, c := range b {
		if c >= 0x80 {
			ascii = false
			break
		}
	}
	if ascii {
		return "ASCII"
	}
	if i := lastStart(b); !whole && !utf8.FullRune(b[i:]) {
		b = b[:i]
	}
	if utf8.Valid(b) {
		return "UTF-8"
	}
	return "Windows-1252"
}

func lastStart(b []byte) int {
	for i := len(b) - 1; i >= 0 && i >= len(b)-utf8.UTFMax; i-- {
		if utf8.RuneStart(b[i]) {
			return i
		}
	}
	return len(b)
}

// counter counts lines and words across chunks, the way wc does, with a
// last line that lacks its line break still counted.
type counter struct {
	lines, words int64
	inWord       bool
	last         byte
	any          bool
}

func (c *counter) add(b []byte) {
	if len(b) == 0 {
		return
	}
	c.any = true
	c.lines += int64(bytes.Count(b, []byte{'\n'}))
	for _, x := range b {
		space := x == ' ' || x == '\n' || x == '\t' || x == '\r' || x == '\v' || x == '\f'
		if !space && !c.inWord {
			c.words++
		}
		c.inWord = !space
	}
	c.last = b[len(b)-1]
}

func (c *counter) result() (lines, words int64) {
	lines = c.lines
	if c.any && c.last != '\n' {
		lines++
	}
	return lines, c.words
}
