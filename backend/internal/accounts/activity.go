package accounts

import (
	"net/http"
	"path"
	"time"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/activity"
	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	activitydb "github.com/junkerderprovinz/cannoncommander/backend/internal/database/activity"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/state"
)

// ActivityEntry is one line of the activity log. What is the event type,
// Obj names what it was about in a few words, and Path and Target are
// absolute paths when the event touched files.
type ActivityEntry struct {
	ID     int64     `json:"id"`
	T      time.Time `json:"t"`
	Who    string    `json:"who"`
	What   string    `json:"what"`
	Obj    string    `json:"obj"`
	Path   string    `json:"path,omitempty"`
	Target string    `json:"target,omitempty"`
	IP     string    `json:"ip,omitempty"`
	// AuthMethod and TokenName name the API token or the key of the MCP
	// server something was done with.
	AuthMethod string                     `json:"authMethod,omitempty"`
	TokenName  string                     `json:"tokenName,omitempty"`
	Details    activitydb.FrontendDetails `json:"details"`
}

// ActivityPage is one page of the activity log, the newest entry first.
type ActivityPage struct {
	Items []ActivityEntry `json:"items"`
	Total int             `json:"total"`
	Page  int             `json:"page"`
	Limit int             `json:"limit"`
}

func (h handlers) listActivity(w http.ResponseWriter, r *http.Request) error {
	u := v1.UserFrom(r)
	filter, status, err := activity.ParseFilter(r, &activity.Actor{User: u})
	if err != nil {
		return v1.Errorf(status, err.Error())
	}
	activity.ClampListPaging(&filter)
	rows, total, err := state.ListActivity(filter)
	if err != nil {
		return err
	}
	activity.PrepareItemsForViewer(rows, u)
	items := make([]ActivityEntry, 0, len(rows))
	for _, e := range rows {
		items = append(items, h.entry(e))
	}
	return v1.WriteJSON(w, r, http.StatusOK, ActivityPage{Items: items, Total: total, Page: filter.Page, Limit: filter.Limit})
}

// absolute turns a path inside the source with this name into an API path.
func (h handlers) absolute(source, p string) string {
	if p == "" {
		return ""
	}
	for _, s := range h.fs.Sources() {
		if s.Name == source {
			return path.Join(s.Mount, "/"+p)
		}
	}
	return p
}

func (h handlers) entry(e activitydb.FrontendEntry) ActivityEntry {
	out := ActivityEntry{
		ID:         e.ID,
		T:          time.Unix(e.CreatedAt, 0).UTC(),
		Who:        e.Username,
		What:       string(e.EventType),
		Path:       h.absolute(e.Source, e.Path),
		Target:     h.absolute(e.Source, e.TargetPath),
		IP:         e.IPAddress,
		AuthMethod: e.AuthMethod,
		TokenName:  e.TokenName,
		Details:    e.Details,
	}
	d := e.Details
	switch {
	case d.TargetUsername != "":
		out.Obj = d.TargetUsername
	case d.AffectedTokenName != "":
		out.Obj = d.AffectedTokenName
	case d.PasskeyName != "":
		out.Obj = d.PasskeyName
	case out.Target != "":
		out.Obj = out.Path + " → " + out.Target
	case out.Path != "":
		out.Obj = out.Path
	default:
		out.Obj = out.IP
	}
	return out
}
