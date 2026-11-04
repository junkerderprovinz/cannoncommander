package system

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/version"
	"golang.org/x/mod/semver"
)

// releasesAPI is where the newest release is looked up.
const releasesAPI = "https://api.github.com/repos/junkerderprovinz/cannoncommander/releases/latest"

// UpdateCheck is the answer to a look for a new release. The image is
// updated on Unraid's Docker tab, so the server only reports.
type UpdateCheck struct {
	Version         string `json:"version"`
	Latest          string `json:"latest"`
	UpdateAvailable bool   `json:"updateAvailable"`
	ReleaseURL      string `json:"releaseUrl,omitempty"`
}

func (s *service) checkUpdate(ctx context.Context) (UpdateCheck, error) {
	current := strings.TrimPrefix(version.Version, "v")
	out := UpdateCheck{Version: current, Latest: current}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.releases, nil)
	if err != nil {
		return out, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "CannonCommander/"+current)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return out, fmt.Errorf("could not reach GitHub: %w", err)
	}
	defer resp.Body.Close()
	// A repository without a release yet answers 404.
	if resp.StatusCode == http.StatusNotFound {
		return out, nil
	}
	if resp.StatusCode != http.StatusOK {
		return out, fmt.Errorf("GitHub answered %s", resp.Status)
	}
	var rel struct {
		Tag string `json:"tag_name"`
		URL string `json:"html_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return out, err
	}
	out.Latest, out.ReleaseURL = strings.TrimPrefix(rel.Tag, "v"), rel.URL
	cur, latest := "v"+current, "v"+out.Latest
	out.UpdateAvailable = semver.IsValid(cur) && semver.IsValid(latest) && semver.Compare(latest, cur) > 0
	return out, nil
}

// updateCache is the update check with its answer kept for ten minutes, so a
// button pressed again and again does not run into GitHub's rate limit.
type updateCache struct {
	mu  sync.Mutex
	at  time.Time
	got UpdateCheck
}

func (s *service) cachedUpdate(ctx context.Context) (UpdateCheck, error) {
	s.updates.mu.Lock()
	defer s.updates.mu.Unlock()
	if !s.updates.at.IsZero() && time.Since(s.updates.at) < 10*time.Minute {
		return s.updates.got, nil
	}
	got, err := s.checkUpdate(ctx)
	if err != nil {
		return got, err
	}
	s.updates.at, s.updates.got = time.Now(), got
	return got, nil
}
