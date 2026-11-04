package undo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"testing"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/testutil"
)

func TestMain(m *testing.M) { os.Exit(testutil.Main(m)) }

var reversed []string

func init() {
	Register("test", Reverser{Inline: func(_ context.Context, _ v1.Deps, _ *users.User, data json.RawMessage) error {
		var name string
		if err := json.Unmarshal(data, &name); err != nil {
			return err
		}
		if name == "stuck" {
			return errors.New("stuck")
		}
		reversed = append(reversed, name)
		return nil
	}})
}

func user(name string) *users.User {
	u := &users.User{}
	u.Username = name
	return u
}

func TestStepsAreKeptPerUserAndTakenBackNewestFirst(t *testing.T) {
	reversed = nil
	l := &Log{db: testutil.MainDB(t)}
	for i := range Keep + 5 {
		if _, err := l.Record("anna", Step{Kind: "test", Name: fmt.Sprint(i)}, fmt.Sprint(i)); err != nil {
			t.Fatal(err)
		}
	}
	bert, err := l.Record("bert", Step{Kind: "test", Name: "b"}, "b")
	if err != nil {
		t.Fatal(err)
	}
	steps, err := l.List("anna")
	if err != nil {
		t.Fatal(err)
	}
	if len(steps) != Keep || steps[0].Name != fmt.Sprint(Keep+4) {
		t.Fatalf("%d steps, newest %s", len(steps), steps[0].Name)
	}

	ctx := context.Background()
	if _, _, err := l.Take(ctx, user("anna"), ""); err != nil {
		t.Fatal(err)
	}
	if _, _, err := l.Take(ctx, user("anna"), steps[5].ID); err != nil {
		t.Fatal(err)
	}
	if _, _, err := l.Take(ctx, user("anna"), steps[5].ID); !errors.Is(err, ErrNothing) {
		t.Fatalf("a step was reversed back twice: %v", err)
	}
	if _, _, err := l.Take(ctx, user("anna"), bert.ID); !errors.Is(err, ErrNothing) {
		t.Fatal("a step of another user was reversed back")
	}
	if fmt.Sprint(reversed) != fmt.Sprintf("[%d %s]", Keep+4, steps[5].Name) {
		t.Fatalf("reversed back: %v", reversed)
	}
}

func TestAStepThatFailsStaysInTheList(t *testing.T) {
	l := &Log{db: testutil.MainDB(t)}
	if _, err := l.Record("anna", Step{Kind: "test", Name: "x"}, "stuck"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := l.Take(context.Background(), user("anna"), ""); err == nil {
		t.Fatal("no error")
	}
	if steps, _ := l.List("anna"); len(steps) != 1 {
		t.Fatal("the failed step is gone")
	}
}
