package main

import (
	"slices"
	"testing"
)

func TestFilesCutHereReadBackAsCutInEveryFormat(t *testing.T) {
	paths := []string{"/home/jdp/Bilder/Strand 2024.jpg", "/home/jdp/Grüße#1?.txt", "/home/jdp/100%"}
	for _, cut := range []bool{true, false} {
		gnome, uris, kde, text := clipFormats(paths, cut)
		got, gotCut := parseGnomeFiles(gnome)
		if !slices.Equal(got, paths) || gotCut != cut {
			t.Fatalf("gnome %q: got %v cut=%v", gnome, got, gotCut)
		}
		if got := parseURIList(uris); !slices.Equal(got, paths) {
			t.Fatalf("uri list %q: got %v", uris, got)
		}
		if (kde == "1") != cut {
			t.Fatalf("kde mark %q for cut=%v", kde, cut)
		}
		if text != "/home/jdp/Bilder/Strand 2024.jpg\n/home/jdp/Grüße#1?.txt\n/home/jdp/100%" {
			t.Fatalf("text %q", text)
		}
	}
}

func TestOtherFileManagersListsAreRead(t *testing.T) {
	gnome := "cut\nfile:///home/jdp/a%20b.txt\nfile:///home/jdp/c.txt\n\x00"
	if got, cut := parseGnomeFiles(gnome); !slices.Equal(got, []string{"/home/jdp/a b.txt", "/home/jdp/c.txt"}) || !cut {
		t.Fatalf("got %v cut=%v", got, cut)
	}
	uris := "# from Dolphin\r\nfile://localhost/home/jdp/x.txt\r\nhttps://example.org/y\r\nsmb://nas/share/z\r\nfile:///home/jdp/%C3%BC.txt\r\n"
	if got := parseURIList(uris); !slices.Equal(got, []string{"/home/jdp/x.txt", "/home/jdp/ü.txt"}) {
		t.Fatalf("got %v", got)
	}
}
