package meta

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/xml"
	"io"
	"os"
	"strings"
)

// oleMagic starts a compound file. An Office file in that shape instead of
// a zip is one that was saved with a password.
var oleMagic = []byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1}

type ooxmlCore struct {
	Title          string `xml:"title"`
	Subject        string `xml:"subject"`
	Creator        string `xml:"creator"`
	LastModifiedBy string `xml:"lastModifiedBy"`
}

type ooxmlApp struct {
	Application string `xml:"Application"`
	Pages       int64  `xml:"Pages"`
	Slides      int64  `xml:"Slides"`
	Words       int64  `xml:"Words"`
}

type odfMeta struct {
	Meta struct {
		Generator      string `xml:"generator"`
		Title          string `xml:"title"`
		Subject        string `xml:"subject"`
		InitialCreator string `xml:"initial-creator"`
		Creator        string `xml:"creator"`
		Statistic      struct {
			Pages int64 `xml:"page-count,attr"`
			Words int64 `xml:"word-count,attr"`
		} `xml:"document-statistic"`
	} `xml:"meta"`
}

// maxXML caps one XML part, the parts read here are a few kilobytes.
const maxXML = 4 << 20

func readOffice(_ context.Context, _ *Service, f File, rec Record) error {
	fh, err := os.Open(f.Real)
	if err != nil {
		return err
	}
	defer fh.Close()
	head := make([]byte, len(oleMagic))
	if _, err := io.ReadFull(fh, head); err != nil {
		return err
	}
	if bytes.Equal(head, oleMagic) {
		rec["encrypted"] = true
		return nil
	}
	zr, err := zip.NewReader(fh, f.Size)
	if err != nil {
		return err
	}
	parts := map[string]*zip.File{}
	for _, zf := range zr.File {
		parts[zf.Name] = zf
	}
	if p := parts["meta.xml"]; p != nil {
		var m odfMeta
		if err := decodePart(p, &m); err != nil {
			return err
		}
		rec["doctitle"] = strings.TrimSpace(m.Meta.Title)
		rec["subject"] = strings.TrimSpace(m.Meta.Subject)
		rec["author"] = strings.TrimSpace(m.Meta.InitialCreator)
		rec["lastauthor"] = strings.TrimSpace(m.Meta.Creator)
		if rec["author"] == "" {
			rec["author"] = rec["lastauthor"]
		}
		rec["app"] = generator(m.Meta.Generator)
		if n := m.Meta.Statistic.Pages; n > 0 {
			rec["pages"] = n
		}
		if n := m.Meta.Statistic.Words; n > 0 {
			rec["words"] = n
		}
		rec["encrypted"] = false
		if mf := parts["META-INF/manifest.xml"]; mf != nil {
			if raw, err := readPart(mf); err == nil && bytes.Contains(raw, []byte("encryption-data")) {
				rec["encrypted"] = true
			}
		}
		return nil
	}
	if p := parts["docProps/core.xml"]; p != nil {
		var c ooxmlCore
		if err := decodePart(p, &c); err == nil {
			rec["doctitle"] = strings.TrimSpace(c.Title)
			rec["subject"] = strings.TrimSpace(c.Subject)
			rec["author"] = strings.TrimSpace(c.Creator)
			rec["lastauthor"] = strings.TrimSpace(c.LastModifiedBy)
		}
	}
	if p := parts["docProps/app.xml"]; p != nil {
		var a ooxmlApp
		if err := decodePart(p, &a); err == nil {
			rec["app"] = generator(a.Application)
			if n := max(a.Pages, a.Slides); n > 0 {
				rec["pages"] = n
			}
			if a.Words > 0 {
				rec["words"] = a.Words
			}
		}
	}
	rec["encrypted"] = false
	return nil
}

func readPart(zf *zip.File) ([]byte, error) {
	rc, err := zf.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(io.LimitReader(rc, maxXML))
}

func decodePart(zf *zip.File, v any) error {
	raw, err := readPart(zf)
	if err != nil {
		return err
	}
	return xml.Unmarshal(raw, v)
}

// generator shortens what LibreOffice writes about itself, such as
// "LibreOffice/7.6.4.1$Linux_X86_64 LibreOffice_project/...", to its name
// and version.
func generator(s string) string {
	s, _, _ = strings.Cut(strings.TrimSpace(s), "$")
	return strings.Replace(s, "/", " ", 1)
}
