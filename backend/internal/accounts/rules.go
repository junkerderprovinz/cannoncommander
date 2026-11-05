package accounts

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"slices"
	"strings"
	"sync"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/state"
)

// What a rule does with the paths it matches.
const (
	ruleHide     = "hide"
	ruleReadOnly = "read"
	ruleAllow    = "allow"
)

// everyone is the who of a rule for all users. It can never be a login name.
const everyone = "*"

const maxRules = 500

// Rule is one access rule. Who is a login name, "@" and a group, or "*" for
// everybody. Path is an absolute path that covers the folder and everything
// in it, or a pattern with a star that matches names anywhere.
type Rule struct {
	Who  string `json:"who"`
	Path string `json:"path"`
	Rule string `json:"rule"`
}

type compiledRule struct {
	Rule
	glob *regexp.Regexp
	// A pattern in plain ASCII also keeps what stands before its first star,
	// after its last star and between its stars, in order.
	plain       bool
	first, last string
	between     []string
}

func compile(r Rule) compiledRule {
	c := compiledRule{Rule: r}
	if strings.Contains(r.Path, "*") {
		c.glob = regexp.MustCompile("(?i)^" + strings.ReplaceAll(regexp.QuoteMeta(r.Path), `\*`, ".*") + "$")
		if plainText(r.Path) {
			pieces := strings.Split(r.Path, "*")
			c.plain, c.first, c.last = true, pieces[0], pieces[len(pieces)-1]
			for _, p := range pieces[1 : len(pieces)-1] {
				if p != "" {
					c.between = append(c.between, p)
				}
			}
		}
	}
	return c
}

// plainText reports whether s holds only printable ASCII, where ignoring case
// means folding letters and nothing else. Beyond it the regular expression also
// folds the Kelvin sign into k, for one, and stops at line breaks.
func plainText(s string) bool {
	for i := range len(s) {
		if s[i] < 0x20 || s[i] > 0x7e {
			return false
		}
	}
	return true
}

// matches reports whether the pattern fits s, with the answer of the regular
// expression. On plain text it gets there ten times faster, which counts because
// every pattern is asked about every part of every entry of a listing.
func (c compiledRule) matches(s string) bool {
	if !c.plain || !plainText(s) {
		return c.glob.MatchString(s)
	}
	if len(s) < len(c.first)+len(c.last) ||
		!strings.EqualFold(s[:len(c.first)], c.first) ||
		!strings.EqualFold(s[len(s)-len(c.last):], c.last) {
		return false
	}
	s = s[len(c.first) : len(s)-len(c.last)]
	for _, p := range c.between {
		i := indexFold(s, p)
		if i < 0 {
			return false
		}
		s = s[i+len(p):]
	}
	return true
}

// indexFold finds p in s, ignoring case. Both are plain text and p is not empty.
func indexFold(s, p string) int {
	for i := 0; i+len(p) <= len(s); i++ {
		if strings.EqualFold(s[i:i+len(p)], p) {
			return i
		}
	}
	return -1
}

// applies reports whether the rule covers path p for the user. groups is
// only asked when a rule names a group.
func (c compiledRule) applies(name string, groups func() []string, p string) bool {
	if !c.names(name, groups) {
		return false
	}
	if c.glob != nil {
		if c.matches(p) {
			return true
		}
		for part := range strings.SplitSeq(p, "/") {
			if part != "" && c.matches(part) {
				return true
			}
		}
		return false
	}
	return c.Path == "/" || p == c.Path || strings.HasPrefix(p, c.Path+"/")
}

// names reports whether the rule is about the user at all.
func (c compiledRule) names(name string, groups func() []string) bool {
	switch {
	case c.Who == everyone:
		return true
	case strings.HasPrefix(c.Who, "@"):
		return slices.Contains(groups(), c.Who[1:])
	}
	return c.Who == name
}

// ruleBook keeps the rules in memory, because the file system asks it for
// every entry of every listing.
type ruleBook struct {
	db       *sql.DB
	groupsOf func(username string) []string

	mu    sync.RWMutex
	rules []compiledRule
}

func loadRules(db *sql.DB, groupsOf func(string) []string) (*ruleBook, error) {
	b := &ruleBook{db: db, groupsOf: groupsOf}
	rows, err := db.Query(`SELECT who, path, rule FROM path_rules ORDER BY pos`)
	if err != nil {
		return nil, fmt.Errorf("load access rules: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var r Rule
		if err := rows.Scan(&r.Who, &r.Path, &r.Rule); err != nil {
			return nil, err
		}
		b.rules = append(b.rules, compile(r))
	}
	return b, rows.Err()
}

func (b *ruleBook) list() []Rule {
	b.mu.RLock()
	defer b.mu.RUnlock()
	out := make([]Rule, len(b.rules))
	for i, c := range b.rules {
		out[i] = c.Rule
	}
	return out
}

// match returns the first rule that covers p for u, which is the one that
// decides.
func (b *ruleBook) match(u *users.User, p string) (int, Rule, bool) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	var groups []string
	asked := false
	groupsOf := func() []string {
		if !asked {
			groups, asked = b.groupsOf(u.Username), true
		}
		return groups
	}
	for i, c := range b.rules {
		if c.applies(u.Username, groupsOf, p) {
			return i, c.Rule, true
		}
	}
	return -1, Rule{}, false
}

// Effect tells the file system what the rules say about p for u.
func (b *ruleBook) Effect(u *users.User, p string) ccfs.Effect {
	_, r, ok := b.match(u, p)
	switch {
	case !ok:
		return ccfs.Open
	case r.Rule == ruleHide:
		return ccfs.Hidden
	case r.Rule == ruleReadOnly:
		return ccfs.ReadOnly
	}
	return ccfs.Open
}

// Below reports whether a rule could hide or freeze something below p for u.
// Patterns match names anywhere, so they always could.
func (b *ruleBook) Below(u *users.User, p string) bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	var groups []string
	asked := false
	groupsOf := func() []string {
		if !asked {
			groups, asked = b.groupsOf(u.Username), true
		}
		return groups
	}
	for _, c := range b.rules {
		if c.Rule.Rule == ruleAllow || !c.names(u.Username, groupsOf) {
			continue
		}
		if c.glob != nil || c.Path == p || strings.HasPrefix(c.Path, strings.TrimSuffix(p, "/")+"/") {
			return true
		}
	}
	return false
}

// replace stores list as the new rules, in this order.
func (b *ruleBook) replace(ctx context.Context, list []Rule) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.store(ctx, list)
}

// edit changes the rules in place, for renamed and deleted users and groups.
func (b *ruleBook) edit(ctx context.Context, f func([]Rule) []Rule) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	list := make([]Rule, len(b.rules))
	for i, c := range b.rules {
		list[i] = c.Rule
	}
	return b.store(ctx, f(list))
}

// renameWho points the rules of one user or group at its new name.
func (b *ruleBook) renameWho(ctx context.Context, from, to string) error {
	return b.edit(ctx, func(list []Rule) []Rule {
		for i := range list {
			if list[i].Who == from {
				list[i].Who = to
			}
		}
		return list
	})
}

// dropWho removes the rules of a user or group that is gone.
func (b *ruleBook) dropWho(ctx context.Context, who string) error {
	return b.edit(ctx, func(list []Rule) []Rule {
		return slices.DeleteFunc(list, func(r Rule) bool { return r.Who == who })
	})
}

// store writes list and takes it over. Call with b.mu held.
func (b *ruleBook) store(ctx context.Context, list []Rule) error {
	tx, err := b.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM path_rules`); err != nil {
		return err
	}
	for i, r := range list {
		if _, err := tx.ExecContext(ctx, `INSERT INTO path_rules (pos, who, path, rule) VALUES (?, ?, ?, ?)`, i, r.Who, r.Path, r.Rule); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	b.rules = b.rules[:0:0]
	for _, r := range list {
		b.rules = append(b.rules, compile(r))
	}
	return nil
}

type ruleList struct {
	Items []Rule `json:"items"`
}

func (h handlers) getRules(w http.ResponseWriter, r *http.Request) error {
	return v1.WriteJSON(w, r, http.StatusOK, ruleList{Items: h.svc.rules.list()})
}

func (h handlers) putRules(w http.ResponseWriter, r *http.Request) error {
	var in ruleList
	if err := v1.DecodeJSON(r, &in); err != nil {
		return err
	}
	if len(in.Items) > maxRules {
		return v1.Errorf(http.StatusBadRequest, fmt.Sprintf("there can be at most %d rules", maxRules))
	}
	for i := range in.Items {
		if err := h.checkRule(&in.Items[i]); err != nil {
			return v1.Errorf(http.StatusBadRequest, fmt.Sprintf("rule %d: %s", i+1, err))
		}
	}
	if err := h.svc.rules.replace(r.Context(), in.Items); err != nil {
		return err
	}
	return v1.WriteJSON(w, r, http.StatusOK, ruleList{Items: h.svc.rules.list()})
}

// checkRule validates a rule and brings its path into the stored form.
func (h handlers) checkRule(rule *Rule) error {
	rule.Who = strings.TrimSpace(rule.Who)
	switch {
	case rule.Who == everyone:
	case strings.HasPrefix(rule.Who, "@"):
		if !state.GroupExists(rule.Who[1:]) {
			return errors.New("there is no group " + rule.Who[1:])
		}
	case rule.Who == "":
		return errors.New("say who the rule is for")
	default:
		if _, err := state.GetUserByUsername(rule.Who); err != nil {
			return errors.New("there is no user " + rule.Who)
		}
	}
	switch rule.Rule {
	case ruleHide, ruleReadOnly, ruleAllow:
	default:
		return errors.New("the rule has to be hide, read or allow")
	}
	rule.Path = strings.TrimSpace(rule.Path)
	if rule.Path == "" {
		return errors.New("the path or pattern is missing")
	}
	if strings.Contains(rule.Path, "*") {
		if len(rule.Path) > 256 || strings.ContainsRune(rule.Path, 0) {
			return errors.New("the pattern is too long")
		}
		return nil
	}
	p, err := ccfs.Clean(rule.Path)
	if err != nil || strings.Contains(p, ccfs.ArchiveSep) || !h.exists(p) {
		return errors.New("this path does not exist. For a pattern use a star, such as *.tmp")
	}
	rule.Path = p
	return nil
}

// exists reports whether p is a folder or file the server has.
func (h handlers) exists(p string) bool {
	loc, err := h.fs.Resolve(p)
	if err != nil {
		return false
	}
	if loc.Virtual() {
		return true
	}
	_, err = os.Lstat(loc.Real)
	return err == nil
}

// Verdict is what one user gets at one path. Result is visible, read,
// hidden or missing. Reason names what decided: scope, rule, role, source
// or none. Rule is the index of the deciding rule, counted from 0, and Scope
// the folder of the user when that decided.
type Verdict struct {
	Result string `json:"result"`
	Reason string `json:"reason"`
	Rule   *int   `json:"rule,omitempty"`
	Scope  string `json:"scope,omitempty"`
}

type ruleProbe struct {
	User string `json:"user"`
	Path string `json:"path"`
}

func (h handlers) testRules(w http.ResponseWriter, r *http.Request) error {
	var in ruleProbe
	if err := v1.DecodeJSON(r, &in); err != nil {
		return err
	}
	target, err := state.GetUserByUsername(strings.TrimSpace(in.User))
	if err != nil {
		return v1.Errorf(http.StatusNotFound, "there is no user "+in.User)
	}
	p, err := ccfs.Clean(strings.TrimSpace(in.Path))
	if err != nil {
		return v1.Errorf(http.StatusBadRequest, "the path has to start with /")
	}
	return v1.WriteJSON(w, r, http.StatusOK, h.verdict(&target, p))
}

// verdict asks the file system itself, so the answer matches what the user
// will see, and takes the reason from the rules, the role and the source.
func (h handlers) verdict(u *users.User, p string) Verdict {
	if !h.exists(p) {
		return Verdict{Result: "missing", Reason: "none"}
	}
	outer, _, _ := strings.Cut(p, ccfs.ArchiveSep)
	i, rule, matched := h.svc.rules.match(u, outer)
	decided := func(result string) Verdict { return Verdict{Result: result, Reason: "rule", Rule: &i} }
	byScope := func(result string) Verdict {
		return Verdict{Result: result, Reason: "scope", Scope: commonFolder(h.scopePaths(u))}
	}
	if _, err := h.fs.Check(u, p, ccfs.View); err != nil {
		if matched && rule.Rule == ruleHide {
			return decided("hidden")
		}
		return byScope("hidden")
	}
	if _, err := h.fs.Check(u, p, ccfs.Modify); err != nil {
		switch {
		case matched && rule.Rule == ruleReadOnly:
			return decided("read")
		case errors.Is(err, ccfs.ErrReadOnly):
			return Verdict{Result: "read", Reason: "source"}
		case roleOf(u) == roleReadOnly:
			return Verdict{Result: "read", Reason: "role"}
		}
		return byScope("read")
	}
	if matched {
		return decided("visible")
	}
	return Verdict{Result: "visible", Reason: "none"}
}
