package labels_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/migrate"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/hub"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/labels"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/testutil"
)

type fixture struct {
	t    *testing.T
	h    http.Handler
	deps v1.Deps
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	base := t.TempDir()
	share, boot := filepath.Join(base, "user"), filepath.Join(base, "boot")
	for _, p := range []string{
		"dokumente/Mietvertrag.pdf", "dokumente/Steuer 2025.xlsx", "dokumente/alt/Rechnung.pdf",
		"fotos/IMG_0203.jpg", "archiv/dokumente/alt/.keep", "dokumente-alt/.keep",
	} {
		full := filepath.Join(share, p)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(boot, "config"), 0o755); err != nil {
		t.Fatal(err)
	}
	fs, err := ccfs.New(nil,
		ccfs.Source{Name: "Shares", Mount: "/mnt/user", Root: share},
		ccfs.Source{Name: "USB-Stick", Mount: "/boot", Root: boot, ReadOnly: true},
	)
	if err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", filepath.Join(base, "main.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if _, err := migrate.Apply(context.Background(), db, migrate.Main); err != nil {
		t.Fatal(err)
	}

	all := users.SourceFilePermissions{View: true, Download: true, Create: true, Modify: true, Delete: true}
	look := users.SourceFilePermissions{View: true, Download: true}
	people := map[string]*users.User{
		"jdp":  {ID: 1, BackendScopes: []users.BackendScope{{Path: share, Scope: "/", Permissions: all}, {Path: boot, Scope: "/", Permissions: all}}},
		"anna": {ID: 2, BackendScopes: []users.BackendScope{{Path: share, Scope: "/fotos", Permissions: all}}},
		"bert": {ID: 3, BackendScopes: []users.BackendScope{{Path: share, Scope: "/", Permissions: look}}},
	}
	for name, u := range people {
		u.Username = name
	}
	people["jdp"].Permissions.Admin = true
	auth := func(_ http.ResponseWriter, r *http.Request) (*users.User, error) {
		if u, ok := people[r.Header.Get("X-User")]; ok {
			return u, nil
		}
		return nil, v1.Errorf(http.StatusUnauthorized, "login required")
	}
	deps := v1.Deps{FS: fs, Hub: hub.New(hub.Options{}), DB: db}
	r := v1.NewRouter(deps, auth)
	labels.Routes(r)
	h, err := r.Handler()
	if err != nil {
		t.Fatal(err)
	}
	return &fixture{t: t, h: h, deps: deps}
}

func (f *fixture) do(user, method, target string, body any) *httptest.ResponseRecorder {
	f.t.Helper()
	raw := ""
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			f.t.Fatal(err)
		}
		raw = string(b)
	}
	req := httptest.NewRequest(method, target, strings.NewReader(raw))
	req.Header.Set("X-User", user)
	rec := httptest.NewRecorder()
	f.h.ServeHTTP(rec, req)
	return rec
}

func (f *fixture) must(want int, user, method, target string, body any) *httptest.ResponseRecorder {
	f.t.Helper()
	rec := f.do(user, method, target, body)
	if rec.Code != want {
		f.t.Fatalf("%s %s as %s: %d, want %d: %s", method, target, user, rec.Code, want, rec.Body)
	}
	return rec
}

func decode[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("decode %s: %v", rec.Body, err)
	}
	return v
}

type items struct {
	Path  string                  `json:"path"`
	Items map[string]labels.Label `json:"items"`
}

func (f *fixture) folder(user, p string, deep bool) map[string]labels.Label {
	f.t.Helper()
	target := "/labels/items?path=" + p
	if deep {
		target += "&deep=true"
	}
	return decode[items](f.t, f.must(200, user, "GET", target, nil)).Items
}

func (f *fixture) set(body map[string]any) {
	f.t.Helper()
	f.must(200, "jdp", "PUT", "/labels/items", body)
}

func (f *fixture) lists() labels.Overview {
	f.t.Helper()
	return decode[labels.Overview](f.t, f.must(200, "jdp", "GET", "/labels", nil))
}

func colourCount(o labels.Overview, id string) int {
	for _, c := range o.Colours {
		if c.ID == id {
			return c.Count
		}
	}
	return -1
}

func tagNames(o labels.Overview) []string {
	var out []string
	for _, t := range o.Tags {
		out = append(out, t.Name)
	}
	return out
}

func TestTheSixColoursComeWithoutNames(t *testing.T) {
	f := newFixture(t)
	o := f.lists()
	var ids []string
	for _, c := range o.Colours {
		ids = append(ids, c.ID)
		if c.Name != "" || c.Count != 0 {
			t.Errorf("colour %+v", c)
		}
	}
	if strings.Join(ids, ",") != "red,orange,yellow,green,blue,purple" || len(o.Tags) != 0 {
		t.Errorf("lists %+v", o)
	}
}

func TestLabelsAreSetPerPathAndReadPerFolder(t *testing.T) {
	f := newFixture(t)
	f.set(map[string]any{"paths": []string{"/mnt/user/dokumente/Mietvertrag.pdf", "/mnt/user/dokumente/Steuer 2025.xlsx"}, "colour": "red", "add": []string{"#wichtig"}})
	f.set(map[string]any{"paths": []string{"/mnt/user/dokumente/Steuer 2025.xlsx"}, "add": []string{"Steuer", "prüfen"}})
	f.set(map[string]any{"paths": []string{"/mnt/user/dokumente/alt/Rechnung.pdf"}, "tags": []string{"Steuer"}})

	got := f.folder("jdp", "/mnt/user/dokumente", false)
	want := map[string]labels.Label{
		"Mietvertrag.pdf":  {Colour: "red", Tags: []string{"wichtig"}},
		"Steuer 2025.xlsx": {Colour: "red", Tags: []string{"Steuer", "prüfen", "wichtig"}},
	}
	if !equal(got, want) {
		t.Errorf("folder %+v", got)
	}
	if deep := f.folder("jdp", "/mnt/user/dokumente", true); len(deep) != 3 || deep["alt/Rechnung.pdf"].Tags[0] != "Steuer" {
		t.Errorf("deep %+v", deep)
	}

	o := f.lists()
	if colourCount(o, "red") != 2 || strings.Join(tagNames(o), ",") != "prüfen,Steuer,wichtig" {
		t.Errorf("lists %+v", o)
	}

	answer := decode[struct{ Items map[string]labels.Label }](t, f.must(200, "jdp", "PUT", "/labels/items",
		map[string]any{"paths": []string{"/mnt/user/dokumente/Steuer 2025.xlsx"}, "colour": "", "tags": []string{}}))
	if l, ok := answer.Items["/mnt/user/dokumente/Steuer 2025.xlsx"]; !ok || l.Colour != "" || len(l.Tags) != 0 {
		t.Errorf("answer %+v", answer)
	}
	if _, ok := f.folder("jdp", "/mnt/user/dokumente", false)["Steuer 2025.xlsx"]; ok {
		t.Error("a label without colour and tags is still there")
	}
}

func equal(a, b map[string]labels.Label) bool {
	if len(a) != len(b) {
		return false
	}
	for k, l := range a {
		m, ok := b[k]
		if !ok || l.Colour != m.Colour || !slices.Equal(l.Tags, m.Tags) {
			return false
		}
	}
	return true
}

func TestLabelsNeedTheRightToChangeTheFile(t *testing.T) {
	f := newFixture(t)
	doc := []string{"/mnt/user/dokumente/Mietvertrag.pdf"}
	f.must(403, "bert", "PUT", "/labels/items", map[string]any{"paths": doc, "colour": "red"})
	f.must(403, "anna", "PUT", "/labels/items", map[string]any{"paths": doc, "colour": "red"})
	f.must(403, "jdp", "PUT", "/labels/items", map[string]any{"paths": []string{"/boot/config"}, "colour": "red"})
	f.must(403, "jdp", "PUT", "/labels/items", map[string]any{"paths": []string{"/mnt"}, "colour": "red"})
	f.must(404, "jdp", "PUT", "/labels/items", map[string]any{"paths": []string{"/mnt/user/nowhere.txt"}, "colour": "red"})
	f.must(400, "jdp", "PUT", "/labels/items", map[string]any{"paths": doc, "colour": "pink"})
	f.must(400, "jdp", "PUT", "/labels/items", map[string]any{"paths": doc, "add": []string{" # "}})
	f.must(400, "jdp", "PUT", "/labels/items", map[string]any{"paths": []string{}})
	f.must(200, "anna", "PUT", "/labels/items", map[string]any{"paths": []string{"/mnt/user/fotos/IMG_0203.jpg"}, "colour": "purple", "add": []string{"für Anna"}})
}

func TestEveryoneSeesOnlyTheLabelsOfWhatTheyMaySee(t *testing.T) {
	f := newFixture(t)
	f.set(map[string]any{"paths": []string{"/mnt/user/dokumente/Mietvertrag.pdf", "/mnt/user/fotos/IMG_0203.jpg"}, "colour": "green"})

	if got := f.folder("bert", "/mnt/user/dokumente", false); len(got) != 1 {
		t.Errorf("bert reads %+v", got)
	}
	f.must(403, "anna", "GET", "/labels/items?path=/mnt/user/dokumente", nil)
	got := f.folder("anna", "/mnt/user", true)
	if _, ok := got["fotos/IMG_0203.jpg"]; !ok || len(got) != 1 {
		t.Errorf("anna reads %+v", got)
	}
	if got := f.folder("anna", "/mnt/user", false); len(got) != 0 {
		t.Errorf("anna reads the share root %+v", got)
	}
}

func TestOnlyAdminsEditTheSharedLists(t *testing.T) {
	f := newFixture(t)
	f.must(403, "bert", "PUT", "/labels/colours", map[string]any{"colours": []any{}})
	f.must(403, "bert", "PUT", "/labels/tags", map[string]any{"tags": []any{}})
}

func TestColoursCanBeRenamedRecolouredRemovedAndAdded(t *testing.T) {
	f := newFixture(t)
	f.set(map[string]any{"paths": []string{"/mnt/user/dokumente/Mietvertrag.pdf"}, "colour": "orange"})
	f.set(map[string]any{"paths": []string{"/mnt/user/dokumente/Steuer 2025.xlsx"}, "colour": "orange", "add": []string{"Steuer"}})

	o := decode[labels.Overview](t, f.must(200, "jdp", "PUT", "/labels/colours", map[string]any{"colours": []map[string]string{
		{"id": "red", "name": " Dringend ", "hex": "#E53935"},
		{"name": "Türkis", "hex": "#2ec4b6"},
		{"id": "blue", "name": "", "hex": "#3b9cf5"},
	}}))
	if len(o.Colours) != 3 || o.Colours[0] != (labels.Colour{ID: "red", Name: "Dringend", Hex: "#e53935"}) ||
		!strings.HasPrefix(o.Colours[1].ID, "c") || o.Colours[1].Name != "Türkis" || o.Colours[2].ID != "blue" {
		t.Fatalf("colours %+v", o.Colours)
	}

	got := f.folder("jdp", "/mnt/user/dokumente", false)
	if _, ok := got["Mietvertrag.pdf"]; ok {
		t.Error("a file that only had the removed colour kept its label")
	}
	if l := got["Steuer 2025.xlsx"]; l.Colour != "" || len(l.Tags) != 1 {
		t.Errorf("the removed colour stayed on %+v", l)
	}

	f.must(400, "jdp", "PUT", "/labels/colours", map[string]any{"colours": []map[string]string{{"id": "orange", "name": "x", "hex": "#ffffff"}}})
	f.must(400, "jdp", "PUT", "/labels/colours", map[string]any{"colours": []map[string]string{{"id": "red", "hex": "#fff"}}})
	f.must(400, "jdp", "PUT", "/labels/colours", map[string]any{"colours": []map[string]string{{"id": "red", "hex": "#ffffff"}, {"id": "red", "hex": "#000000"}}})
}

func TestTagsCanBeRenamedMergedRemovedAndDeclared(t *testing.T) {
	f := newFixture(t)
	f.set(map[string]any{"paths": []string{"/mnt/user/dokumente/Mietvertrag.pdf"}, "tags": []string{"wichtig", "Vertrag"}})
	f.set(map[string]any{"paths": []string{"/mnt/user/dokumente/Steuer 2025.xlsx"}, "tags": []string{"Steuer", "prüfen"}})

	o := decode[labels.Overview](t, f.must(200, "jdp", "PUT", "/labels/tags", map[string]any{"tags": []map[string]string{
		{"from": "wichtig", "name": "Papiere"},
		{"from": "Vertrag", "name": "Papiere"},
		{"from": "Steuer", "name": "Steuer"},
		{"name": "#Zug"},
		{"name": "Äpfel"},
	}}))
	if got := strings.Join(tagNames(o), ","); got != "Äpfel,Papiere,Steuer,Zug" {
		t.Errorf("tags %s", got)
	}
	want := map[string]labels.Label{
		"Mietvertrag.pdf":  {Tags: []string{"Papiere"}},
		"Steuer 2025.xlsx": {Tags: []string{"Steuer"}},
	}
	if got := f.folder("jdp", "/mnt/user/dokumente", false); !equal(got, want) {
		t.Errorf("items %+v", got)
	}

	f.must(200, "jdp", "PUT", "/labels/tags", map[string]any{"tags": []map[string]string{}})
	if got := f.folder("jdp", "/mnt/user/dokumente", false); len(got) != 0 {
		t.Errorf("tags survived their removal: %+v", got)
	}
}

func receive(t *testing.T, c *hub.Client) hub.Event {
	t.Helper()
	select {
	case e := <-c.Events:
		return e
	case <-time.After(time.Second):
		t.Fatal("no event")
	}
	return hub.Event{}
}

func TestLabelsFollowAMovedFolder(t *testing.T) {
	f := newFixture(t)
	f.set(map[string]any{"paths": []string{"/mnt/user/dokumente/Mietvertrag.pdf"}, "colour": "red"})
	f.set(map[string]any{"paths": []string{"/mnt/user/dokumente/alt/Rechnung.pdf"}, "tags": []string{"Steuer"}})
	f.set(map[string]any{"paths": []string{"/mnt/user/dokumente"}, "colour": "blue"})
	f.set(map[string]any{"paths": []string{"/mnt/user/dokumente-alt"}, "colour": "green"})
	f.set(map[string]any{"paths": []string{"/mnt/user/archiv/dokumente"}, "colour": "yellow"})
	f.set(map[string]any{"paths": []string{"/mnt/user/archiv/dokumente/alt"}, "tags": []string{"weg"}})

	f.deps.Hub.Subscribe("tab", "jdp", hub.FolderTopic("/mnt/user"), hub.FolderTopic("/mnt/user/archiv"))
	tab, _, _ := f.deps.Hub.Connect("tab", "jdp", 0)
	defer f.deps.Hub.Disconnect(tab)

	if err := labels.Moved(context.Background(), f.deps, labels.Move{From: "/mnt/user/dokumente", To: "/mnt/user/archiv/dokumente/"}); err != nil {
		t.Fatal(err)
	}
	if got := f.folder("jdp", "/mnt/user/archiv", false); got["dokumente"].Colour != "blue" {
		t.Errorf("the folder lost its own label: %+v", got)
	}
	got := f.folder("jdp", "/mnt/user/archiv/dokumente", true)
	want := map[string]labels.Label{"Mietvertrag.pdf": {Colour: "red"}, "alt/Rechnung.pdf": {Tags: []string{"Steuer"}}}
	if !equal(got, want) {
		t.Errorf("moved %+v", got)
	}
	if left := f.folder("jdp", "/mnt/user", false); len(left) != 1 || left["dokumente-alt"].Colour != "green" {
		t.Errorf("the old place or its neighbour is wrong: %+v", left)
	}

	var folders []string
	for range 2 {
		e := receive(t, tab)
		var c labels.Change
		if err := json.Unmarshal(e.Data, &c); err != nil || e.Type != labels.EventChanged || c.Kind != "items" {
			t.Fatalf("event %s %s", e.Type, e.Data)
		}
		folders = append(folders, c.Path)
	}
	slices.Sort(folders)
	if strings.Join(folders, " ") != "/mnt/user /mnt/user/archiv" {
		t.Errorf("announced %v", folders)
	}
}

func TestCopiesTakeTheLabelsAlong(t *testing.T) {
	f := newFixture(t)
	f.set(map[string]any{"paths": []string{"/mnt/user/dokumente/alt/Rechnung.pdf"}, "colour": "red", "tags": []string{"Steuer"}})
	err := labels.Copied(context.Background(), f.deps, labels.Move{From: "/mnt/user/dokumente/alt", To: "/mnt/user/archiv/alt"})
	if err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{"/mnt/user/dokumente/alt", "/mnt/user/archiv/alt"} {
		if l := f.folder("jdp", dir, false)["Rechnung.pdf"]; l.Colour != "red" || len(l.Tags) != 1 {
			t.Errorf("%s: %+v", dir, l)
		}
	}
}

func TestMovesInOneCallRunInOrder(t *testing.T) {
	f := newFixture(t)
	f.set(map[string]any{"paths": []string{"/mnt/user/dokumente/Mietvertrag.pdf"}, "colour": "red"})
	f.set(map[string]any{"paths": []string{"/mnt/user/dokumente/Steuer 2025.xlsx"}, "colour": "blue"})
	// Swapping two names goes through a temporary one.
	err := labels.Moved(context.Background(), f.deps,
		labels.Move{From: "/mnt/user/dokumente/Mietvertrag.pdf", To: "/mnt/user/dokumente/.swap"},
		labels.Move{From: "/mnt/user/dokumente/Steuer 2025.xlsx", To: "/mnt/user/dokumente/Mietvertrag.pdf"},
		labels.Move{From: "/mnt/user/dokumente/.swap", To: "/mnt/user/dokumente/Steuer 2025.xlsx"},
	)
	if err != nil {
		t.Fatal(err)
	}
	got := f.folder("jdp", "/mnt/user/dokumente", false)
	if got["Mietvertrag.pdf"].Colour != "blue" || got["Steuer 2025.xlsx"].Colour != "red" || len(got) != 2 {
		t.Errorf("swapped %+v", got)
	}
}

func TestRemovedPathsLoseTheirLabels(t *testing.T) {
	f := newFixture(t)
	f.set(map[string]any{"paths": []string{"/mnt/user/dokumente/alt/Rechnung.pdf", "/mnt/user/dokumente/Mietvertrag.pdf"}, "colour": "red"})
	if err := labels.Removed(context.Background(), f.deps, "/mnt/user/dokumente/alt"); err != nil {
		t.Fatal(err)
	}
	if got := f.folder("jdp", "/mnt/user/dokumente", true); len(got) != 1 || got["Mietvertrag.pdf"].Colour != "red" {
		t.Errorf("left %+v", got)
	}
	if err := labels.Removed(context.Background(), f.deps, "relative/path"); err == nil {
		t.Error("a relative path was accepted")
	}
}

func TestLabelsFollowAMoveWhileAnotherWriterIsAtWork(t *testing.T) {
	db := testutil.MainDB(t)
	ctx := context.Background()
	if _, err := db.ExecContext(ctx, `INSERT INTO label_items (path, dir, colour, updated_at) VALUES ('/mnt/user/a.txt', '/mnt/user', 'red', 1)`); err != nil {
		t.Fatal(err)
	}
	other, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := other.ExecContext(ctx, `INSERT INTO label_items (path, dir, colour, updated_at) VALUES ('/mnt/user/anders.txt', '/mnt/user', 'blue', 1)`); err != nil {
		t.Fatal(err)
	}
	finished := time.AfterFunc(100*time.Millisecond, func() { other.Commit() })
	defer finished.Stop()

	deps := v1.Deps{DB: db, Hub: hub.New(hub.Options{})}
	if err := labels.Moved(ctx, deps, labels.Move{From: "/mnt/user/a.txt", To: "/mnt/user/b.txt"}); err != nil {
		t.Fatal(err)
	}
	var colour string
	if err := db.QueryRowContext(ctx, `SELECT colour FROM label_items WHERE path = '/mnt/user/b.txt'`).Scan(&colour); err != nil || colour != "red" {
		t.Errorf("colour %q, %v", colour, err)
	}
}
