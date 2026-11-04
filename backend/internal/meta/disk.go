package meta

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"os"
	"strings"
)

// probeSize covers every superblock looked at here: ISO 9660 at 32 KiB,
// the El Torito record behind it and btrfs at 64 KiB.
const probeSize = 0x11000

// espGUID is the EFI system partition type as GPT stores it on disk.
var espGUID = []byte{0x28, 0x73, 0x2A, 0xC1, 0x1F, 0xF8, 0xD2, 0x11, 0xBA, 0x4B, 0x00, 0xA0, 0xC9, 0x3E, 0xC9, 0x3B}

func readDisk(_ context.Context, _ *Service, f File, rec Record) error {
	fh, err := os.Open(f.Real)
	if err != nil {
		return err
	}
	defer fh.Close()
	b := readAt(fh, 0, probeSize)
	if fs, label := fileSystem(b); fs != "" {
		rec["fs"], rec["volume"] = fs, label
		if fs == "ISO 9660" || fs == "UDF" {
			rec["bootable"] = elTorito(b)
		}
		return nil
	}
	if len(b) < 512 || b[510] != 0x55 || b[511] != 0xAA {
		return nil
	}
	start, boots := firstPartition(fh, b)
	rec["bootable"] = boots
	if start > 0 {
		rec["fs"], rec["volume"] = fileSystem(readAt(fh, start, probeSize))
	}
	return nil
}

func readAt(r io.ReaderAt, off int64, n int) []byte {
	b := make([]byte, n)
	got, _ := r.ReadAt(b, off)
	return b[:got]
}

// firstPartition reads an MBR or GPT partition table and returns where the
// first partition starts and whether the disk would boot: a partition
// marked active, or an EFI system partition.
func firstPartition(r io.ReaderAt, mbr []byte) (start int64, boots bool) {
	entry := mbr[446:462]
	if entry[4] != 0xEE {
		for i := 0; i < 4; i++ {
			if mbr[446+16*i] == 0x80 {
				boots = true
			}
		}
		return int64(binary.LittleEndian.Uint32(entry[8:])) * 512, boots
	}
	head := readAt(r, 512, 92)
	if len(head) < 92 || string(head[:8]) != "EFI PART" {
		return 0, false
	}
	table := int64(binary.LittleEndian.Uint64(head[72:])) * 512
	count := min(int(binary.LittleEndian.Uint32(head[80:])), 128)
	size := int(binary.LittleEndian.Uint32(head[84:]))
	if size < 128 || size > 4096 {
		return 0, false
	}
	entries := readAt(r, table, count*size)
	for i := 0; i+size <= len(entries); i += size {
		e := entries[i : i+size]
		if bytes.Equal(e[:16], make([]byte, 16)) {
			continue
		}
		if bytes.Equal(e[:16], espGUID) {
			boots = true
		}
		if start == 0 {
			start = int64(binary.LittleEndian.Uint64(e[32:])) * 512
		}
	}
	return start, boots
}

func at(b []byte, off int, s string) bool {
	return len(b) >= off+len(s) && string(b[off:off+len(s)]) == s
}

func label(b []byte, off, n int) string {
	if len(b) < off+n {
		return ""
	}
	s := strings.TrimRight(string(bytes.TrimRight(b[off:off+n], "\x00")), " ")
	if s == "NO NAME" {
		return ""
	}
	return strings.TrimSpace(s)
}

// fileSystem recognises a file system by its superblock at the start of b.
func fileSystem(b []byte) (fs, name string) {
	switch {
	case at(b, 0x8001, "CD001"):
		for off := 0x8000; off+6 <= len(b); off += 0x800 {
			if at(b, off+1, "BEA01") || at(b, off+1, "NSR02") || at(b, off+1, "NSR03") {
				return "UDF", label(b, 0x8028, 32)
			}
		}
		return "ISO 9660", label(b, 0x8028, 32)
	case at(b, 0x8001, "BEA01"):
		return "UDF", ""
	case at(b, 3, "NTFS    "):
		return "NTFS", ""
	case at(b, 3, "EXFAT   "):
		return "exFAT", ""
	case at(b, 0x52, "FAT32   "):
		return "FAT32", label(b, 0x47, 11)
	case at(b, 0x36, "FAT16") || at(b, 0x36, "FAT12"):
		return string(b[0x36:0x3B]), label(b, 0x2B, 11)
	case len(b) > 0x478+16 && binary.LittleEndian.Uint16(b[0x438:]) == 0xEF53:
		compat := binary.LittleEndian.Uint32(b[0x45C:])
		incompat := binary.LittleEndian.Uint32(b[0x460:])
		name := label(b, 0x478, 16)
		switch {
		case incompat&(0x40|0x80|0x200) != 0:
			return "ext4", name
		case compat&0x4 != 0:
			return "ext3", name
		}
		return "ext2", name
	case at(b, 0, "XFSB"):
		return "XFS", label(b, 108, 12)
	case at(b, 0x10040, "_BHRfS_M"):
		return "Btrfs", label(b, 0x1012B, 256)
	case at(b, 0, "hsqs"):
		return "SquashFS", ""
	case at(b, 0, "LUKS\xba\xbe"):
		return "LUKS", ""
	}
	return "", ""
}

// elTorito looks for the boot record a bootable CD or DVD image carries in
// the sector after its volume descriptor.
func elTorito(b []byte) bool {
	return len(b) > 0x8800+30 && b[0x8800] == 0 && at(b, 0x8801, "CD001") && at(b, 0x8807, "EL TORITO SPECIFICATION")
}
