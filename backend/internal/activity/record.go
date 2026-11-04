package activity

import (
	"net/http"
	"strings"
	"time"

	activitydb "github.com/junkerderprovinz/cannoncommander/backend/internal/database/activity"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
)

// MoveCopyItem identifies a single move/copy/rename for activity logging.
type MoveCopyItem struct {
	FromSource string
	FromPath   string
	ToPath     string
}

// BulkDeleteItem identifies a deleted path for activity logging.
type BulkDeleteItem struct {
	Source string
	Path   string
}

type activityActorPolicy int

const (
	activityActorUser activityActorPolicy = iota
	activityActorAny
)

func recordEntry(entry activitydb.Entry) {
	Record(entry)
}

func finalizeActivityEntry(r *http.Request, actor *Actor, entry activitydb.Entry, policy activityActorPolicy) {
	switch policy {
	case activityActorUser:
		if actor == nil || actor.User == nil || actor.User.ID == 0 {
			return
		}
		if entry.UserID == 0 {
			entry.UserID = actor.User.ID
		}
	case activityActorAny:
		if actor == nil {
			return
		}
		if actor.User != nil && entry.UserID == 0 {
			entry.UserID = actor.User.ID
		}
	}

	if entry.CreatedAt == 0 {
		entry.CreatedAt = time.Now().Unix()
	}
	if entry.IPAddress == "" && r != nil {
		entry.IPAddress = remoteIP(r)
	}
	applyActivityAuthContext(actor, &entry)
	recordEntry(entry)
}

func applyActivityAuthContext(actor *Actor, entry *activitydb.Entry) {
	if actor == nil || actor.Token == "" {
		return
	}
	if actor.User != nil {
		if name, ok := users.TokenNameByRaw(actor.User.Tokens, actor.Token); ok {
			entry.Details.TokenName = name
			if strings.HasPrefix(name, "WEB_TOKEN") {
				entry.Details.AuthMethod = "webToken"
			} else {
				entry.Details.AuthMethod = "apiKey"
			}
			return
		}
	}
	entry.Details.AuthMethod = "webToken"
}

func RecordUser(r *http.Request, actor *Actor, entry activitydb.Entry) {
	finalizeActivityEntry(r, actor, entry, activityActorUser)
}

func RecordWebDAVUser(r *http.Request, user *users.User, entry activitydb.Entry) {
	if user == nil || user.ID == 0 {
		return
	}
	actor := &Actor{User: user}
	if entry.UserID == 0 {
		entry.UserID = user.ID
	}
	finalizeActivityEntry(r, actor, entry, activityActorUser)
}

func scopesToActivityDetails(target *users.User) []activitydb.ScopeDetail {
	if target == nil {
		return nil
	}
	frontend := target.GetFrontendScopes()
	out := make([]activitydb.ScopeDetail, 0, len(frontend))
	for _, s := range frontend {
		out = append(out, activitydb.ScopeDetail{
			Source: s.Name,
			Path:   s.Scope,
		})
	}
	return out
}

func RecordUserMutation(r *http.Request, actor *Actor, eventType activitydb.EventType, target *users.User, changes []activitydb.FieldChange) {
	if actor == nil || actor.User == nil || target == nil {
		return
	}
	details := activitydb.Details{
		TargetUsername: target.Username,
		Changes:        changes,
	}
	if eventType == activitydb.EventUserCreate {
		details.Scopes = scopesToActivityDetails(target)
	}
	RecordUser(r, actor, activitydb.Entry{
		EventType: eventType,
		Details:   details,
	})
}

func RecordTokenMutation(r *http.Request, actor *Actor, eventType activitydb.EventType, affectedTokenName string) {
	RecordUser(r, actor, activitydb.Entry{
		EventType: eventType,
		Details: activitydb.Details{
			AffectedTokenName: affectedTokenName,
		},
	})
}

func RecordAuth(r *http.Request, user *users.User, eventType activitydb.EventType, details activitydb.Details) {
	if user == nil || user.ID == 0 {
		return
	}
	entry := activitydb.Entry{
		UserID:    user.ID,
		EventType: eventType,
		Details:   details,
		CreatedAt: time.Now().Unix(),
	}
	if r != nil {
		entry.IPAddress = remoteIP(r)
	}
	recordEntry(entry)
}

func RecordLogin(r *http.Request, user *users.User) {
	if user == nil {
		return
	}
	RecordAuth(r, user, activitydb.EventLogin, activitydb.Details{
		LoginMethod: string(user.LoginMethod),
	})
}
