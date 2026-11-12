// Command assemble puts together the modules of one platform from the
// upstream files modules/sources.json pins. desktop.yml runs it for the
// release zips, scripts/desktop.mjs for the modules inside the Mac bundle:
//
//	go run ./modules/assemble -platform windows/amd64 -release v1.4.0 -zip dist
//	go run ./modules/assemble -platform darwin/arm64 -release v1.4.0 -dir CannonCommander.app/Contents/Resources/modules/arm64
//
// Every download is checked against its SHA-256 before it is unpacked. Zips
// and 7z archives are unpacked here, tarballs by tar.
package main

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"maps"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/bodgit/sevenzip"

	"github.com/junkerderprovinz/cannoncommander/desktop/modules"
	"github.com/junkerderprovinz/cannoncommander/desktop/update"
)

func main() {
	sources := flag.String("sources", filepath.Join("modules", "sources.json"), "the pinned sources")
	platform := flag.String("platform", runtime.GOOS+"/"+runtime.GOARCH, "the platform to assemble, such as windows/arm64")
	release := flag.String("release", "", "the app version the modules go with")
	zipDir := flag.String("zip", "", "write cannoncommander-modules-<os>-<arch>.zip into this folder")
	dir := flag.String("dir", "", "write the modules into this folder instead, which must not exist yet")
	cache := flag.String("cache", "", "keep downloads in this folder and take them from there next time")
	flag.Parse()
	if *release == "" || (*zipDir == "") == (*dir == "") {
		log.Fatal("assemble needs -release and one of -zip and -dir")
	}

	data, err := os.ReadFile(*sources)
	if err != nil {
		log.Fatal(err)
	}
	src, err := modules.ParseSources(data)
	if err != nil {
		log.Fatalf("%s: %v", *sources, err)
	}
	a := &assembler{client: &http.Client{Timeout: 30 * time.Minute}, cache: *cache}
	ctx := context.Background()

	if *dir != "" {
		if err := a.assemble(ctx, src, *platform, *release, *dir); err != nil {
			log.Fatal(err)
		}
		return
	}
	goos, goarch, _ := strings.Cut(*platform, "/")
	stage, err := os.MkdirTemp("", "modules-")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(stage)
	out := filepath.Join(stage, "modules")
	if err := a.assemble(ctx, src, *platform, *release, out); err != nil {
		log.Fatal(err)
	}
	if err := os.MkdirAll(*zipDir, 0o755); err != nil {
		log.Fatal(err)
	}
	name := filepath.Join(*zipDir, modules.AssetName(goos, goarch))
	if err := writeZip(out, name); err != nil {
		log.Fatal(err)
	}
	log.Printf("wrote %s", name)
}

type assembler struct {
	client *http.Client
	cache  string
}

// assemble writes the modules of platform into out, one folder per module
// and modules.json beside them.
func (a *assembler) assemble(ctx context.Context, src modules.Sources, platform, release, out string) error {
	mods, ok := src[platform]
	if !ok {
		return fmt.Errorf("the sources name no modules for %s", platform)
	}
	if err := os.Mkdir(out, 0o755); err != nil {
		return err
	}
	rec := modules.Record{Release: release, Platform: platform, Modules: map[string]modules.Module{}}
	for _, name := range slices.Sorted(maps.Keys(mods)) {
		m := mods[name]
		for _, s := range m.Sources {
			if err := a.take(ctx, s, filepath.Join(out, name)); err != nil {
				return fmt.Errorf("%s: %s: %w", name, s.URL, err)
			}
		}
		m.Sources = nil
		rec.Modules[name] = m
		log.Printf("%s %s for %s", name, m.Version, platform)
	}
	data, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(out, modules.RecordFile), append(data, '\n'), 0o644)
}

// take fetches one source, unpacks and builds it in a folder of its own and
// copies what it keeps into the module.
func (a *assembler) take(ctx context.Context, s modules.Source, module string) error {
	file, err := a.fetch(ctx, s)
	if err != nil {
		return err
	}
	if a.cache == "" {
		defer os.Remove(file)
	}
	work, err := os.MkdirTemp("", "module-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(work)
	if err := unpack(file, sourceName(s.URL), work); err != nil {
		return fmt.Errorf("unpacking: %w", err)
	}
	if len(s.Build) > 0 {
		cmd := exec.CommandContext(ctx, s.Build[0], s.Build[1:]...)
		cmd.Dir = work
		cmd.Stdout, cmd.Stderr = os.Stderr, os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("building: %w", err)
		}
	}
	return pick(work, s.Keep, module)
}

// fetch downloads s and checks it, or takes it from the cache, checked
// again.
func (a *assembler) fetch(ctx context.Context, s modules.Source) (string, error) {
	dir := a.cache
	if dir == "" {
		dir = os.TempDir()
	} else if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	file := filepath.Join(dir, s.SHA256[:16]+"-"+sourceName(s.URL))
	if a.cache != "" {
		if err := verify(file, s.SHA256); err == nil {
			return file, nil
		}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.URL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "cannoncommander-modules")
	resp, err := a.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GET: %s", resp.Status)
	}
	f, err := os.Create(file)
	if err != nil {
		return "", err
	}
	_, err = io.Copy(f, resp.Body)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = verify(file, s.SHA256)
	}
	if err != nil {
		os.Remove(file)
		return "", err
	}
	return file, nil
}

func verify(file, want string) error {
	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	if got := hex.EncodeToString(h.Sum(nil)); got != want {
		return fmt.Errorf("checksum %s, but sources.json pins %s", got, want)
	}
	return nil
}

// sourceName is the file name at the end of a source's address.
func sourceName(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return path.Base(raw)
	}
	return path.Base(u.Path)
}

// unpack extracts the archive file, called name, into dir. A file that is no
// archive is copied there under its name.
func unpack(file, name, dir string) error {
	lower := strings.ToLower(name)
	switch {
	case strings.HasSuffix(lower, ".zip"):
		return update.Unzip(file, dir)
	case strings.Contains(lower, ".tar.") || strings.HasSuffix(lower, ".tgz"):
		return run("tar", "-xf", file, "-C", dir)
	case strings.HasSuffix(lower, ".7z"):
		return un7z(file, dir)
	}
	return copyFile(file, filepath.Join(dir, name), 0o644)
}

// un7z extracts through an os.Root, as update.Unzip does. The 7z archives
// it meets hold plain files and folders.
func un7z(file, dir string) error {
	r, err := sevenzip.OpenReader(file)
	if err != nil {
		return err
	}
	defer r.Close()
	root, err := os.OpenRoot(dir)
	if err != nil {
		return err
	}
	defer root.Close()
	for _, f := range r.File {
		name := strings.TrimSuffix(f.Name, "/")
		if f.FileInfo().IsDir() {
			if err := root.MkdirAll(name, 0o755); err != nil {
				return err
			}
			continue
		}
		if i := strings.LastIndex(name, "/"); i >= 0 {
			if err := root.MkdirAll(name[:i], 0o755); err != nil {
				return err
			}
		}
		if err := extract7z(root, f, name); err != nil {
			return fmt.Errorf("extracting %s: %w", f.Name, err)
		}
	}
	return nil
}

func extract7z(root *os.Root, f *sevenzip.File, name string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	out, err := root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, rc); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

func run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout, cmd.Stderr = os.Stderr, os.Stderr
	return cmd.Run()
}

// pick copies every file under work that matches a pattern of keep into the
// folder keep names for it inside module. A pattern that matches nothing
// means the upstream file changed shape, which is an error.
func pick(work string, keep map[string]string, module string) error {
	used := map[string]bool{}
	taken := map[string]string{}
	err := filepath.WalkDir(work, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(work, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		for pattern, dir := range keep {
			if ok, _ := path.Match(pattern, rel); !ok {
				continue
			}
			used[pattern] = true
			to := filepath.Join(module, filepath.FromSlash(dir), d.Name())
			if other, ok := taken[to]; ok {
				return fmt.Errorf("%s and %s both go to %s", other, rel, to)
			}
			taken[to] = rel
			if err := copyEntry(p, to, d); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	for _, pattern := range slices.Sorted(maps.Keys(keep)) {
		if !used[pattern] {
			return fmt.Errorf("nothing matches %q", pattern)
		}
	}
	return nil
}

func copyEntry(from, to string, d fs.DirEntry) error {
	if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
		return err
	}
	if d.Type()&fs.ModeSymlink != 0 {
		link, err := os.Readlink(from)
		if err != nil {
			return err
		}
		return os.Symlink(link, to)
	}
	info, err := d.Info()
	if err != nil {
		return err
	}
	return copyFile(from, to, info.Mode().Perm())
}

func copyFile(from, to string, perm fs.FileMode) error {
	in, err := os.Open(from)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(to, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// writeZip packs the folder dir into the zip name, with file modes and
// symlinks, which the app's unpacking keeps.
func writeZip(dir, name string) error {
	f, err := os.Create(name)
	if err != nil {
		return err
	}
	w := zip.NewWriter(f)
	err = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		h, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		h.Name = filepath.ToSlash(rel)
		h.Method = zip.Deflate
		fw, err := w.CreateHeader(h)
		if err != nil {
			return err
		}
		if d.Type()&fs.ModeSymlink != 0 {
			link, err := os.Readlink(p)
			if err != nil {
				return err
			}
			_, err = io.WriteString(fw, link)
			return err
		}
		in, err := os.Open(p)
		if err != nil {
			return err
		}
		defer in.Close()
		_, err = io.Copy(fw, in)
		return err
	})
	if cerr := w.Close(); err == nil {
		err = cerr
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	return err
}
