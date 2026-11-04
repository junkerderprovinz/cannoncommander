// Package places names the town nearest to a GPS position, offline, from the
// GeoNames list of places with at least 15,000 people.
package places

//go:generate go run generate.go

import (
	"bufio"
	"bytes"
	"compress/gzip"
	_ "embed"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"sync"
)

// cities.tsv.gz holds name, country code, latitude and longitude in
// ten-thousandths of a degree, one place per line. Data by GeoNames under
// CC BY 4.0.
//
//go:embed cities.tsv.gz
var embedded []byte

// MaxDistance is how far from a town a position may be and still carry its
// name, in kilometres. Further out at sea or in the mountains the column
// stays empty rather than naming a town nobody would think of.
const MaxDistance = 100

const earthRadius = 6371.0

// Place is one town.
type Place struct {
	Name    string
	Country string
	Lat     float64
	Lon     float64
}

// Table finds places by position. Positions are bucketed in cells of one
// degree, so a lookup only measures the towns of a few cells.
type Table struct {
	places []Place
	cells  map[int][]int32
}

var (
	defaultOnce  sync.Once
	defaultTable *Table
	defaultErr   error
)

// Default returns the embedded table, read on first use.
func Default() (*Table, error) {
	defaultOnce.Do(func() {
		zr, err := gzip.NewReader(bytes.NewReader(embedded))
		if err != nil {
			defaultErr = err
			return
		}
		defaultTable, defaultErr = Read(zr)
	})
	return defaultTable, defaultErr
}

// Read builds a table from lines in the form of cities.tsv.gz, unpacked.
func Read(r io.Reader) (*Table, error) {
	t := &Table{cells: map[int][]int32{}}
	sc := bufio.NewScanner(r)
	for n := 1; sc.Scan(); n++ {
		f := strings.Split(sc.Text(), "\t")
		if len(f) != 4 {
			return nil, fmt.Errorf("places: line %d has %d fields", n, len(f))
		}
		lat, err1 := strconv.Atoi(f[2])
		lon, err2 := strconv.Atoi(f[3])
		if err1 != nil || err2 != nil {
			return nil, fmt.Errorf("places: line %d has a bad position", n)
		}
		t.add(Place{Name: f[0], Country: f[1], Lat: float64(lat) / 1e4, Lon: float64(lon) / 1e4})
	}
	return t, sc.Err()
}

func (t *Table) add(p Place) {
	t.places = append(t.places, p)
	c := cellOf(p.Lat, p.Lon)
	t.cells[c] = append(t.cells[c], int32(len(t.places)-1))
}

func cellOf(lat, lon float64) int {
	return key(int(math.Floor(lat)), int(math.Floor(lon)))
}

func key(latCell, lonCell int) int {
	lonCell = ((lonCell+180)%360+360)%360 - 180
	return (latCell+90)*360 + lonCell + 180
}

// Nearest returns the town closest to the position and its distance in
// kilometres. ok is false when no town lies within MaxDistance.
func (t *Table) Nearest(lat, lon float64) (p Place, km float64, ok bool) {
	if t == nil || math.IsNaN(lat) || math.IsNaN(lon) || lat < -90 || lat > 90 || lon < -180 || lon > 180 {
		return Place{}, 0, false
	}
	dLat := int(math.Ceil(MaxDistance / 111.0))
	// A degree of longitude shrinks towards the poles, so more cells are
	// needed there to cover the same distance.
	cos := math.Cos((math.Abs(lat) + float64(dLat)) * math.Pi / 180)
	dLon := 180
	if cos > 0.01 {
		dLon = min(180, int(math.Ceil(MaxDistance/(111.0*cos))))
	}
	la, lo := int(math.Floor(lat)), int(math.Floor(lon))
	best := -1
	km = math.Inf(1)
	for y := la - dLat; y <= la+dLat; y++ {
		if y < -90 || y > 89 {
			continue
		}
		for x := lo - dLon; x <= lo+dLon; x++ {
			for _, i := range t.cells[key(y, x)] {
				if d := distance(lat, lon, t.places[i].Lat, t.places[i].Lon); d < km {
					best, km = int(i), d
				}
			}
		}
	}
	if best < 0 || km > MaxDistance {
		return Place{}, 0, false
	}
	return t.places[best], km, true
}

// distance is the great circle distance in kilometres.
func distance(lat1, lon1, lat2, lon2 float64) float64 {
	const rad = math.Pi / 180
	dLat := (lat2 - lat1) * rad
	dLon := (lon2 - lon1) * rad
	a := math.Sin(dLat/2)*math.Sin(dLat/2) + math.Cos(lat1*rad)*math.Cos(lat2*rad)*math.Sin(dLon/2)*math.Sin(dLon/2)
	return 2 * earthRadius * math.Asin(math.Min(1, math.Sqrt(a)))
}
