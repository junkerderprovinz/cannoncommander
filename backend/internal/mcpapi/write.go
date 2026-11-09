package mcpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/activity"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	activitydb "github.com/junkerderprovinz/cannoncommander/backend/internal/database/activity"
)

// The guard against an assistant that changes too much at once: entries per
// call, entries per hour and key, and files and folders below the entries of
// one call. The settings card reads the numbers off the API.
const (
	itemsPerCall = 25
	itemsPerHour = 200
	filesPerCall = 10000
	bytesPerCall = 50 << 30

	// walkFolders bounds how many folders counting what lies below an entry
	// may open, when the index does not know the number yet.
	walkFolders = 200
)

// jobStarted is the answer of a write tool that started a job.
type jobStarted struct {
	JobID string `json:"jobId"`
	State string `json:"state"`
	Name  string `json:"name"`
	Note  string `json:"note"`
}

const startedNote = "The job runs in CannonCommander; follow it with get_job. The user sees it in the jobs list and can undo it."

func started(j apiJob) *mcp.CallToolResult {
	return ok(jobStarted{JobID: j.ID, State: j.State, Name: j.Name, Note: startedNote})
}

// check refuses what no key may touch, whatever its owner may do: a path
// inside an archive, a share of another computer nobody opened, and with
// whole set, the top of a source or a folder that only holds sources. That
// last refusal would tell a source the account cannot see from no source at
// all, so it comes only after the account was shown to see the path.
func (s *Server) check(ctx context.Context, c *caller, paths []string, whole bool) *mcp.CallToolResult {
	for _, p := range paths {
		if strings.Contains(p, ccfs.ArchiveSep) {
			return toolError("in_archive", "the tools do not change archives; "+p+" lies inside one")
		}
		if r := s.offNet(p); r != nil {
			return r
		}
		if !whole {
			continue
		}
		if err := s.call(ctx, c, http.MethodGet, "/fs/stat", "/fs/stat", url.Values{"path": {p}}, nil, nil); err != nil {
			return s.fail(c, err)
		}
		loc, err := s.deps.FS.Resolve(p)
		if err == nil && (loc.Virtual() || loc.Path == loc.Source.Mount) {
			return toolError("source_root", p+" is the top of a source; pick what lies inside it")
		}
	}
	return nil
}

// spend takes n entries from the key's hourly budget. The release it returns
// gives them back for a change that did not happen.
func (s *Server) spend(c *caller, n int) (release func(), refusal *mcp.CallToolResult) {
	now := s.now()
	ok, retry := s.writes.take(c.key.ID, n, now)
	if !ok {
		return nil, toolError("hourly_limit", fmt.Sprintf("this key changed %d entries in the last hour, the most it may; try again in %d minutes",
			itemsPerHour-s.writes.left(c.key.ID, now), int(retry.Minutes())+1))
	}
	return func() { s.writes.give(c.key.ID, n, now) }, nil
}

// preflight is what /ops/preflight says about a copy or a move before it
// starts.
type preflight struct {
	Bytes      int64  `json:"bytes"`
	Files      int64  `json:"files"`
	SameDevice bool   `json:"sameDevice"`
	Space      *space `json:"space"`
}

type space struct {
	Free   int64 `json:"free"`
	Need   int64 `json:"need"`
	Enough bool  `json:"enough"`
}

// refusal stops a transfer that would change too much at once or not fit. A
// move on one disk only renames, so its bytes cost nothing.
func (p preflight) refusal() *mcp.CallToolResult {
	switch {
	case p.Files > filesPerCall:
		return tooBig(p.Files)
	case !p.SameDevice && p.Bytes > bytesPerCall:
		return toolError("too_big", fmt.Sprintf("these entries hold %d bytes, more than the %d one call may copy; pick smaller folders", p.Bytes, int64(bytesPerCall)))
	case p.Space != nil && !p.Space.Enough:
		return toolError("no_space", fmt.Sprintf("the target has %d bytes free and needs %d", p.Space.Free, p.Space.Need))
	}
	return nil
}

// tooBig refuses a call whose entries hold more than filesPerCall files and
// folders, the kind of change nobody would want made by mistake.
func tooBig(n int64) *mcp.CallToolResult {
	return toolError("too_big", fmt.Sprintf("these entries hold %d files and folders, more than the %d one call may change; pick smaller folders", n, filesPerCall))
}

// record puts a change into the activity log under the key's name.
func (s *Server) record(c *caller, event activitydb.EventType, from, to string, paths []string) {
	d := activitydb.Details{TokenName: c.key.Label, AuthMethod: "mcp", Path: from, TargetPath: to}
	if len(paths) > 1 {
		d.Paths, d.FileCount = paths, len(paths)
		d.CapPaths()
	}
	activity.Record(activitydb.Entry{EventType: event, UserID: c.user.ID, Path: from, TargetPath: to, IPAddress: c.addr, Details: d})
}

func (s *Server) createFolder(ctx context.Context, c *caller, args json.RawMessage) *mcp.CallToolResult {
	var in struct {
		Path string `json:"path"`
	}
	if err := decodeArgs(args, &in); err != nil {
		return badArgs(err)
	}
	if !c.key.CanWrite {
		return s.fail(c, errWriteOff)
	}
	if r := s.check(ctx, c, []string{in.Path}, false); r != nil {
		return r
	}
	release, refusal := s.spend(c, 1)
	if refusal != nil {
		return refusal
	}
	if err := s.call(ctx, c, http.MethodPost, "/fs/mkdir", "/fs/mkdir", nil, map[string]string{"path": in.Path}, nil); err != nil {
		release()
		return s.fail(c, err)
	}
	s.record(c, activitydb.EventFolderCreate, in.Path, "", nil)
	return ok(map[string]any{"path": in.Path, "created": true})
}

func (s *Server) renameItem(ctx context.Context, c *caller, args json.RawMessage) *mcp.CallToolResult {
	var in struct {
		Path    string `json:"path"`
		NewName string `json:"newName"`
	}
	if err := decodeArgs(args, &in); err != nil {
		return badArgs(err)
	}
	if !c.key.CanWrite {
		return s.fail(c, errWriteOff)
	}
	if in.NewName == "" || strings.ContainsAny(in.NewName, `/\`) || in.NewName == "." || in.NewName == ".." {
		return toolError("invalid_arguments", "newName is a plain name, without a folder")
	}
	if r := s.check(ctx, c, []string{in.Path}, true); r != nil {
		return r
	}
	release, refusal := s.spend(c, 1)
	if refusal != nil {
		return refusal
	}
	body := map[string]string{"path": in.Path, "name": in.NewName}
	if err := s.call(ctx, c, http.MethodPost, "/fs/rename", "/fs/rename", nil, body, nil); err != nil {
		release()
		return s.fail(c, err)
	}
	to := path.Join(path.Dir(in.Path), in.NewName)
	s.record(c, activitydb.EventRename, in.Path, to, nil)
	return ok(map[string]any{"path": to, "renamed": true})
}

func (s *Server) copyItems(ctx context.Context, c *caller, args json.RawMessage) *mcp.CallToolResult {
	return s.transfer(ctx, c, args, "copy", activitydb.EventCopy)
}

func (s *Server) moveItems(ctx context.Context, c *caller, args json.RawMessage) *mcp.CallToolResult {
	return s.transfer(ctx, c, args, "move", activitydb.EventMove)
}

// transfer copies or moves as the job the panels start with F5 and F6. A
// taken name never gets replaced: replacing would put the old entry in the
// trash, which is the user's call.
func (s *Server) transfer(ctx context.Context, c *caller, args json.RawMessage, kind string, event activitydb.EventType) *mcp.CallToolResult {
	var in struct {
		Sources     []string `json:"sources"`
		Destination string   `json:"destination"`
		OnConflict  string   `json:"onConflict"`
	}
	if err := decodeArgs(args, &in); err != nil {
		return badArgs(err)
	}
	if !c.key.CanWrite {
		return s.fail(c, errWriteOff)
	}
	rule := "skip"
	switch in.OnConflict {
	case "", "skip":
	case "keepBoth":
		rule = "both"
	default:
		return toolError("invalid_arguments", "onConflict is skip or keepBoth")
	}
	if len(in.Sources) == 0 || in.Destination == "" {
		return toolError("invalid_arguments", "give the sources and the destination")
	}
	if r := s.check(ctx, c, in.Sources, kind == "move"); r != nil {
		return r
	}
	if r := s.check(ctx, c, []string{in.Destination}, false); r != nil {
		return r
	}
	if len(in.Sources) > itemsPerCall {
		return toolError("too_many", fmt.Sprintf("at most %d entries per call; split the work into smaller calls", itemsPerCall))
	}
	params := map[string]any{"sources": in.Sources, "dest": in.Destination}
	var pre preflight
	preBody := map[string]any{"kind": kind, "sources": in.Sources, "dest": in.Destination}
	if err := s.call(ctx, c, http.MethodPost, "/ops/preflight", "/ops/preflight", nil, preBody, &pre); err != nil {
		return s.fail(c, err)
	}
	if r := pre.refusal(); r != nil {
		return r
	}
	release, refusal := s.spend(c, len(in.Sources))
	if refusal != nil {
		return refusal
	}
	params["rule"] = rule
	var j apiJob
	if err := s.call(ctx, c, http.MethodPost, "/jobs", "/jobs", nil, jobRequest{Kind: kind, Params: params}, &j); err != nil {
		release()
		return s.fail(c, err)
	}
	s.record(c, event, in.Sources[0], in.Destination, in.Sources)
	return started(j)
}

func (s *Server) moveToTrash(ctx context.Context, c *caller, args json.RawMessage) *mcp.CallToolResult {
	var in struct {
		Paths []string `json:"paths"`
	}
	if err := decodeArgs(args, &in); err != nil {
		return badArgs(err)
	}
	if !c.key.CanWrite {
		return s.fail(c, errWriteOff)
	}
	if len(in.Paths) == 0 {
		return toolError("invalid_arguments", "give the paths to move to the trash")
	}
	if len(in.Paths) > itemsPerCall {
		return toolError("too_many", fmt.Sprintf("at most %d entries per call; split the work into smaller calls", itemsPerCall))
	}
	if r := s.check(ctx, c, in.Paths, true); r != nil {
		return r
	}
	var total int64
	for _, p := range in.Paths {
		n, err := s.countBelow(ctx, c, p, filesPerCall-total)
		if err != nil {
			return s.fail(c, err)
		}
		if total += n; total > filesPerCall {
			return tooBig(total)
		}
	}
	release, refusal := s.spend(c, len(in.Paths))
	if refusal != nil {
		return refusal
	}
	var j apiJob
	body := jobRequest{Kind: "trash", Params: map[string]any{"paths": in.Paths}}
	if err := s.call(ctx, c, http.MethodPost, "/jobs", "/jobs", nil, body, &j); err != nil {
		release()
		return s.fail(c, err)
	}
	s.record(c, activitydb.EventTrash, in.Paths[0], "", in.Paths)
	return started(j)
}

// countBelow counts an entry and the files and folders below it. The index
// knows the number for most folders; for the others it lists them, and stops
// once the count passes limit.
func (s *Server) countBelow(ctx context.Context, c *caller, p string, limit int64) (int64, error) {
	var st apiItem
	if err := s.call(ctx, c, http.MethodGet, "/fs/stat", "/fs/stat", url.Values{"path": {p}}, nil, &st); err != nil {
		return 0, err
	}
	if !st.Dir || st.Link != "" {
		return 1, nil
	}
	if st.Files != nil && st.Dirs != nil {
		return 1 + *st.Files + *st.Dirs, nil
	}
	n, queue := int64(1), []string{st.Path}
	for opened := 0; len(queue) > 0 && n <= limit; opened++ {
		if opened == walkFolders {
			return limit + 1, nil
		}
		var list struct {
			Items []apiItem `json:"items"`
		}
		if err := s.call(ctx, c, http.MethodGet, "/fs/list", "/fs/list", url.Values{"path": {queue[0]}}, nil, &list); err != nil {
			return 0, err
		}
		for _, it := range list.Items {
			n++
			if it.Dir && it.Link == "" {
				queue = append(queue, path.Join(queue[0], it.Name))
			}
		}
		queue = queue[1:]
	}
	return n, nil
}

func (s *Server) restoreFromTrash(ctx context.Context, c *caller, args json.RawMessage) *mcp.CallToolResult {
	var in struct {
		IDs []string `json:"ids"`
	}
	if err := decodeArgs(args, &in); err != nil {
		return badArgs(err)
	}
	if !c.key.CanWrite {
		return s.fail(c, errWriteOff)
	}
	if len(in.IDs) == 0 {
		return toolError("invalid_arguments", "give the ids list_trash names")
	}
	if len(in.IDs) > itemsPerCall {
		return toolError("too_many", fmt.Sprintf("at most %d entries per call; split the work into smaller calls", itemsPerCall))
	}
	var list struct {
		Items []struct {
			ID     string `json:"id"`
			Origin string `json:"origin"`
			Files  int64  `json:"files"`
		} `json:"items"`
	}
	if err := s.call(ctx, c, http.MethodGet, "/trash", "/trash", nil, nil, &list); err != nil {
		return s.fail(c, err)
	}
	var total int64
	var origins []string
	for _, id := range in.IDs {
		found := false
		for _, it := range list.Items {
			if it.ID == id {
				found = true
				total += max(it.Files, 1)
				if s.apiPath(it.Origin) {
					origins = append(origins, it.Origin)
				}
			}
		}
		if !found {
			return toolError("not_found", "there is no trash entry "+id+" this account can see")
		}
	}
	if r := s.check(ctx, c, origins, false); r != nil {
		return r
	}
	if total > filesPerCall {
		return tooBig(total)
	}
	release, refusal := s.spend(c, len(in.IDs))
	if refusal != nil {
		return refusal
	}
	var j apiJob
	if err := s.call(ctx, c, http.MethodPost, "/trash/restore", "/trash/restore", nil, map[string]any{"ids": in.IDs}, &j); err != nil {
		release()
		return s.fail(c, err)
	}
	first := ""
	if len(origins) > 0 {
		first = origins[0]
	}
	s.record(c, activitydb.EventRestore, first, "", origins)
	return started(j)
}
