package fsapi

import (
	"bytes"
	"net/http"
	"strings"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/unicode"
)

// charsets are the single-byte encodings the editor reads and writes besides
// ASCII and Unicode: those older tools on Windows and Linux write in Europe
// and Russia.
var charsets = map[string]*charmap.Charmap{
	"windows-1250": charmap.Windows1250,
	"windows-1251": charmap.Windows1251,
	"windows-1252": charmap.Windows1252,
	"iso-8859-1":   charmap.ISO8859_1,
	"iso-8859-2":   charmap.ISO8859_2,
	"iso-8859-15":  charmap.ISO8859_15,
	"koi8-r":       charmap.KOI8R,
}

var (
	errUnknownEncoding = v1.Coded(http.StatusBadRequest, "unknownEncoding",
		"encoding must be ascii, utf-8, utf-8-bom, utf-16le, utf-16be, windows-1250, windows-1251, windows-1252, iso-8859-1, iso-8859-2, iso-8859-15 or koi8-r")
	errNotEncodable = v1.Coded(http.StatusUnprocessableEntity, "notEncodable", "the text holds characters this encoding cannot store, save it as UTF-8 instead")
)

var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

// readText decodes buf in the encoding asked for, or finds the encoding when
// none is asked for.
func readText(buf []byte, asked string, truncated bool) (encoding, text string, err error) {
	if asked == "" {
		return decodeText(buf, truncated)
	}
	text, err = decodeAs(buf, asked, truncated)
	return asked, text, err
}

// decodeAs reads buf as encoding, for a file whose encoding the guess got
// wrong. A byte order mark of that encoding is dropped, and bytes that do not
// fit it become replacement characters.
func decodeAs(buf []byte, encoding string, truncated bool) (string, error) {
	switch encoding {
	case "ascii", "utf-8", "utf-8-bom":
		return strings.ToValidUTF8(string(trimRune(bytes.TrimPrefix(buf, utf8BOM), truncated)), "\uFFFD"), nil
	case "utf-16le", "utf-16be":
		order, bom := unicode.LittleEndian, []byte{0xFF, 0xFE}
		if encoding == "utf-16be" {
			order, bom = unicode.BigEndian, []byte{0xFE, 0xFF}
		}
		buf = bytes.TrimPrefix(buf, bom)
		buf = buf[:len(buf)&^1]
		out, err := unicode.UTF16(order, unicode.IgnoreBOM).NewDecoder().Bytes(buf)
		return string(out), err
	}
	cs, ok := charsets[encoding]
	if !ok {
		return "", errUnknownEncoding
	}
	out, err := cs.NewDecoder().Bytes(buf)
	return string(out), err
}

// encodeText writes text in encoding, which the editor either read the file
// in or was asked to convert it to.
func encodeText(text, encoding string) ([]byte, error) {
	switch encoding {
	case "utf-8", "ascii":
		return []byte(text), nil
	case "utf-8-bom":
		return append(bytes.Clone(utf8BOM), text...), nil
	case "utf-16le", "utf-16be":
		order := unicode.LittleEndian
		if encoding == "utf-16be" {
			order = unicode.BigEndian
		}
		return unicode.UTF16(order, unicode.UseBOM).NewEncoder().Bytes([]byte(text))
	}
	cs, ok := charsets[encoding]
	if !ok {
		return nil, errUnknownEncoding
	}
	out, err := cs.NewEncoder().Bytes([]byte(text))
	if err != nil {
		return nil, errNotEncodable.With("encoding", charsetName(encoding))
	}
	return out, nil
}

// charsetName writes an encoding the way people know it, Windows-1252 or
// ISO-8859-15.
func charsetName(encoding string) string {
	if n, ok := strings.CutPrefix(encoding, "windows-"); ok {
		return "Windows-" + n
	}
	return strings.ToUpper(encoding)
}
