package search

import (
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

const (
	kb  = 1024.0
	mb  = kb * 1024
	gb  = mb * 1024
	tb  = gb * 1024
	day = 24 * time.Hour
)

// Query is a parsed search line: words for the name and tests on fields
// such as breite:>=3840 or codec:hevc.
type Query struct {
	names []namePart
	tests []test
	// Unknown are field names nobody knows. A query with any finds nothing,
	// and the client says which words it does understand.
	Unknown []string
	// Meta are the metadata columns the tests read.
	Meta []string
}

type namePart struct {
	re  *regexp.Regexp
	sub string
	tag string
}

type test func(r *row) bool

var token = regexp.MustCompile(`([\p{L}][\p{L}\d.-]*):(>=|<=|!=|>|<|=)?("[^"]*"?|\S*)|"[^"]*"?|\S+`)

// Parse splits a search line into field tests and the words for the name.
// now is the time ages such as 7d count back from.
func Parse(line string, now time.Time) Query {
	var q Query
	var rest []string
	for _, m := range token.FindAllStringSubmatch(line, -1) {
		if m[1] == "" {
			rest = append(rest, m[0])
			continue
		}
		f, ok := fields[fold(m[1])]
		if !ok {
			q.Unknown = append(q.Unknown, m[1])
			continue
		}
		if m[3] == "" || m[3] == `"` {
			continue
		}
		q.tests = append(q.tests, f.test(m[2], unquote(m[3]), now))
		if f.meta != "" && !contains(q.Meta, f.meta) {
			q.Meta = append(q.Meta, f.meta)
		}
	}
	q.names = nameParts(rest)
	return q
}

// Fields reports whether the query tests anything besides the name.
func (q Query) Fields() bool { return len(q.tests) > 0 }

// Tags returns the #words of the name part, without the #.
func (q Query) Tags() []string {
	var out []string
	for _, p := range q.names {
		if p.tag != "" {
			out = append(out, p.tag)
		}
	}
	return out
}

// matchName follows the list's filter: any word may match. A word with *
// or ? is a pattern for the whole name, any other word matches anywhere
// in it, and a word with # matches a tag or a colour.
func (q Query) matchName(r *row) bool {
	if len(q.names) == 0 {
		return true
	}
	low := strings.ToLower(r.Name)
	for _, p := range q.names {
		switch {
		case p.re != nil:
			if p.re.MatchString(r.Name) {
				return true
			}
		case p.tag != "":
			if r.tagged != nil && r.tagged(r.Path, p.tag) {
				return true
			}
		case strings.Contains(low, p.sub):
			return true
		}
	}
	return false
}

func (q Query) matchFields(r *row) bool {
	for _, t := range q.tests {
		if !t(r) {
			return false
		}
	}
	return true
}

func nameParts(words []string) []namePart {
	var out []namePart
	for _, w := range words {
		// A quoted word keeps its spaces, so a name with spaces can be
		// searched for as a whole.
		var parts []string
		if strings.HasPrefix(w, `"`) {
			parts = []string{strings.Trim(w, `"`)}
		} else {
			parts = strings.FieldsFunc(w, func(r rune) bool { return r == ';' || unicode.IsSpace(r) })
		}
		for _, p := range parts {
			if p == "" {
				continue
			}
			low := strings.ToLower(p)
			switch {
			case strings.ContainsAny(p, "*?"):
				out = append(out, namePart{re: glob(p)})
			case strings.HasPrefix(p, "#") && len(p) > 1:
				out = append(out, namePart{tag: low[1:]})
			default:
				out = append(out, namePart{sub: low})
			}
		}
	}
	return out
}

func glob(p string) *regexp.Regexp {
	var b strings.Builder
	b.WriteString("(?is)^")
	for _, r := range p {
		switch r {
		case '*':
			b.WriteString(".*")
		case '?':
			b.WriteString(".")
		default:
			b.WriteString(regexp.QuoteMeta(string(r)))
		}
	}
	b.WriteString("$")
	return regexp.MustCompile(b.String())
}

func unquote(v string) string {
	if len(v) >= 2 && strings.HasPrefix(v, `"`) && strings.HasSuffix(v, `"`) {
		return v[1 : len(v)-1]
	}
	return v
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// fold makes field names comparable the way people type them: no case,
// no accents, ß as ss and nothing but letters and digits.
func fold(s string) string {
	s = strings.ReplaceAll(strings.ToLower(s), "ß", "ss")
	var b strings.Builder
	for _, r := range norm.NFD.String(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

var leading = regexp.MustCompile(`^\s*[+-]?(\d+\.?\d*|\.\d+)([eE][+-]?\d+)?`)

// number reads the number a text starts with, after the first comma has
// become a point, so "2,5 GB" gives 2.5. It is NaN when there is none.
func number(s string) float64 {
	m := leading.FindString(strings.Replace(s, ",", ".", 1))
	if m == "" {
		return math.NaN()
	}
	v, err := strconv.ParseFloat(strings.TrimSpace(m), 64)
	if err != nil {
		return math.NaN()
	}
	return v
}

var sizeRe = regexp.MustCompile(`^([\d.,]+)\s*([kmgt]?b?)$`)

var sizeUnits = map[string]float64{"b": 1, "k": kb, "kb": kb, "m": mb, "mb": mb, "g": gb, "gb": gb, "t": tb, "tb": tb}

// Bytes reads a size such as 300 KB or 2,5 GB. A number alone counts in
// MB. It is NaN for anything else.
func Bytes(s string) float64 {
	m := sizeRe.FindStringSubmatch(strings.ToLower(strings.TrimSpace(s)))
	if m == nil {
		return math.NaN()
	}
	v := number(m[1])
	unit, ok := sizeUnits[m[2]]
	if !ok {
		unit = mb
	}
	return v * unit
}

var secsRe = regexp.MustCompile(`^([\d.]+)\s*(h|std|min|m|s)?$`)

// secs reads a duration: 90 is minutes, 2h hours and 45s seconds.
func secs(s string) float64 {
	m := secsRe.FindStringSubmatch(strings.Replace(strings.ToLower(s), ",", ".", 1))
	if m == nil {
		return math.NaN()
	}
	v, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return math.NaN()
	}
	switch m[2] {
	case "h", "std":
		return v * 3600
	case "s":
		return v
	}
	return v * 60
}

var (
	ageRe   = regexp.MustCompile(`^(\d+)\s*(h|d|t|w|mo|m|y|j)$`)
	isoRe   = regexp.MustCompile(`^(\d{4})(?:-(\d{1,2})(?:-(\d{1,2}))?)?$`)
	ageUnit = map[string]time.Duration{"h": time.Hour, "d": day, "t": day, "w": 7 * day, "m": 30 * day, "mo": 30 * day, "y": 365 * day, "j": 365 * day}
)

// moment is a time a search compares with: an age such as 30d, or a day,
// month or year starting at at, written as in 2026-10-30, 2026-10 or 2026.
type moment struct {
	age  time.Duration
	at   time.Time
	span time.Duration
	ok   bool
}

func when(v string) moment {
	if m := ageRe.FindStringSubmatch(strings.ToLower(v)); m != nil {
		n, _ := strconv.Atoi(m[1])
		return moment{age: time.Duration(n) * ageUnit[m[2]], ok: true}
	}
	m := isoRe.FindStringSubmatch(v)
	if m == nil {
		return moment{}
	}
	y, mo, d, span := atoi(m[1]), 1, 1, day
	switch {
	case m[2] == "":
		span = 365 * day
	case m[3] == "":
		mo, span = atoi(m[2]), 30*day
	default:
		mo, d = atoi(m[2]), atoi(m[3])
	}
	return moment{at: time.Date(y, time.Month(mo), d, 0, 0, 0, 0, time.Local), span: span, ok: true}
}

func atoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

func compare(a float64, op string, b float64) bool {
	switch op {
	case ">":
		return a > b
	case "<":
		return a < b
	case ">=":
		return a >= b
	case "<=":
		return a <= b
	case "=":
		return a == b
	case "!=":
		return a != b
	}
	return false
}
