//go:build ignore

// Generate writes cities.tsv.gz from the GeoNames dump: every town with at
// least 15,000 people, under its German name where GeoNames has one.
//
//	go run generate.go [dir]
//
// dir holds cities15000.zip and alternateNamesV2.zip from
// https://download.geonames.org/export/dump/. Missing files are fetched.
package main

import (
	"archive/zip"
	"bufio"
	"compress/gzip"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

const dump = "https://download.geonames.org/export/dump/"

type city struct {
	id       int
	name     string
	country  string
	lat, lon float64
}

// Sections of a city such as a Vienna district carry their own name, but a
// photo taken there belongs to the city. Historical, abandoned and
// destroyed places do not count either.
var skipCodes = map[string]bool{"PPLX": true, "PPLH": true, "PPLQ": true, "PPLW": true}

func main() {
	dir := "."
	if len(os.Args) > 1 {
		dir = os.Args[1]
	}
	cities := readCities(fetch(dir, "cities15000.zip"))
	german := readGerman(fetch(dir, "alternateNamesV2.zip"), cities)
	out, err := os.Create("cities.tsv.gz")
	if err != nil {
		log.Fatal(err)
	}
	gz, _ := gzip.NewWriterLevel(out, gzip.BestCompression)
	w := bufio.NewWriter(gz)
	ids := make([]int, 0, len(cities))
	for id := range cities {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	for _, id := range ids {
		c := cities[id]
		name := c.name
		if g, ok := german[id]; ok {
			name = g
		}
		fmt.Fprintf(w, "%s\t%s\t%d\t%d\n", name, c.country, int(math.Round(c.lat*1e4)), int(math.Round(c.lon*1e4)))
	}
	if err := w.Flush(); err != nil {
		log.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		log.Fatal(err)
	}
	if err := out.Close(); err != nil {
		log.Fatal(err)
	}
	log.Printf("%d places, %d with a German name", len(cities), len(german))
}

func fetch(dir, name string) string {
	p := filepath.Join(dir, name)
	if _, err := os.Stat(p); err == nil {
		return p
	}
	log.Printf("downloading %s", name)
	resp, err := http.Get(dump + name)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		log.Fatalf("%s: %s", name, resp.Status)
	}
	f, err := os.Create(p)
	if err != nil {
		log.Fatal(err)
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		log.Fatal(err)
	}
	if err := f.Close(); err != nil {
		log.Fatal(err)
	}
	return p
}

// lines calls fn with the fields of every line of member inside the zip.
func lines(zipPath, member string, fn func([]string)) {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		log.Fatal(err)
	}
	defer zr.Close()
	for _, f := range zr.File {
		if f.Name != member {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			log.Fatal(err)
		}
		sc := bufio.NewScanner(rc)
		sc.Buffer(make([]byte, 1<<20), 1<<24)
		for sc.Scan() {
			fn(strings.Split(sc.Text(), "\t"))
		}
		if err := sc.Err(); err != nil {
			log.Fatal(err)
		}
		rc.Close()
		return
	}
	log.Fatalf("%s has no %s", zipPath, member)
}

func readCities(zipPath string) map[int]city {
	out := map[int]city{}
	lines(zipPath, "cities15000.txt", func(f []string) {
		if len(f) < 9 || skipCodes[f[7]] {
			return
		}
		id, err1 := strconv.Atoi(f[0])
		lat, err2 := strconv.ParseFloat(f[4], 64)
		lon, err3 := strconv.ParseFloat(f[5], 64)
		if err1 != nil || err2 != nil || err3 != nil {
			return
		}
		out[id] = city{id: id, name: f[1], country: f[8], lat: lat, lon: lon}
	})
	return out
}

// readGerman picks one German name per city: a preferred one first, else
// the first that is neither short, colloquial nor historic.
func readGerman(zipPath string, cities map[int]city) map[int]string {
	preferred, plain := map[int]string{}, map[int]string{}
	lines(zipPath, "alternateNamesV2.txt", func(f []string) {
		if len(f) < 8 || f[2] != "de" {
			return
		}
		id, err := strconv.Atoi(f[1])
		if err != nil {
			return
		}
		if _, ok := cities[id]; !ok || f[6] == "1" || f[7] == "1" {
			return
		}
		switch {
		case f[4] == "1":
			if _, ok := preferred[id]; !ok {
				preferred[id] = f[3]
			}
		case f[5] != "1":
			if _, ok := plain[id]; !ok {
				plain[id] = f[3]
			}
		}
	})
	for id, n := range plain {
		if _, ok := preferred[id]; !ok {
			preferred[id] = n
		}
	}
	return preferred
}
