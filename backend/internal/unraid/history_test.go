package unraid

import (
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"
)

var day0 = time.Date(2026, 11, 17, 9, 30, 0, 0, time.UTC)

func diskState(disks ...Disk) *State { return &State{Available: true, Disks: disks} }

func dataDisk(name string, used int64, asleep bool, temp int) Disk {
	d := Disk{Name: name, Kind: "data", Mounted: true, Size: 4000, FsSize: 4000, Used: used, Free: 4000 - used, Spundown: asleep}
	if !asleep {
		d.Temp = &temp
	}
	return d
}

func TestHistoryKeepsOneSampleADayWithTheLastValue(t *testing.T) {
	h := NewHistory("")
	h.Observe(diskState(dataDisk("disk1", 1000, false, 30)), day0)
	h.Observe(diskState(dataDisk("disk1", 1100, false, 30)), day0.Add(3*time.Hour))
	h.Observe(diskState(dataDisk("disk1", 1200, false, 30)), day0.Add(24*time.Hour))
	got := h.Samples("disk1")
	if len(got) != 2 || got[0].Used != 1100 || got[1].Used != 1200 || got[1].Day != got[0].Day+1 {
		t.Errorf("samples = %+v", got)
	}
}

func TestHistoryKeepsAtLeastTwelveWeeksAndForgetsOlderDays(t *testing.T) {
	h := NewHistory("")
	for i := range 120 {
		h.Observe(diskState(dataDisk("disk1", int64(1000+i), false, 30)), day0.AddDate(0, 0, i))
	}
	got := h.Samples("disk1")
	if len(got) != keepDays+1 || got[len(got)-1].Used != 1119 {
		t.Fatalf("%d samples, last %+v", len(got), got[len(got)-1])
	}
	if span := got[len(got)-1].Day - got[0].Day; span < 12*7 {
		t.Errorf("only %d days kept", span)
	}
}

func TestHistorySurvivesARestartInACompactFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "drive-history.json")
	h := NewHistory(path)
	for i := range 84 {
		h.Observe(diskState(dataDisk("disk1", int64(1000+i), false, 30), dataDisk("disk2", 5, false, 31)), day0.AddDate(0, 0, i))
	}
	if err := h.Save(); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() > 4096 {
		t.Errorf("twelve weeks of two disks take %d bytes", info.Size())
	}
	again := NewHistory(path)
	if got := again.Samples("disk1"); len(got) != 84 || got[83].Used != 1083 {
		t.Errorf("after the restart: %d samples", len(got))
	}
}

func TestHistoryRemembersWhenADiskFellAsleepAndItsLastTemperature(t *testing.T) {
	h := NewHistory("")
	h.Observe(diskState(dataDisk("disk1", 1000, false, 33)), day0)
	h.Observe(diskState(dataDisk("disk1", 1000, true, 0)), day0.Add(10*time.Minute))
	h.Observe(diskState(dataDisk("disk1", 1000, true, 0)), day0.Add(2*time.Hour))
	tr := h.Track("disk1")
	if tr.Temp == nil || *tr.Temp != 33 || tr.TempAt != day0.Unix() || tr.AsleepSince != day0.Add(10*time.Minute).Unix() {
		t.Errorf("track = %+v", tr)
	}
	h.Observe(diskState(dataDisk("disk1", 1000, false, 36)), day0.Add(3*time.Hour))
	if tr := h.Track("disk1"); tr.AsleepSince != 0 || *tr.Temp != 36 {
		t.Errorf("awake again: %+v", tr)
	}
}

func TestHistoryDoesNotGuessWhenADiskThatWasAsleepFromTheStartFellAsleep(t *testing.T) {
	h := NewHistory("")
	h.Observe(diskState(dataDisk("disk1", 1000, true, 0)), day0)
	if tr := h.Track("disk1"); tr.AsleepSince != 0 || tr.Temp != nil {
		t.Errorf("track = %+v", tr)
	}
}

func samplesOf(days int, used func(i int) int64) []Sample {
	out := make([]Sample, days)
	for i := range out {
		out[i] = Sample{Day: 20400 + int64(i), Used: used(i)}
	}
	return out
}

func TestFullInFollowsTheTrendOfTheWeeksShown(t *testing.T) {
	// 10 bytes a day, so 70 a week, with 730 free.
	s := samplesOf(28, func(i int) int64 { return 3000 + int64(i)*10 })
	weeks, ok := FullIn(s, 4000, 4)
	if !ok || math.Abs(weeks-10) > 0.5 {
		t.Errorf("full in %.2f weeks, %v", weeks, ok)
	}
}

func TestFullInSaysNothingWhileTheDiskStaysFlatOrEmpties(t *testing.T) {
	for name, used := range map[string]func(int) int64{
		"flat":    func(int) int64 { return 3000 },
		"falling": func(i int) int64 { return 3000 - int64(i)*10 },
		"barely":  func(i int) int64 { return 3000 + int64(i/14) },
	} {
		if weeks, ok := FullIn(samplesOf(56, used), 4000, 8); ok {
			t.Errorf("%s: full in %.1f weeks", name, weeks)
		}
	}
}

func TestFullInWaitsForAWeekOfSamples(t *testing.T) {
	if _, ok := FullIn(samplesOf(5, func(i int) int64 { return 3000 + int64(i)*100 }), 4000, 4); ok {
		t.Error("five days are taken as a trend")
	}
}

func TestFullInLooksOnlyAtTheWeeksShown(t *testing.T) {
	// Growing for eight weeks, then flat for the last four.
	s := samplesOf(84, func(i int) int64 { return 1000 + int64(min(i, 56))*10 })
	if _, ok := FullIn(s, 4000, 4); ok {
		t.Error("the last four flat weeks show a trend")
	}
	if _, ok := FullIn(s, 4000, 12); !ok {
		t.Error("twelve weeks show no trend")
	}
}
