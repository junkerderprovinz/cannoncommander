package places

import (
	"strings"
	"testing"
)

func TestNearestNamesTheTownInGerman(t *testing.T) {
	tab, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		lat, lon float64
		want     string
	}{
		{48.2085, 16.3731, "Wien"},
		{47.7981, 13.0465, "Salzburg"},
		{48.1372, 11.5755, "München"},
		{41.8902, 12.4922, "Rom"},
		{47.0707, 15.4395, "Graz"},
	}
	for _, c := range cases {
		p, km, ok := tab.Nearest(c.lat, c.lon)
		if !ok || p.Name != c.want || km > 10 {
			t.Errorf("Nearest(%v, %v) = %q %.1f km %v, want %s", c.lat, c.lon, p.Name, km, ok, c.want)
		}
	}
}

func TestNearestLeavesTheOpenSeaEmpty(t *testing.T) {
	tab, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	if p, _, ok := tab.Nearest(35, -40); ok {
		t.Errorf("middle of the Atlantic is near %s", p.Name)
	}
}

func TestNearestReachesAcrossTheDateLine(t *testing.T) {
	tab, err := Read(strings.NewReader("Ostrand\tXX\t0\t1799900\nWestrand\tXX\t0\t-1795000\n"))
	if err != nil {
		t.Fatal(err)
	}
	p, km, ok := tab.Nearest(0, -179.99)
	if !ok || p.Name != "Ostrand" || km > 5 {
		t.Errorf("got %q %.1f km %v", p.Name, km, ok)
	}
}

func TestReadRejectsBrokenLines(t *testing.T) {
	if _, err := Read(strings.NewReader("Wien\tAT\t482085\n")); err == nil {
		t.Error("a line without longitude was accepted")
	}
}
