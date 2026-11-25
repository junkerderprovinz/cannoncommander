package indexing

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// pngOfKilobyte is a PNG header padded to the size of the pieces typedTree
// writes, so a file can change its content and keep its size.
var pngOfKilobyte = append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, 1016)...)

// pieceIndex indexes one piece of unknown type whose time lies an hour
// back, as on a share that was scanned before.
func pieceIndex(t *testing.T) (idx *Index, file string, then time.Time) {
	t.Helper()
	root := typedTree(t, 1, 1, ".sj1")
	file = filepath.Join(root, "pieces", "00", "0000", "piece-0000.sj1")
	then = time.Now().Add(-time.Hour).Truncate(time.Second)
	if err := os.Chtimes(file, then, then); err != nil {
		t.Fatal(err)
	}
	idx = scanIndex(t, root)
	fullScan(t, idx)
	return idx, file, then
}

const piecePath = "/pieces/00/0000/piece-0000.sj1"

func fullScan(t *testing.T, idx *Index) {
	t.Helper()
	if err := idx.createScanner("/pieces/").runFullScanChild(); err != nil {
		t.Fatal(err)
	}
}

func storedType(t *testing.T, idx *Index) string {
	t.Helper()
	item, err := idx.db.GetItem(idx.Name, piecePath)
	if err != nil || item == nil {
		t.Fatalf("no row for the piece: %v", err)
	}
	return item.Type
}

func setStored(t *testing.T, idx *Index, column string, value any) {
	t.Helper()
	if _, err := idx.db.Exec("UPDATE index_items SET "+column+" = ? WHERE source = ? AND path = ?", value, idx.Name, piecePath); err != nil {
		t.Fatal(err)
	}
}

func rewrite(t *testing.T, file string, body []byte, mtime time.Time) {
	t.Helper()
	if err := os.WriteFile(file, body, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(file, mtime, mtime); err != nil {
		t.Fatal(err)
	}
}

func TestAFullScanTakesTheTypeOfAnUnchangedFileFromTheIndex(t *testing.T) {
	idx, _, _ := pieceIndex(t)
	if got := storedType(t, idx); got != "blob" {
		t.Fatalf("first scan stored %q, want blob", got)
	}
	setStored(t, idx, "type", "application/x-stored")
	fullScan(t, idx)
	if got := storedType(t, idx); got != "application/x-stored" {
		t.Errorf("the second scan read the file again and stored %q", got)
	}
}

func TestAFullScanReadsAFileThatChangedItsSize(t *testing.T) {
	idx, file, then := pieceIndex(t)
	rewrite(t, file, pngOfKilobyte[:512], then)
	fullScan(t, idx)
	if got := storedType(t, idx); got != "image/png" {
		t.Errorf("type %q, want image/png", got)
	}
}

func TestAFullScanReadsAFileThatChangedItsTime(t *testing.T) {
	idx, file, then := pieceIndex(t)
	rewrite(t, file, pngOfKilobyte, then.Add(time.Minute))
	fullScan(t, idx)
	if got := storedType(t, idx); got != "image/png" {
		t.Errorf("type %q, want image/png", got)
	}
}

func TestAFullScanReadsAFileWhoseRowHasNoType(t *testing.T) {
	idx, _, _ := pieceIndex(t)
	setStored(t, idx, "type", "")
	fullScan(t, idx)
	if got := storedType(t, idx); got != "blob" {
		t.Errorf("type %q, want blob", got)
	}
}

// A file can change again in the second the scan read it, with its time
// still showing that second. Its row then proves nothing, as git treats an
// entry as racy when the file is as new as the index.
func TestAFullScanReadsAFileThatChangedInTheSecondItsRowWasWritten(t *testing.T) {
	idx, file, then := pieceIndex(t)
	setStored(t, idx, "last_updated", then.Unix())
	rewrite(t, file, pngOfKilobyte, then)
	fullScan(t, idx)
	if got := storedType(t, idx); got != "image/png" {
		t.Errorf("type %q, want image/png", got)
	}
}

// Size and time are all a full scan compares, as rsync's quick check does,
// so content rewritten with both kept as they were keeps its old type until
// the time moves or the file is opened in a listing.
func TestAFullScanTrustsAnUnchangedSizeAndTime(t *testing.T) {
	idx, file, then := pieceIndex(t)
	rewrite(t, file, pngOfKilobyte, then)
	fullScan(t, idx)
	if got := storedType(t, idx); got != "blob" {
		t.Errorf("type %q, want the stored blob", got)
	}
}
