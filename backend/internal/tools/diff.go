package tools

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
)

const (
	maxDiffText   = 8 << 20
	maxDiffBinary = 1 << 20
	// maxDiffCells bounds the table of the line comparison once the lines
	// both files share at their start and end are taken off.
	maxDiffCells = 1 << 24
)

// DiffRow pairs a line of the left file with one of the right, by index.
// T is eq, chg, del or add, and a side without a line is nil.
type DiffRow struct {
	T string `json:"t"`
	L *int   `json:"l"`
	R *int   `json:"r"`
}

// DiffSide is one of the two files.
type DiffSide struct {
	Path  string    `json:"path"`
	Size  int64     `json:"size"`
	MTime time.Time `json:"mtime"`
	Lines []string  `json:"lines"`
}

// DiffLines compares two texts line by line, as the prototype does: the
// longest common subsequence, then a run of removals next to a run of
// additions is paired up into changed lines.
func DiffLines(a, b []string) ([]DiffRow, error) {
	pre := 0
	for pre < len(a) && pre < len(b) && a[pre] == b[pre] {
		pre++
	}
	suf := 0
	for suf < len(a)-pre && suf < len(b)-pre && a[len(a)-1-suf] == b[len(b)-1-suf] {
		suf++
	}
	ma, mb := a[pre:len(a)-suf], b[pre:len(b)-suf]
	n, m := len(ma), len(mb)
	if (n+1)*(m+1) > maxDiffCells {
		return nil, fmt.Errorf("the files differ in too many lines to compare them")
	}
	t := make([]int32, (n+1)*(m+1))
	at := func(i, j int) int32 { return t[i*(m+1)+j] }
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if ma[i] == mb[j] {
				t[i*(m+1)+j] = at(i+1, j+1) + 1
			} else {
				t[i*(m+1)+j] = max(at(i+1, j), at(i, j+1))
			}
		}
	}
	type op struct {
		kind byte
		i, j int
	}
	ops := make([]op, 0, len(a)+len(b))
	for k := 0; k < pre; k++ {
		ops = append(ops, op{'=', k, k})
	}
	i, j := 0, 0
	for i < n || j < m {
		switch {
		case i < n && j < m && ma[i] == mb[j]:
			ops = append(ops, op{'=', pre + i, pre + j})
			i++
			j++
		case j < m && (i == n || at(i, j+1) >= at(i+1, j)):
			ops = append(ops, op{'+', -1, pre + j})
			j++
		default:
			ops = append(ops, op{'-', pre + i, -1})
			i++
		}
	}
	for k := 0; k < suf; k++ {
		ops = append(ops, op{'=', len(a) - suf + k, len(b) - suf + k})
	}
	ptr := func(v int) *int { return &v }
	var rows []DiffRow
	for k := 0; k < len(ops); {
		if ops[k].kind == '=' {
			rows = append(rows, DiffRow{T: "eq", L: ptr(ops[k].i), R: ptr(ops[k].j)})
			k++
			continue
		}
		var dels, adds []int
		for k < len(ops) && ops[k].kind != '=' {
			if ops[k].kind == '-' {
				dels = append(dels, ops[k].i)
			} else {
				adds = append(adds, ops[k].j)
			}
			k++
		}
		for x := 0; x < max(len(dels), len(adds)); x++ {
			row := DiffRow{}
			if x < len(dels) {
				row.L = ptr(dels[x])
			}
			if x < len(adds) {
				row.R = ptr(adds[x])
			}
			switch {
			case row.L != nil && row.R != nil:
				row.T = "chg"
			case row.L != nil:
				row.T = "del"
			default:
				row.T = "add"
			}
			rows = append(rows, row)
		}
	}
	return rows, nil
}

// hexLines shows binary content as the hex view does, 16 bytes a line.
func hexLines(b []byte) []string {
	var out []string
	for off := 0; off < len(b); off += 16 {
		chunk := b[off:min(off+16, len(b))]
		var hx, text strings.Builder
		for i, c := range chunk {
			if i > 0 {
				hx.WriteByte(' ')
			}
			fmt.Fprintf(&hx, "%02x", c)
			if c >= 0x20 && c < 0x7f {
				text.WriteByte(c)
			} else {
				text.WriteByte('.')
			}
		}
		out = append(out, fmt.Sprintf("%08x  %-47s  %s", off, hx.String(), text.String()))
	}
	return out
}

// isBinary looks for a NUL byte near the start, as git and grep do.
func isBinary(b []byte) bool {
	return bytes.IndexByte(b[:min(len(b), 8000)], 0) >= 0
}

func splitLines(s string) []string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.TrimSuffix(s, "\n")
	if s == "" {
		return []string{}
	}
	return strings.Split(s, "\n")
}

type diffRequest struct {
	Left  string `json:"left"`
	Right string `json:"right"`
}

func (h handlers) diff(w http.ResponseWriter, r *http.Request) error {
	u := v1.UserFrom(r)
	var req diffRequest
	if err := v1.DecodeJSON(r, &req); err != nil {
		return err
	}
	var sides [2]DiffSide
	var raw [2][]byte
	binary := false
	for i, p := range []string{req.Left, req.Right} {
		loc, info, err := file(h.fs, u, p, ccfs.View|ccfs.Download)
		if err != nil {
			return err
		}
		if info.Size() > maxDiffText {
			return v1.Errorf(http.StatusRequestEntityTooLarge, "the files are too big to compare line by line")
		}
		if raw[i], err = os.ReadFile(loc.Real); err != nil {
			return err
		}
		binary = binary || isBinary(raw[i])
		sides[i] = DiffSide{Path: loc.Path, Size: info.Size(), MTime: info.ModTime()}
	}
	for i := range sides {
		if binary {
			if len(raw[i]) > maxDiffBinary {
				return v1.Errorf(http.StatusRequestEntityTooLarge, "binary files are compared up to 1 MiB")
			}
			sides[i].Lines = hexLines(raw[i])
		} else {
			sides[i].Lines = splitLines(string(raw[i]))
		}
	}
	rows, err := DiffLines(sides[0].Lines, sides[1].Lines)
	if err != nil {
		return v1.Errorf(http.StatusUnprocessableEntity, err.Error())
	}
	hunks := 0
	for i, row := range rows {
		if row.T != "eq" && (i == 0 || rows[i-1].T == "eq") {
			hunks++
		}
	}
	if rows == nil {
		rows = []DiffRow{}
	}
	return v1.WriteJSON(w, r, http.StatusOK, map[string]any{
		"left": sides[0], "right": sides[1], "binary": binary, "rows": rows, "hunks": hunks,
	})
}
