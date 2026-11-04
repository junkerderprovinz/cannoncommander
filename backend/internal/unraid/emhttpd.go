package unraid

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

// ErrWriteOff is returned while write access to Unraid is switched off.
var ErrWriteOff = errors.New("unraid: write access is switched off")

// ErrUnknownDisk is returned for a disk name the array does not have.
var ErrUnknownDisk = errors.New("unraid: unknown disk")

// Spindown spins the named disks down, or every disk when names is empty.
// It talks to emhttpd the way the emcmd tool does.
func (s *Service) Spindown(ctx context.Context, names []string) ([]string, error) {
	if !s.cfg.Write {
		return nil, ErrWriteOff
	}
	st, err := s.State()
	if err != nil {
		return nil, err
	}
	if len(names) == 0 {
		if err := s.emcmd(ctx, "cmdSpindownAll=Apply"); err != nil {
			return nil, err
		}
		s.fresh()
		return []string{}, nil
	}
	for _, n := range names {
		d, ok := st.Disk(n)
		if !ok || d.Kind == "flash" || d.Pool != "" {
			return nil, fmt.Errorf("%w: %s", ErrUnknownDisk, n)
		}
	}
	done := []string{}
	for _, n := range names {
		if slices.Contains(done, n) {
			continue
		}
		if err := s.emcmd(ctx, "cmdSpindown="+url.QueryEscape(n)); err != nil {
			return done, err
		}
		done = append(done, n)
	}
	s.fresh()
	return done, nil
}

// emcmd posts one command to emhttpd. The socket wants the csrf token
// from var.ini with every command and answers with text only on failure.
func (s *Service) emcmd(ctx context.Context, cmd string) error {
	vars, err := readSections(filepath.Join(s.cfg.Emhttp, "var.ini"))
	if err != nil {
		return err
	}
	token := ""
	if len(vars) > 0 {
		token = vars[0].str("csrf_token")
	}
	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, "unix", s.cfg.Socket)
		}},
	}
	body := strings.NewReader(cmd + "&csrf_token=" + url.QueryEscape(token))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://localhost/update", body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("emhttpd: %w", err)
	}
	defer resp.Body.Close()
	reply, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("emhttpd: %s", resp.Status)
	}
	if msg := strings.TrimSpace(string(reply)); msg != "" {
		return fmt.Errorf("emhttpd: %s", msg)
	}
	return nil
}

// Notice is one Unraid notification. Importance is normal, warning or
// alert.
type Notice struct {
	Subject     string `json:"subject"`
	Description string `json:"description"`
	Importance  string `json:"importance,omitempty"`
	Link        string `json:"link,omitempty"`
}

var noticeSeq atomic.Uint64

// Notify puts a notice into the Unraid notifications. It writes the files
// the notify script writes, so the notice shows in the web interface. The
// notification agents only run from that script on the host and stay out
// of reach of a container.
func (s *Service) Notify(n Notice) error {
	if !s.cfg.Write {
		return ErrWriteOff
	}
	switch n.Importance {
	case "":
		n.Importance = "normal"
	case "normal", "warning", "alert":
	default:
		return fmt.Errorf("unraid: importance %q", n.Importance)
	}
	now := time.Now().Unix()
	name := fmt.Sprintf("CannonCommander_%d_%d.notify", now, noticeSeq.Add(1))
	fields := [][2]string{
		{"timestamp", strconv.FormatInt(now, 10)},
		{"event", "CannonCommander"},
		{"subject", n.Subject},
		{"description", n.Description},
		{"importance", n.Importance},
	}
	if err := writeNotice(filepath.Join(s.cfg.Notifications, "archive", name), fields); err != nil {
		return err
	}
	return writeNotice(filepath.Join(s.cfg.Notifications, "unread", name), append(fields, [2]string{"link", n.Link}))
}

// writeNotice writes through a temporary name, because the web interface
// reads every *.notify file it finds and must never see half of one.
func writeNotice(p string, fields [][2]string) error {
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	var b strings.Builder
	for _, f := range fields {
		if f[0] == "timestamp" {
			fmt.Fprintf(&b, "%s=%s\n", f[0], f[1])
			continue
		}
		fmt.Fprintf(&b, "%s=\"%s\"\n", f[0], iniValue(f[1]))
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, []byte(b.String()), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}

// iniValue keeps a value on one line and free of the quotes that would end
// it early in parse_ini_file.
func iniValue(v string) string {
	return strings.NewReplacer("\r", " ", "\n", " ", `"`, "'", `\`, "/").Replace(v)
}
