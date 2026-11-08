package ffmpeg

import (
	"context"
	"testing"
	"time"
)

func TestWaitReturnsAtOnceWhenNobodyIsLooking(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if Checking() {
		t.Fatal("checking before anything started")
	}
	_ = Wait(ctx)
	if ctx.Err() != nil {
		t.Fatal("waited although nothing was looked for")
	}
}

func TestWaitHoldsUntilTheLookEnds(t *testing.T) {
	done := Probing()
	if !Checking() {
		t.Fatal("not checking while the look runs")
	}
	waited := make(chan struct{})
	go func() {
		Wait(context.Background())
		close(waited)
	}()
	select {
	case <-waited:
		t.Fatal("Wait returned before the look ended")
	case <-time.After(50 * time.Millisecond):
	}
	done()
	select {
	case <-waited:
	case <-time.After(time.Second):
		t.Fatal("Wait did not return once the look ended")
	}
	if Checking() {
		t.Fatal("still checking after the look ended")
	}
}

func TestWaitGivesUpWithItsContext(t *testing.T) {
	done := Probing()
	defer done()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if Wait(ctx) != nil {
		t.Fatal("a service turned up while the look still runs")
	}
}
