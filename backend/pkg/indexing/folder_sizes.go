package indexing

import (
	"hash/maphash"
	"math/bits"
	"strconv"
	"strings"
)

// folderSizes holds the size of every folder the index has measured, keyed
// by index path. A folder is stored as a node that knows its parent and its
// own name, so the long shared beginnings of millions of deep paths are kept
// once, and nothing in it holds a pointer the garbage collector has to
// follow. In BenchmarkRestoreFolderSizes a map keyed by full paths needs
// about 128 bytes per folder, this about 77.
//
// A parent always comes before its children in nodes, so one pass in order
// can tell for every node whether it lies below a given folder. Nodes are
// only taken out by compact, which renumbers them. A nil *folderSizes reads
// as empty.
type folderSizes struct {
	nodes []folderNode
	names []byte
	kids  map[uint64]uint32
	// clash holds the folders whose key a sibling took first.
	clash map[string]uint32
	sized int
}

type folderNode struct {
	size    uint64
	parent  uint32
	nameAt  uint32
	nameLen uint16
	flags   uint8
}

const (
	hasSize uint8 = 1 << iota
	unsynced
)

var nameSeed = maphash.MakeSeed()

func newFolderSizes() *folderSizes {
	return &folderSizes{nodes: make([]folderNode, 1), kids: map[uint64]uint32{}}
}

// key folds parent and the hash of a name into one word, which keeps the
// map slots at 16 bytes. A key that two folders share sends the second to
// clash.
func (f *folderSizes) key(parent uint32, name string) uint64 {
	return mixParent(parent, maphash.String(nameSeed, name))
}

func mixParent(parent uint32, name uint64) uint64 {
	return name ^ bits.RotateLeft64(uint64(parent)*0x9e3779b97f4a7c15, 32)
}

func clashKey(parent uint32, name string) string {
	return strconv.FormatUint(uint64(parent), 10) + "/" + name
}

func (f *folderSizes) name(id uint32) []byte {
	n := &f.nodes[id]
	return f.names[n.nameAt : n.nameAt+uint32(n.nameLen)]
}

func (f *folderSizes) child(parent uint32, name string) (uint32, bool) {
	if id, ok := f.kids[f.key(parent, name)]; ok && f.nodes[id].parent == parent && string(f.name(id)) == name {
		return id, true
	}
	if len(f.clash) == 0 {
		return 0, false
	}
	id, ok := f.clash[clashKey(parent, name)]
	return id, ok
}

func (f *folderSizes) addChild(parent uint32, name string) uint32 {
	id := uint32(len(f.nodes))
	f.nodes = append(f.nodes, folderNode{parent: parent, nameAt: uint32(len(f.names)), nameLen: uint16(len(name))})
	f.names = append(f.names, name...)
	k := f.key(parent, name)
	if _, taken := f.kids[k]; !taken {
		f.kids[k] = id
		return id
	}
	if f.clash == nil {
		f.clash = map[string]uint32{}
	}
	f.clash[clashKey(parent, name)] = id
	return id
}

// walk follows path from the root, adding the folders that are missing
// when add is set.
func (f *folderSizes) walk(path string, add bool) (uint32, bool) {
	var id uint32
	for rest := path; rest != ""; {
		var name string
		name, rest, _ = strings.Cut(rest, "/")
		if name == "" {
			continue
		}
		next, ok := f.child(id, name)
		if !ok {
			if !add {
				return 0, false
			}
			next = f.addChild(id, name)
		}
		id = next
	}
	return id, true
}

// lookup returns the node of path, whether or not it has a size.
func (f *folderSizes) lookup(path string) (uint32, bool) {
	if f == nil {
		return 0, false
	}
	return f.walk(path, false)
}

func (f *folderSizes) node(path string) uint32 {
	id, _ := f.walk(path, true)
	return id
}

func (f *folderSizes) get(path string) (uint64, bool) {
	id, ok := f.lookup(path)
	if !ok || f.nodes[id].flags&hasSize == 0 {
		return 0, false
	}
	return f.nodes[id].size, true
}

// size returns the size of id and whether it has one.
func (f *folderSizes) size(id uint32) (uint64, bool) {
	n := &f.nodes[id]
	return n.size, n.flags&hasSize != 0
}

// setID gives id a size and marks it for the next sync to the database.
func (f *folderSizes) setID(id uint32, size uint64) {
	n := &f.nodes[id]
	if n.flags&hasSize == 0 {
		f.sized++
	}
	n.size = size
	n.flags |= hasSize | unsynced
}

func (f *folderSizes) set(path string, size uint64) { f.setID(f.node(path), size) }

// restore sets a size the database already holds.
func (f *folderSizes) restore(path string, size uint64) {
	id := f.node(path)
	f.setID(id, size)
	f.nodes[id].flags &^= unsynced
}

// remove drops the size of path and returns the one it had. The folders
// below keep theirs.
func (f *folderSizes) remove(path string) (uint64, bool) {
	id, ok := f.lookup(path)
	if !ok {
		return 0, false
	}
	return f.removeID(id)
}

func (f *folderSizes) removeID(id uint32) (uint64, bool) {
	n := &f.nodes[id]
	if n.flags&hasSize == 0 {
		return 0, false
	}
	f.sized--
	n.flags = 0
	return n.size, true
}

func (f *folderSizes) synced(id uint32) { f.nodes[id].flags &^= unsynced }

func (f *folderSizes) len() int {
	if f == nil {
		return 0
	}
	return f.sized
}

// path is the index path of id, with the trailing slash of a folder.
func (f *folderSizes) path(id uint32) string {
	if id == 0 {
		return "/"
	}
	var stack [32]uint32
	up := stack[:0]
	n := 1
	for i := id; i != 0; i = f.nodes[i].parent {
		up = append(up, i)
		n += int(f.nodes[i].nameLen) + 1
	}
	var b strings.Builder
	b.Grow(n)
	b.WriteByte('/')
	for k := len(up) - 1; k >= 0; k-- {
		b.Write(f.name(up[k]))
		b.WriteByte('/')
	}
	return b.String()
}

// each calls fn for every folder with a size.
func (f *folderSizes) each(fn func(id uint32)) {
	if f == nil {
		return
	}
	for i := range f.nodes {
		if f.nodes[i].flags&hasSize != 0 {
			fn(uint32(i))
		}
	}
}

func (f *folderSizes) eachUnsynced(fn func(id uint32)) {
	if f == nil {
		return
	}
	for i := range f.nodes {
		if f.nodes[i].flags&unsynced != 0 {
			fn(uint32(i))
		}
	}
}

// eachBelow calls fn for the folders with a size at or below any of tops.
func (f *folderSizes) eachBelow(tops []string, fn func(id uint32)) {
	if f == nil {
		return
	}
	below := make([]bool, len(f.nodes))
	first := len(f.nodes)
	for _, t := range tops {
		if id, ok := f.lookup(t); ok {
			below[id] = true
			first = min(first, int(id))
		}
	}
	for i := first; i < len(f.nodes); i++ {
		n := &f.nodes[i]
		if i > 0 && below[n.parent] {
			below[i] = true
		}
		if below[i] && n.flags&hasSize != 0 {
			fn(uint32(i))
		}
	}
}

// sparse reports whether most nodes are left over from folders that are
// gone, so compacting would give back memory.
func (f *folderSizes) sparse() bool {
	return f != nil && len(f.nodes) > 4096 && len(f.nodes) > 2*f.sized
}

// compact drops the nodes that neither have a size nor lead to one. It
// renumbers the nodes, so nobody may hold an id across it.
func (f *folderSizes) compact() {
	keep := make([]bool, len(f.nodes))
	keep[0] = true
	for i := len(f.nodes) - 1; i > 0; i-- {
		if keep[i] || f.nodes[i].flags&hasSize != 0 {
			keep[i] = true
			keep[f.nodes[i].parent] = true
		}
	}
	out := &folderSizes{nodes: make([]folderNode, 1, f.sized+1), kids: make(map[uint64]uint32, f.sized)}
	out.nodes[0] = f.nodes[0]
	renumbered := make([]uint32, len(f.nodes))
	for i := 1; i < len(f.nodes); i++ {
		if !keep[i] {
			continue
		}
		old, name := f.nodes[i], f.name(uint32(i))
		id := uint32(len(out.nodes))
		renumbered[i] = id
		parent := renumbered[old.parent]
		out.nodes = append(out.nodes, folderNode{size: old.size, parent: parent, nameAt: uint32(len(out.names)), nameLen: old.nameLen, flags: old.flags})
		out.names = append(out.names, name...)
		// Bytes hashes like String, so the names need no copy to be keyed.
		k := mixParent(parent, maphash.Bytes(nameSeed, name))
		if _, taken := out.kids[k]; !taken {
			out.kids[k] = id
			continue
		}
		if out.clash == nil {
			out.clash = map[string]uint32{}
		}
		out.clash[clashKey(parent, string(name))] = id
	}
	out.sized = f.sized
	*f = *out
}
