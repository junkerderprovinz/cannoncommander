package meta

import (
	"context"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"hash/crc32"
	"io"
	"os"
)

// readSums works out all four checksums in one read, the disk is the slow
// part and not the hashing.
func readSums(ctx context.Context, _ *Service, f File, rec Record) error {
	fh, err := os.Open(f.Real)
	if err != nil {
		return err
	}
	defer fh.Close()
	s256, s1, m5, c32 := sha256.New(), sha1.New(), md5.New(), crc32.NewIEEE()
	w := io.MultiWriter(s256, s1, m5, c32)
	buf := make([]byte, 1<<20)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		n, err := fh.Read(buf)
		w.Write(buf[:n])
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}
	rec["sha256"] = hex.EncodeToString(s256.Sum(nil))
	rec["sha1"] = hex.EncodeToString(s1.Sum(nil))
	rec["md5"] = hex.EncodeToString(m5.Sum(nil))
	rec["crc32"] = hex.EncodeToString(c32.Sum(nil))
	return nil
}
