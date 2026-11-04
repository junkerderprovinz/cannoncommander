package activity

import (
	"fmt"
	"net/http"
	"strings"

	activitydb "github.com/junkerderprovinz/cannoncommander/backend/internal/database/activity"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/utils"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/indexing"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/settings"
)

// resolveActivityPathFilters validates source/path filters and scopes paths to the actor's access.
func ResolvePathFilters(actor *Actor, filter *activitydb.QueryFilter) (int, error) {
	if filter.Source == "" && filter.PathPrefix == "" && filter.PathGlob == "" {
		return 0, nil
	}
	if filter.PathPrefix != "" && filter.PathGlob != "" {
		return http.StatusBadRequest, fmt.Errorf("use path or pathGlob, not both")
	}
	if filter.Source == "" && (filter.PathPrefix != "" || filter.PathGlob != "") {
		return http.StatusBadRequest, fmt.Errorf("source is required when filtering by path")
	}

	sourceInfo, ok := settings.Config.Server.NameToSource[filter.Source]
	if !ok {
		return http.StatusBadRequest, fmt.Errorf("invalid source: %s", filter.Source)
	}
	if _, err := actor.User.GetScopeForSourceName(filter.Source); err != nil {
		return http.StatusForbidden, err
	}
	if filter.PathPrefix == "" && filter.PathGlob == "" {
		return 0, nil
	}
	idx := indexing.GetIndex(sourceInfo.Name)
	if idx == nil {
		return http.StatusBadRequest, fmt.Errorf("index not found for source: %s", filter.Source)
	}

	userScope, err := actor.User.GetScopeForSourceName(filter.Source)
	if err != nil {
		return http.StatusForbidden, err
	}

	if filter.PathPrefix != "" {
		indexPath, status, err := resolveActivityIndexPath(actor, idx, userScope, filter.PathPrefix)
		if err != nil {
			return status, err
		}
		filter.PathPrefix = indexPath
	}
	if filter.PathGlob != "" {
		scopedGlob, status, err := resolveActivityPathGlob(actor, idx, userScope, filter.PathGlob)
		if err != nil {
			return status, err
		}
		filter.PathGlob = scopedGlob
	}
	return 0, nil
}

func resolveActivityIndexPath(actor *Actor, idx *indexing.Index, userScope, clientPath string) (string, int, error) {
	clientPath = strings.TrimSpace(clientPath)
	if clientPath == "" {
		return "", http.StatusBadRequest, fmt.Errorf("path is empty")
	}
	cleanPath, err := utils.SanitizePath(clientPath)
	if err != nil {
		return "", http.StatusBadRequest, err
	}
	clientPath = cleanPath
	if !strings.HasPrefix(clientPath, "/") {
		clientPath = "/" + clientPath
	}

	userScope = strings.TrimRight(userScope, "/")
	fullIndexPath := clientPath
	if userScope != "" && userScope != "/" {
		if !strings.HasPrefix(clientPath, userScope+"/") && clientPath != userScope {
			fullIndexPath = utils.JoinPathAsUnix(userScope, strings.TrimPrefix(clientPath, "/"))
		}
	}

	isDir := strings.HasSuffix(fullIndexPath, "/")
	fullPath := idx.MakeIndexPath(fullIndexPath, isDir)
	if !accessPermitted(idx.Path, fullPath, actor.User.Username) {
		return "", http.StatusForbidden, fmt.Errorf("user is not allowed to access this location")
	}
	return strings.TrimSuffix(fullIndexPath, "/"), 0, nil
}

func resolveActivityPathGlob(actor *Actor, idx *indexing.Index, userScope, glob string) (string, int, error) {
	glob = strings.TrimSpace(glob)
	if glob == "" {
		return "", http.StatusBadRequest, fmt.Errorf("pathGlob is empty")
	}
	cleanGlob, err := utils.SanitizePath(glob)
	if err != nil {
		return "", http.StatusBadRequest, err
	}
	glob = cleanGlob

	userScope = strings.TrimRight(userScope, "/")
	var scoped string
	if strings.HasPrefix(glob, "/") {
		scoped = glob
	} else if userScope != "" && userScope != "/" {
		scoped = userScope + "/" + glob
	} else {
		scoped = "/" + glob
	}

	if userScope != "" && userScope != "/" && !strings.HasPrefix(scoped, userScope) {
		return "", http.StatusForbidden, fmt.Errorf("glob pattern outside user scope")
	}

	rootPath := userScope
	if rootPath == "" {
		rootPath = "/"
	}
	fullPath := idx.MakeIndexPath(rootPath, true)
	if !accessPermitted(idx.Path, fullPath, actor.User.Username) {
		return "", http.StatusForbidden, fmt.Errorf("user is not allowed to access this location")
	}
	return scoped, 0, nil
}

// enforceActivityPathGlobFilter restricts path glob queries to admins.
func EnforcePathGlobFilter(r *http.Request, actor *Actor) (int, error) {
	if actor.User.Permissions.Admin {
		return 0, nil
	}
	if strings.TrimSpace(r.URL.Query().Get("pathGlob")) != "" {
		return http.StatusForbidden, fmt.Errorf("forbidden: pathGlob requires admin")
	}
	return 0, nil
}

// scopeToOwnShares limits a non-admin query about share links to the links the user owns.
func scopeToOwnShares(actor *Actor, filter *activitydb.QueryFilter) {
	if !actor.User.Permissions.Admin && (filter.Scope == "shares" || filter.ShareHash != "") {
		filter.UserFilter = false
		filter.ShareOwnerUserID = actor.User.ID
		filter.ShareOwnerFilter = true
	}
}
