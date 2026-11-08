package nativeshell

import (
	"slices"
	"strings"
	"testing"
)

var hostileNames = []string{
	"a;rm -rf ~",
	"it's",
	`say "hi" $HOME`,
	"back\\slash `id`",
	"$(reboot)",
	"tab\there\nline",
	"#not a comment",
	"--help",
	"",
}

func TestAQuotedNameComesBackAsOneArgument(t *testing.T) {
	for _, name := range hostileNames {
		for _, quote := range []func(string) string{shellQuote, doubleQuote} {
			got, err := splitArgv("tool " + quote(name) + " end")
			if err != nil || !slices.Equal(got, []string{"tool", name, "end"}) {
				t.Errorf("%q as %s: got %q %v", name, quote(name), got, err)
			}
		}
	}
}

func TestACommandLineSplitsLikeGLib(t *testing.T) {
	cases := []struct{ line, want string }{
		{"a b\tc\nd", "a|b|c|d"},
		{"  a   b  ", "a|b"},
		{`a 'b c' "d e"`, "a|b c|d e"},
		{`a'b'"c"d`, "abcd"},
		{`a '' ""`, "a||"},
		{`a 'x\y' "x\y"`, `a|x\y|x\y`},
		{`a "\$x \"q\" \\ \a"`, `a|$x "q" \ \a`},
		{`a \"b\" \ c \\`, `a|"b"| c|\`},
		{"a \\\nb", "a|b"},
		{"a \"x\\\ny\"", "a|xy"},
		{"a #comment\nb # c", "a|b"},
		{"a#b '#'", "a#b|#"},
		{"a $HOME `id` $(x) ~ *", "a|$HOME|`id`|$(x)|~|*"},
		{"a;b|c&d", "a;b|c&d"},
	}
	for _, c := range cases {
		got, err := splitArgv(c.line)
		if err != nil || strings.Join(got, "|") != c.want {
			t.Errorf("%q: got %q %v, want %s", c.line, got, err, c.want)
		}
	}
	for _, bad := range []string{"", "   ", "# only", "a 'open", `a "open`, `a "x\"`, `a \`} {
		if got, err := splitArgv(bad); err == nil {
			t.Errorf("%q: got %q", bad, got)
		}
	}
}

func TestCodesAreFilledInAndPercentSignsKept(t *testing.T) {
	codes := map[byte]string{'f': "'/a b'", 'F': "'/a' '/b'", 'D': ""}
	cases := map[string]string{
		"x %f":       "x '/a b'",
		"x %F %D!":   "x '/a' '/b' !",
		"100%% %z %": "100% %z %",
		"%f%f":       "'/a b''/a b'",
	}
	for command, want := range cases {
		if got := fill(command, codes); got != want {
			t.Errorf("%q: got %q", command, got)
		}
	}
}
