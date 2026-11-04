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

func readPaged(ctx context.Context, s *Service, f File, rec Record) error {
	mutool, ok := s.tools.find("mutool")
	if !ok {
		return errNoTool
	}
	script, err := scriptFile()
	if err != nil {
		return err
	}
	out, err := run(ctx, mutool, "run", script, f.Real)
	if err != nil && len(out) == 0 {
		return err
	}
	var d docInfo
	// MuPDF prints warnings about damaged files before the answer.
	line := out
	if i := strings.LastIndexByte(strings.TrimSpace(string(out)), '\n'); i >= 0 {
		line = []byte(strings.TrimSpace(string(out))[i+1:])
	}
	if err := json.Unmarshal(line, &d); err != nil {
		return fmt.Errorf("mutool output: %w", err)
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
