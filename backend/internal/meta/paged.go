package meta

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// infoScript asks MuPDF for the page count and the document information.
// MuPDF decodes the PDF string encodings, which a hand-written parser would
// get wrong for half the files around.
const infoScript = `var D = typeof mupdf !== "undefined" ? mupdf.Document : Document;
var doc = D.openDocument(scriptArgs[0]);
var out = { locked: doc.needsPassword() };
if (!out.locked) {
	out.pages = doc.countPages();
	out.title = doc.getMetaData("info:Title");
	out.author = doc.getMetaData("info:Author");
	out.subject = doc.getMetaData("info:Subject");
	out.creator = doc.getMetaData("info:Creator");
	out.producer = doc.getMetaData("info:Producer");
	out.encryption = doc.getMetaData("encryption");
}
print(JSON.stringify(out));
`

var scriptFile = sync.OnceValues(func() (string, error) {
	dir, err := os.MkdirTemp("", "cannoncommander-meta-")
	if err != nil {
		return "", err
	}
	p := filepath.Join(dir, "info.js")
	return p, os.WriteFile(p, []byte(infoScript), 0o600)
})

type docInfo struct {
	Locked     bool   `json:"locked"`
	Pages      int64  `json:"pages"`
	Title      string `json:"title"`
	Author     string `json:"author"`
	Subject    string `json:"subject"`
	Creator    string `json:"creator"`
	Producer   string `json:"producer"`
	Encryption string `json:"encryption"`
}

func (s *Service) docInfo(ctx context.Context, real string) (docInfo, error) {
	var d docInfo
	mutool, ok := s.tools.find("mutool")
	if !ok {
		return d, errNoTool
	}
	script, err := scriptFile()
	if err != nil {
		return d, err
	}
	out, err := run(ctx, mutool, "run", script, real)
	if err != nil && len(out) == 0 {
		return d, err
	}
	// MuPDF prints warnings about damaged files before the answer.
	text := strings.TrimSpace(string(out))
	if i := strings.LastIndexByte(text, '\n'); i >= 0 {
		text = text[i+1:]
	}
	if err := json.Unmarshal([]byte(text), &d); err != nil {
		return d, fmt.Errorf("mutool output: %w", err)
	}
	return d, nil
}

func readPaged(ctx context.Context, s *Service, f File, rec Record) error {
	d, err := s.docInfo(ctx, f.Real)
	if err != nil {
		return err
	}
	rec["encrypted"] = d.Locked || (d.Encryption != "" && d.Encryption != "None")
	if d.Pages > 0 {
		rec["pages"] = d.Pages
	}
	rec["doctitle"] = strings.TrimSpace(d.Title)
	rec["author"] = strings.TrimSpace(d.Author)
	rec["subject"] = strings.TrimSpace(d.Subject)
	rec["app"] = strings.TrimSpace(d.Creator)
	if rec["app"] == "" {
		rec["app"] = strings.TrimSpace(d.Producer)
	}
	return nil
}
