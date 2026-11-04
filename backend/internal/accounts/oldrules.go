package accounts

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/gtsteffaniak/go-logger/logger"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/migrate"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/settings"
)

// The older interface kept its access rules in access_rules, one row per
// source and folder with lists of users and groups to deny or allow. They
// are carried into path_rules once, so the app has a single set of rules
// that its settings page shows and that deleting a user cleans up.
func init() {
	migrate.Register(migrate.Main, migrate.Migration{Name: "accounts/007_old_access_rules", Up: carryOverOldRules})
}

type oldNames map[string]struct{}

type oldRule struct {
	DenyAll bool `json:"denyAll"`
	Deny    struct{ Users, Groups oldNames }
	Allow   struct{ Users, Groups oldNames }
}

type oldRow struct {
	source, path string
	rule         oldRule
}

// oldSource is what the carry over needs to know about a configured source.
// Key is the path the old rules used for it, Root the folder on disk.
type oldSource struct {
	name, key, root, mount string
	denyByDefault          bool
}

// carriedRules is the outcome of the carry over: the rules in their order,
// and a line for everything that could not be taken over as it was.
type carriedRules struct {
	rules []Rule
	notes []string
}

func carryOverOldRules(ctx context.Context, tx *sql.Tx) error {
	var found int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'access_rules'`).Scan(&found); err != nil {
		return err
	}
	var rows []oldRow
	if found > 0 {
		var err error
		if rows, err = readOldRules(ctx, tx); err != nil {
			return err
		}
	}
	known := map[string]struct{}{}
	if len(rows) > 0 {
		var err error
		if known, err = knownNames(ctx, tx); err != nil {
			return err
		}
	}
	out := carryOver(rows, configuredSources(), func(who string) bool { _, ok := known[who]; return ok }, onDisk)
	for _, n := range out.notes {
		logger.Warningf("old access rules: %s", n)
	}
	if len(out.rules) > 0 {
		if err := prependRules(ctx, tx, out.rules); err != nil {
			return err
		}
	}
	for _, r := range out.rules {
		logger.Infof("old access rules: carried over %s %s %s", r.Who, r.Rule, r.Path)
	}
	if found > 0 {
		if _, err := tx.ExecContext(ctx, `DROP TABLE access_rules`); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM settings WHERE key = 'access_rules_lowercase_migrated'`); err != nil {
			return err
		}
	}
	return nil
}

func readOldRules(ctx context.Context, tx *sql.Tx) ([]oldRow, error) {
	rs, err := tx.QueryContext(ctx, `SELECT source, path, rule_data FROM access_rules ORDER BY source, path`)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	var out []oldRow
	for rs.Next() {
		var r oldRow
		var data string
		if err := rs.Scan(&r.source, &r.path, &data); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(data), &r.rule); err != nil {
			return nil, fmt.Errorf("old access rule %s %s: %w", r.source, r.path, err)
		}
		out = append(out, r)
	}
	return out, rs.Err()
}

// knownNames returns the users by login name and the groups as "@" and
// their name, which is how rules name them.
func knownNames(ctx context.Context, tx *sql.Tx) (map[string]struct{}, error) {
	known := map[string]struct{}{}
	for _, q := range []string{`SELECT username FROM users`, `SELECT '@' || name FROM groups`} {
		rs, err := tx.QueryContext(ctx, q)
		if err != nil {
			return nil, err
		}
		for rs.Next() {
			var name string
			if err := rs.Scan(&name); err != nil {
				rs.Close()
				return nil, err
			}
			known[name] = struct{}{}
		}
		if err := rs.Close(); err != nil {
			return nil, err
		}
	}
	return known, nil
}

func configuredSources() []oldSource {
	var out []oldSource
	for _, s := range settings.Config.Server.Sources {
		if s == nil {
			continue
		}
		out = append(out, oldSource{
			name:          s.Name,
			key:           s.Path,
			root:          s.Path,
			mount:         ccfs.MountFor(s.Path),
			denyByDefault: s.Config.DenyByDefault,
		})
	}
	return out
}

// prependRules puts list in front of the rules already stored. Before, a
// path had to pass both sets, so the carried rules keep their say over
// what was set up in the app since.
func prependRules(ctx context.Context, tx *sql.Tx, list []Rule) error {
	rs, err := tx.QueryContext(ctx, `SELECT who, path, rule FROM path_rules ORDER BY pos`)
	if err != nil {
		return err
	}
	var existing []Rule
	for rs.Next() {
		var r Rule
		if err := rs.Scan(&r.Who, &r.Path, &r.Rule); err != nil {
			rs.Close()
			return err
		}
		existing = append(existing, r)
	}
	if err := rs.Close(); err != nil {
		return err
	}
	if len(existing) > 0 {
		logger.Warningf("old access rules: they come before the %d rules set up in the app, please look over the order", len(existing))
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM path_rules`); err != nil {
		return err
	}
	for i, r := range append(slices.Clone(list), existing...) {
		if _, err := tx.ExecContext(ctx, `INSERT INTO path_rules (pos, who, path, rule) VALUES (?, ?, ?, ?)`, i, r.Who, r.Path, r.Rule); err != nil {
			return err
		}
	}
	if n := len(list) + len(existing); n > maxRules {
		logger.Warningf("old access rules: there are now %d rules, the settings page saves at most %d", n, maxRules)
	}
	return nil
}

// carryOver turns the old rules into ordered rules with the same outcome.
//
// The old check walked from a path up to the source root and let the
// deepest folder that named the user decide: a denied user, then a denied
// group, then an allowed user, then an allowed group. Only when no folder
// named the user did a deny for everyone apply, and after that the source
// default. First match wins in the new rules, so they come out deepest
// folder first, the denies for everyone after all of those, and a hide for
// the whole source last. The old rules matched paths without regard to
// case, the new ones match exactly, so a rule gets one entry per spelling
// that exists on disk.
func carryOver(rows []oldRow, sources []oldSource, known func(who string) bool, onDisk func(root, index string) []string) carriedRules {
	var out carriedRules
	note := func(format string, args ...any) { out.notes = append(out.notes, fmt.Sprintf(format, args...)) }

	bySource := map[int]map[string]*oldRule{}
	for _, r := range rows {
		i := slices.IndexFunc(sources, func(s oldSource) bool { return s.key == r.source })
		if i < 0 {
			i = slices.IndexFunc(sources, func(s oldSource) bool { return s.name == r.source })
		}
		if i < 0 {
			note("the rule on %s in %s is left out, no source has that path any more", r.path, r.source)
			continue
		}
		if bySource[i] == nil {
			bySource[i] = map[string]*oldRule{}
		}
		key := oldKey(r.path)
		merged := bySource[i][key]
		if merged == nil {
			merged = &oldRule{}
			bySource[i][key] = merged
		}
		merge(merged, r.rule)
	}

	order := make([]int, len(sources))
	for i := range order {
		order[i] = i
	}
	slices.SortFunc(order, func(a, b int) int { return strings.Compare(sources[a].mount, sources[b].mount) })

	var list []Rule
	for _, i := range order {
		src := sources[i]
		keys := make([]string, 0, len(bySource[i]))
		for k := range bySource[i] {
			keys = append(keys, k)
		}
		slices.SortFunc(keys, func(a, b string) int {
			if d := depth(b) - depth(a); d != 0 {
				return d
			}
			return strings.Compare(a, b)
		})
		places := map[string][]string{}
		for _, k := range keys {
			found := onDisk(src.root, k)
			if len(found) == 0 {
				note("%s in %s does not exist, its rule keeps the path in lower case", k, src.name)
				found = []string{k}
			}
			for _, p := range found {
				places[k] = append(places[k], path.Join(src.mount, p))
			}
		}
		var everyoneHidden []Rule
		for _, k := range keys {
			r := bySource[i][k]
			for _, p := range places[k] {
				for _, e := range []struct {
					names  oldNames
					prefix string
					rule   string
				}{
					{r.Deny.Users, "", ruleHide},
					{r.Deny.Groups, "@", ruleHide},
					{r.Allow.Users, "", ruleAllow},
					{r.Allow.Groups, "@", ruleAllow},
				} {
					for _, name := range sortedNames(e.names) {
						who := e.prefix + name
						if !known(who) {
							note("the rule for %s on %s is left out, there is no such %s", who, p, kind(e.prefix))
							continue
						}
						list = append(list, Rule{Who: who, Path: p, Rule: e.rule})
					}
				}
				if r.DenyAll {
					everyoneHidden = append(everyoneHidden, Rule{Who: everyone, Path: p, Rule: ruleHide})
				}
			}
		}
		list = append(list, everyoneHidden...)
		if src.denyByDefault {
			list = append(list, Rule{Who: everyone, Path: src.mount, Rule: ruleHide})
		}
	}

	for i, r := range list {
		if r.Rule == ruleAllow && !slices.ContainsFunc(list[i+1:], func(h Rule) bool { return h.Rule == ruleHide && overlap(h.Path, r.Path) && mayMeet(h.Who, r.Who) }) {
			note("the allow for %s on %s is left out, no rule after it hides that place", r.Who, r.Path)
			continue
		}
		out.rules = append(out.rules, r)
	}
	return out
}

// oldKey brings a stored path into one form: lower case, starting with a
// slash, without one at the end. Older versions stored files and folders
// with and without the trailing slash and did not always lower the case.
func oldKey(p string) string {
	return strings.ToLower(path.Clean("/" + strings.TrimSpace(p)))
}

func depth(p string) int {
	if p == "/" {
		return 0
	}
	return strings.Count(p, "/")
}

func merge(dst *oldRule, src oldRule) {
	dst.DenyAll = dst.DenyAll || src.DenyAll
	add := func(to *oldNames, from oldNames) {
		if *to == nil {
			*to = oldNames{}
		}
		for n := range from {
			(*to)[n] = struct{}{}
		}
	}
	add(&dst.Deny.Users, src.Deny.Users)
	add(&dst.Deny.Groups, src.Deny.Groups)
	add(&dst.Allow.Users, src.Allow.Users)
	add(&dst.Allow.Groups, src.Allow.Groups)
}

func sortedNames(n oldNames) []string {
	out := make([]string, 0, len(n))
	for name := range n {
		if name != "" {
			out = append(out, name)
		}
	}
	slices.Sort(out)
	return out
}

func kind(prefix string) string {
	if prefix == "@" {
		return "group"
	}
	return "user"
}

// overlap reports whether rules on a and b share a place: one holds the
// other. An allow on a folder also lets its user past a hide for everyone
// deeper down, as the old rules did.
func overlap(a, b string) bool {
	return holds(a, b) || holds(b, a)
}

func holds(dir, p string) bool {
	return dir == "/" || dir == p || strings.HasPrefix(p, dir+"/")
}

// mayMeet reports whether two rules can be about the same user. A group
// can hold anyone.
func mayMeet(a, b string) bool {
	return a == everyone || b == everyone || a == b || strings.HasPrefix(a, "@") || strings.HasPrefix(b, "@")
}

// onDisk returns every spelling of index under root that exists, matching
// each name without regard to case as the old rules did.
func onDisk(root, index string) []string {
	if _, err := os.Stat(root); err != nil {
		return nil
	}
	found := []string{"/"}
	for _, part := range strings.Split(strings.Trim(index, "/"), "/") {
		if part == "" {
			continue
		}
		var next []string
		for _, dir := range found {
			entries, err := os.ReadDir(filepath.Join(root, filepath.FromSlash(dir)))
			if err != nil {
				continue
			}
			for _, e := range entries {
				if strings.ToLower(e.Name()) == part {
					next = append(next, path.Join(dir, e.Name()))
				}
			}
		}
		found = next
	}
	return found
}
