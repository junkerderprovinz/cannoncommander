package mcpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
)

// Sizes of the answers. A listing pages through a folder, so even a folder
// with a hundred thousand entries reaches the assistant in pieces it can
// hold.
const (
	listDefault   = 200
	listMax       = 1000
	textDefault   = 64 << 10
	textMax       = 1 << 20
	searchDefault = 100
	searchMax     = 1000
	trashDefault  = 100
	trashMax      = 1000

	// contentWait is how long search_file_contents waits for its job before
	// it answers with state running.
	contentWait = 20 * time.Second
	jobPoll     = 250 * time.Millisecond
)

// apiItem is a row of a listing, of fs/stat and of a search hit, as /api/v1
// sends it. Only what an assistant needs goes on.
type apiItem struct {
	Name        string   `json:"name"`
	Path        string   `json:"path"`
	At          string   `json:"at"`
	Dir         bool     `json:"dir"`
	Size        int64    `json:"size"`
	SizeUnknown bool     `json:"sizeUnknown"`
	MTime       string   `json:"mtime"`
	BTime       string   `json:"btime"`
	Mode        string   `json:"mode"`
	Owner       string   `json:"owner"`
	Group       string   `json:"group"`
	Hidden      bool     `json:"hidden"`
	System      bool     `json:"system"`
	Link        string   `json:"link"`
	Files       *int64   `json:"files"`
	Dirs        *int64   `json:"dirs"`
	Devices     []string `json:"devices"`
	Line        int      `json:"line"`
}

// Item is one file or folder as the tools return it. A link says only that it
// is one: where it leads may be a path on the host.
type Item struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	Kind        string `json:"kind"`
	Size        int64  `json:"size"`
	SizeUnknown bool   `json:"sizeUnknown,omitempty"`
	Modified    string `json:"modified"`
	Hidden      bool   `json:"hidden,omitempty"`
	Link        bool   `json:"link,omitempty"`
	Line        int    `json:"line,omitempty"`
}

func itemOf(a apiItem, p string) Item {
	kind := "file"
	if a.Dir {
		kind = "folder"
	}
	return Item{Name: a.Name, Path: p, Kind: kind, Size: a.Size, SizeUnknown: a.SizeUnknown, Modified: a.MTime,
		Hidden: a.Hidden || a.System, Link: a.Link != "", Line: a.Line}
}

func (s *Server) getAccess(_ context.Context, c *caller, args json.RawMessage) *mcp.CallToolResult {
	if err := decodeArgs(args, &struct{}{}); err != nil {
		return badArgs(err)
	}
	return ok(map[string]any{
		"account":  c.user.Username,
		"admin":    c.user.Permissions.Admin,
		"key":      c.key.Label,
		"viaOAuth": c.key.Kind == kindOAuth,
		"canWrite": c.key.CanWrite,
		"limits": map[string]any{
			"entriesPerCall":         itemsPerCall,
			"entriesPerHour":         itemsPerHour,
			"entriesLeftThisHour":    s.writes.left(c.key.ID, s.now()),
			"filesAndFoldersPerCall": filesPerCall,
		},
	})
}

type apiTreeItem struct {
	Name   string `json:"name"`
	Path   string `json:"path"`
	Label  string `json:"label"`
	Free   int64  `json:"free"`
	Total  int64  `json:"total"`
	Remote string `json:"remote"`
}

// Source is a place the account can open.
type Source struct {
	Name   string `json:"name"`
	Path   string `json:"path"`
	Free   int64  `json:"free,omitempty"`
	Total  int64  `json:"total,omitempty"`
	Remote bool   `json:"remote,omitempty"`
}

// listSources walks the virtual folders from / down to where the sources,
// drives and remotes begin, which is what the folder tree shows at its top.
func (s *Server) listSources(ctx context.Context, c *caller, args json.RawMessage) *mcp.CallToolResult {
	if err := decodeArgs(args, &struct{}{}); err != nil {
		return badArgs(err)
	}
	out := []Source{}
	queue := []string{"/"}
	for len(queue) > 0 && len(out) < 200 {
		p := queue[0]
		queue = queue[1:]
		if loc, err := s.deps.FS.Resolve(p); err == nil && !loc.Virtual() {
			if s.call(ctx, c, http.MethodGet, "/fs/stat", "/fs/stat", url.Values{"path": {p}}, nil, nil) == nil {
				out = append(out, Source{Name: loc.Source.Name, Path: p})
			}
			continue
		}
		var tree struct {
			Items []apiTreeItem `json:"items"`
		}
		if err := s.call(ctx, c, http.MethodGet, "/fs/tree", "/fs/tree", url.Values{"path": {p}}, nil, &tree); err != nil {
			continue
		}
		for _, it := range tree.Items {
			if it.Label == "" && it.Remote == "" && len(queue) < 200 {
				queue = append(queue, it.Path)
				continue
			}
			name := it.Label
			if name == "" {
				name = it.Name
			}
			out = append(out, Source{Name: name, Path: it.Path, Free: it.Free, Total: it.Total, Remote: it.Remote != ""})
		}
	}
	return ok(map[string]any{"sources": out})
}

func (s *Server) listFolder(ctx context.Context, c *caller, args json.RawMessage) *mcp.CallToolResult {
	var in struct {
		Path          string `json:"path"`
		Offset        int    `json:"offset"`
		Limit         int    `json:"limit"`
		IncludeHidden bool   `json:"includeHidden"`
	}
	if err := decodeArgs(args, &in); err != nil {
		return badArgs(err)
	}
	if in.Limit <= 0 {
		in.Limit = listDefault
	}
	in.Limit = min(in.Limit, listMax)
	in.Offset = max(in.Offset, 0)
	var list struct {
		Path  string    `json:"path"`
		Items []apiItem `json:"items"`
	}
	if err := s.call(ctx, c, http.MethodGet, "/fs/list", "/fs/list", url.Values{"path": {in.Path}}, nil, &list); err != nil {
		return failure(err)
	}
	items := []Item{}
	for _, a := range list.Items {
		if (a.Hidden || a.System) && !in.IncludeHidden {
			continue
		}
		items = append(items, itemOf(a, path.Join(list.Path, a.Name)))
	}
	total := len(items)
	page := items[min(in.Offset, total):min(in.Offset+in.Limit, total)]
	return ok(map[string]any{"path": list.Path, "total": total, "offset": in.Offset, "items": page, "more": in.Offset+len(page) < total})
}

func (s *Server) getItem(ctx context.Context, c *caller, args json.RawMessage) *mcp.CallToolResult {
	var in struct {
		Path string `json:"path"`
	}
	if err := decodeArgs(args, &in); err != nil {
		return badArgs(err)
	}
	var st apiItem
	if err := s.call(ctx, c, http.MethodGet, "/fs/stat", "/fs/stat", url.Values{"path": {in.Path}}, nil, &st); err != nil {
		return failure(err)
	}
	out := map[string]any{
		"item":    itemOf(st, st.Path),
		"created": st.BTime,
		"mode":    st.Mode,
		"owner":   st.Owner,
		"group":   st.Group,
	}
	if st.Files != nil {
		out["filesBelow"] = *st.Files
	}
	if st.Dirs != nil {
		out["foldersBelow"] = *st.Dirs
	}
	if len(st.Devices) > 0 {
		out["disks"] = st.Devices
	}
	return ok(out)
}

func (s *Server) readText(ctx context.Context, c *caller, args json.RawMessage) *mcp.CallToolResult {
	var in struct {
		Path     string `json:"path"`
		MaxBytes int    `json:"maxBytes"`
	}
	if err := decodeArgs(args, &in); err != nil {
		return badArgs(err)
	}
	if in.MaxBytes <= 0 {
		in.MaxBytes = textDefault
	}
	in.MaxBytes = min(in.MaxBytes, textMax)
	var text struct {
		Path      string `json:"path"`
		Size      int64  `json:"size"`
		MTime     string `json:"mtime"`
		Encoding  string `json:"encoding"`
		Text      string `json:"text"`
		Truncated bool   `json:"truncated"`
	}
	q := url.Values{"path": {in.Path}, "max": {strconv.Itoa(in.MaxBytes)}}
	if err := s.call(ctx, c, http.MethodGet, "/fs/text", "/fs/text", q, nil, &text); err != nil {
		return failure(err)
	}
	return ok(map[string]any{"path": text.Path, "size": text.Size, "modified": text.MTime, "encoding": text.Encoding,
		"truncated": text.Truncated, "text": text.Text})
}

func (s *Server) searchFiles(ctx context.Context, c *caller, args json.RawMessage) *mcp.CallToolResult {
	var in struct {
		Path          string `json:"path"`
		Query         string `json:"query"`
		Type          string `json:"type"`
		Limit         int    `json:"limit"`
		IncludeHidden bool   `json:"includeHidden"`
	}
	if err := decodeArgs(args, &in); err != nil {
		return badArgs(err)
	}
	if in.Limit <= 0 {
		in.Limit = searchDefault
	}
	q := url.Values{"path": {in.Path}, "q": {in.Query}, "limit": {strconv.Itoa(min(in.Limit, searchMax))},
		"hidden": {strconv.FormatBool(in.IncludeHidden)}}
	if in.Type != "" {
		q.Set("type", in.Type)
	}
	var res struct {
		Items   []apiItem `json:"items"`
		Total   int       `json:"total"`
		Unknown []string  `json:"unknown"`
	}
	if err := s.call(ctx, c, http.MethodGet, "/search", "/search", q, nil, &res); err != nil {
		return failure(err)
	}
	if len(res.Unknown) > 0 {
		return toolError("unknown_fields", "these fields are unknown: "+strings.Join(res.Unknown, ", "))
	}
	return ok(map[string]any{"total": res.Total, "items": hits(res.Items)})
}

func hits(rows []apiItem) []Item {
	out := make([]Item, 0, len(rows))
	for _, a := range rows {
		out = append(out, itemOf(a, path.Join(a.At, a.Name)))
	}
	return out
}

// apiJob is a job as /api/v1 sends it.
type apiJob struct {
	ID          string          `json:"id"`
	Kind        string          `json:"kind"`
	Via         string          `json:"via"`
	State       string          `json:"state"`
	Name        string          `json:"name"`
	Dest        string          `json:"dest"`
	Count       int             `json:"count"`
	TotalBytes  int64           `json:"totalBytes"`
	DoneBytes   int64           `json:"doneBytes"`
	TotalFiles  int64           `json:"totalFiles"`
	DoneFiles   int64           `json:"doneFiles"`
	FailedItems int             `json:"failedItems"`
	Error       string          `json:"error"`
	ErrorCode   string          `json:"errorCode"`
	Undo        string          `json:"undo"`
	Result      json.RawMessage `json:"result"`
	CreatedAt   string          `json:"createdAt"`
	FinishedAt  string          `json:"finishedAt"`
}

func (j apiJob) open() bool {
	return j.State == "queued" || j.State == "running" || j.State == "paused" || j.State == "asking"
}

func (s *Server) job(ctx context.Context, c *caller, id string) (apiJob, error) {
	var j apiJob
	if id == "" || strings.ContainsAny(id, "/?#") {
		return j, errBadJob
	}
	err := s.call(ctx, c, http.MethodGet, "/jobs/{id}", "/jobs/"+id, nil, nil, &j)
	return j, err
}

var errBadJob = errors.New("bad job id")

func jobView(j apiJob) map[string]any {
	out := map[string]any{
		"jobId": j.ID, "kind": j.Kind, "state": j.State, "name": j.Name, "entries": j.Count,
		"files":     map[string]int64{"done": j.DoneFiles, "total": j.TotalFiles},
		"bytes":     map[string]int64{"done": j.DoneBytes, "total": j.TotalBytes},
		"createdAt": j.CreatedAt,
	}
	if j.Dest != "" {
		out["destination"] = j.Dest
	}
	if j.Via != "" {
		out["startedByKey"] = j.Via
	}
	if j.FailedItems > 0 {
		out["failedEntries"] = j.FailedItems
	}
	if j.Error != "" {
		out["error"] = scrub(j.Error)
	}
	if j.FinishedAt != "" {
		out["finishedAt"] = j.FinishedAt
	}
	if j.Undo != "" {
		out["undoable"] = true
	}
	return out
}

func (s *Server) getJob(ctx context.Context, c *caller, args json.RawMessage) *mcp.CallToolResult {
	var in struct {
		JobID string `json:"jobId"`
	}
	if err := decodeArgs(args, &in); err != nil {
		return badArgs(err)
	}
	j, err := s.job(ctx, c, in.JobID)
	if errors.Is(err, errBadJob) {
		return toolError("not_found", "there is no such job")
	}
	if err != nil {
		return failure(err)
	}
	return ok(jobView(j))
}

// searchContents starts a content search as the job CannonCommander runs for
// its search window, or collects one. Each key gets one at a time, since a
// content search reads every file below its folder.
func (s *Server) searchContents(ctx context.Context, c *caller, args json.RawMessage) *mcp.CallToolResult {
	var in struct {
		Path  string `json:"path"`
		Text  string `json:"text"`
		Query string `json:"query"`
		JobID string `json:"jobId"`
	}
	if err := decodeArgs(args, &in); err != nil {
		return badArgs(err)
	}
	if in.JobID == "" {
		if strings.TrimSpace(in.Text) == "" || in.Path == "" {
			return toolError("invalid_arguments", "give path and text, or the jobId of a search started earlier")
		}
		if busy := s.runningSearch(ctx, c); busy != "" {
			return toolError("busy", "a content search of this key is still running; collect it with jobId "+busy)
		}
		var j apiJob
		body := jobRequest{Kind: kindContentSearch, Params: map[string]any{"path": in.Path, "text": in.Text, "q": in.Query}}
		if err := s.call(ctx, c, http.MethodPost, "/jobs", "/jobs", nil, body, &j); err != nil {
			return failure(err)
		}
		s.contentMu.Lock()
		s.contentRun[c.key.ID] = j.ID
		s.contentMu.Unlock()
		in.JobID = j.ID
	}
	j, err := s.waitJob(ctx, c, in.JobID, contentWait)
	if errors.Is(err, errBadJob) {
		return toolError("not_found", "there is no such job")
	}
	if err != nil {
		return failure(err)
	}
	if j.Kind != kindContentSearch {
		return toolError("not_found", "this job is no content search")
	}
	out := map[string]any{"jobId": j.ID, "state": j.State}
	if j.open() {
		out["files"] = map[string]int64{"searched": j.DoneFiles, "total": j.TotalFiles}
		return ok(out)
	}
	s.contentMu.Lock()
	if s.contentRun[c.key.ID] == j.ID {
		delete(s.contentRun, c.key.ID)
	}
	s.contentMu.Unlock()
	var res struct {
		Items   []apiItem `json:"items"`
		Total   int       `json:"total"`
		Scanned int       `json:"scanned"`
	}
	if len(j.Result) > 0 {
		_ = json.Unmarshal(j.Result, &res)
	}
	out["total"], out["scanned"], out["items"] = res.Total, res.Scanned, hits(res.Items)
	if j.Error != "" {
		out["error"] = scrub(j.Error)
	}
	return ok(out)
}

// runningSearch returns the content search of the key that still runs, if
// any.
func (s *Server) runningSearch(ctx context.Context, c *caller) string {
	s.contentMu.Lock()
	id := s.contentRun[c.key.ID]
	s.contentMu.Unlock()
	if id == "" {
		return ""
	}
	if j, err := s.job(ctx, c, id); err == nil && j.open() {
		return id
	}
	s.contentMu.Lock()
	if s.contentRun[c.key.ID] == id {
		delete(s.contentRun, c.key.ID)
	}
	s.contentMu.Unlock()
	return ""
}

// waitJob reads the job until it ended or wait ran out.
func (s *Server) waitJob(ctx context.Context, c *caller, id string, wait time.Duration) (apiJob, error) {
	deadline := time.Now().Add(wait)
	for {
		j, err := s.job(ctx, c, id)
		if err != nil || !j.open() || time.Now().After(deadline) {
			return j, err
		}
		select {
		case <-ctx.Done():
			return j, nil
		case <-time.After(jobPoll):
		}
	}
}

// TrashEntry is one entry of the trash.
type TrashEntry struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	From      string `json:"from,omitempty"`
	Kind      string `json:"kind"`
	Size      int64  `json:"size"`
	Files     int64  `json:"filesAndFolders"`
	By        string `json:"deletedBy"`
	DeletedAt string `json:"deletedAt"`
	ExpiresAt string `json:"expiresAt,omitempty"`
}

func (s *Server) listTrash(ctx context.Context, c *caller, args json.RawMessage) *mcp.CallToolResult {
	var in struct {
		Limit int `json:"limit"`
	}
	if err := decodeArgs(args, &in); err != nil {
		return badArgs(err)
	}
	if in.Limit <= 0 {
		in.Limit = trashDefault
	}
	var list struct {
		Items []struct {
			ID        string `json:"id"`
			Name      string `json:"name"`
			Origin    string `json:"origin"`
			Dir       bool   `json:"dir"`
			Size      int64  `json:"size"`
			Files     int64  `json:"files"`
			By        string `json:"by"`
			DeletedAt string `json:"deletedAt"`
			ExpiresAt string `json:"expiresAt"`
		} `json:"items"`
	}
	if err := s.call(ctx, c, http.MethodGet, "/trash", "/trash", nil, nil, &list); err != nil {
		return failure(err)
	}
	out := []TrashEntry{}
	for _, it := range list.Items {
		if len(out) == min(in.Limit, trashMax) {
			break
		}
		e := TrashEntry{ID: it.ID, Name: it.Name, Kind: "file", Size: it.Size, Files: it.Files, By: it.By, DeletedAt: it.DeletedAt, ExpiresAt: it.ExpiresAt}
		if it.Dir {
			e.Kind = "folder"
		}
		// An entry of the system's trash from outside every source names a
		// folder on disk, which stays out.
		if s.apiPath(it.Origin) {
			e.From = it.Origin
		}
		out = append(out, e)
	}
	return ok(map[string]any{"total": len(list.Items), "items": out})
}

// apiPath reports whether p is a path of the API inside some source.
func (s *Server) apiPath(p string) bool {
	if p == "" {
		return false
	}
	clean, err := ccfs.Clean(p)
	if err != nil || clean != p {
		return false
	}
	loc, err := s.deps.FS.Resolve(p)
	return err == nil && !loc.Virtual()
}
