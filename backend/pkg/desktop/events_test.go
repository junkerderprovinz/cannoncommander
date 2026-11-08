package desktop

import (
	"reflect"
	"testing"
)

type sent struct {
	typ  string
	data any
}

func listening(t *testing.T) *[]sent {
	t.Helper()
	var got []sent
	Listen(func(typ string, data any) { got = append(got, sent{typ, data}) })
	t.Cleanup(func() { Listen(nil) })
	return &got
}

func TestAnEventReachesTheServerUnderTheDesktopPrefix(t *testing.T) {
	got := listening(t)
	Emit("open", map[string]string{"path": `D:\Daten`})
	want := []sent{{"desktop.open", map[string]string{"path": `D:\Daten`}}}
	if !reflect.DeepEqual(*got, want) {
		t.Fatalf("got %+v", *got)
	}
}

func TestEventsFromBeforeTheServerListensArriveInOrder(t *testing.T) {
	Emit("open", 1)
	Emit("drop", 2)
	got := listening(t)
	Emit("open", 3)
	want := []sent{{"desktop.open", 1}, {"desktop.drop", 2}, {"desktop.open", 3}}
	if !reflect.DeepEqual(*got, want) {
		t.Fatalf("got %+v", *got)
	}
}

func TestAServerThatNeverListensKeepsOnlyTheLatestEvents(t *testing.T) {
	t.Cleanup(func() { Listen(nil) })
	for i := range waitingEvents + 10 {
		Emit("open", i)
	}
	got := listening(t)
	if len(*got) != waitingEvents || (*got)[0].data != 10 {
		t.Fatalf("got %d events, the first %+v", len(*got), (*got)[0])
	}
}
