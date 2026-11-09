package ccfs

import (
	"errors"
	"path/filepath"
	"slices"
	"testing"
)

func TestRootSourceHoldsTheSourcesInsideIt(t *testing.T) {
	root, stick := t.TempDir(), t.TempDir()
	r, err := New(nil,
		Source{Name: "Computer", Mount: "/", Root: root},
		Source{Name: "Stick", Mount: "/media/stick", Root: stick},
	)
	if err != nil {
		t.Fatal(err)
	}
	loc, err := r.Resolve("/media/stick/fotos")
	if err != nil || loc.Source.Name != "Stick" || loc.Real != filepath.Join(stick, "fotos") {
		t.Fatalf("got %+v, %v", loc, err)
	}
	loc, err = r.Resolve("/etc/hosts")
	if err != nil || loc.Source.Name != "Computer" || loc.Index != "/etc/hosts" {
		t.Fatalf("got %+v, %v", loc, err)
	}
	if loc, err := r.Resolve("/"); err != nil || loc.Virtual() || loc.Index != "/" {
		t.Fatalf("/ is the root source itself, got %+v, %v", loc, err)
	}
}

func TestSourcesBelowTheRootStillMayNotNest(t *testing.T) {
	_, err := New(nil,
		Source{Name: "Computer", Mount: "/", Root: t.TempDir()},
		Source{Name: "Home", Mount: "/home/jdp", Root: t.TempDir()},
		Source{Name: "Docs", Mount: "/home/jdp/docs", Root: t.TempDir()},
	)
	if err == nil {
		t.Fatal("two sources inside each other below / were taken")
	}
	if _, err := New(nil, Source{Name: "A", Mount: "/", Root: t.TempDir()}, Source{Name: "B", Mount: "/", Root: t.TempDir()}); err == nil {
		t.Fatal("two sources at / were taken")
	}
}

func TestAVolumeIsASourceWhileTheComputerReportsIt(t *testing.T) {
	r, roots := newResolver(t, nil)
	stick := t.TempDir()
	mount := MountFor(stick)
	r.SetVolumes([]Volume{{Name: "STICK", Root: stick, Drive: Drive{Label: "Urlaub", Kind: "removable", Eject: true}}})

	loc, err := r.Resolve(mount + "/fotos")
	if err != nil || loc.Real != filepath.Join(stick, "fotos") {
		t.Fatalf("got %+v, %v", loc, err)
	}
	if d := loc.Source.Drive; d == nil || d.Label != "Urlaub" || !d.Eject {
		t.Fatalf("drive %+v", d)
	}
	if got := r.Volumes(); len(got) != 1 || got[0].Mount != mount {
		t.Fatalf("volumes %+v", got)
	}

	r.SetVolumes(nil)
	if _, err := r.Resolve(mount + "/fotos"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("a pulled stick still resolves: %v", err)
	}
	if _, err := r.Resolve("/mnt/user"); err != nil {
		t.Fatalf("the configured sources went with it: %v", err)
	}
	_ = roots
}

func TestAConfiguredSourceAtAVolumeGetsItsDrive(t *testing.T) {
	disk := t.TempDir()
	r, err := New(nil, Source{Name: "Daten", Mount: MountFor(disk), Root: disk, ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	r.SetVolumes([]Volume{{Name: "D:", Root: disk, Drive: Drive{Label: "Archiv", Kind: "fixed"}}})
	got := r.Volumes()
	if len(got) != 1 || got[0].Name != "Daten" || !got[0].ReadOnly || got[0].Drive.Label != "Archiv" {
		t.Fatalf("got %+v", got)
	}
	if n := len(r.Sources()); n != 1 {
		t.Fatalf("the volume came in a second time: %d sources", n)
	}
}

func TestVolumesAndDevicesKeepEachOther(t *testing.T) {
	r, _ := newResolver(t, nil)
	stick := t.TempDir()
	r = r.WithDevices("/mnt", "disk2")
	r.SetVolumes([]Volume{{Name: "STICK", Root: stick, Drive: Drive{Kind: "removable"}}})
	r.SetDevices("disk2", "disk3")
	mounts := []string{}
	for _, s := range r.Sources() {
		mounts = append(mounts, s.Mount)
	}
	for _, want := range []string{"/mnt/disk3", MountFor(stick)} {
		if !slices.Contains(mounts, want) {
			t.Fatalf("%s missing from %v", want, mounts)
		}
	}
}

type fakeNetwork map[string]map[string]string

func (n fakeNetwork) Share(server, share string) (string, bool) {
	root, ok := n[server][share]
	return root, ok
}

func (n fakeNetwork) Shares(server string) []string {
	var out []string
	for name := range n[server] {
		out = append(out, name)
	}
	slices.Sort(out)
	return out
}

func TestANetworkShareOpensByItsPath(t *testing.T) {
	daten := t.TempDir()
	r, _ := newResolver(t, nil)
	r = r.WithNetwork(fakeNetwork{"nas": {"daten": daten, "filme": t.TempDir()}})

	loc, err := r.Resolve("/net/nas/daten/fotos")
	if err != nil || loc.Real != filepath.Join(daten, "fotos") || loc.Index != "/fotos" {
		t.Fatalf("got %+v, %v", loc, err)
	}
	if d := loc.Source.Drive; d == nil || d.Kind != "network" {
		t.Fatalf("drive %+v", d)
	}
	for _, p := range []string{"/net", "/net/nas"} {
		if loc, err := r.Resolve(p); err != nil || !loc.Virtual() {
			t.Fatalf("%s: %+v, %v", p, loc, err)
		}
	}
	if got := r.VirtualChildren("/net/nas"); !slices.Equal(got, []string{"daten", "filme"}) {
		t.Fatalf("shares %v", got)
	}
	if got := r.VirtualChildren("/net"); !slices.Equal(got, []string{"nas"}) {
		t.Fatalf("servers %v", got)
	}
	if _, err := r.Resolve("/net/nas/nichts"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("a share that is not there: %v", err)
	}
}

func TestWithoutANetworkNetIsAnUnknownPath(t *testing.T) {
	r, _ := newResolver(t, nil)
	if _, err := r.Resolve("/net/nas/daten"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("got %v", err)
	}
}

func TestAVolumeInsideTheRootSourceIsFoundByAnyPathInIt(t *testing.T) {
	root := t.TempDir()
	share := filepath.Join(root, "gvfs", "smb-share:server=localhost,share=ccshare")
	r, err := New(nil, Source{Name: "Computer", Mount: "/", Root: root})
	if err != nil {
		t.Fatal(err)
	}
	r.SetVolumes([]Volume{{Name: "ccshare", Root: share, Drive: Drive{Label: "ccshare", Kind: "network", Remote: "smb://localhost/ccshare"}}})
	mount := "/gvfs/smb-share:server=localhost,share=ccshare"
	for _, p := range []string{mount, mount + "/Fotos/2024"} {
		at, v, ok := r.VolumeAt(p)
		if !ok || at != mount || v.Root != share || v.Remote != "smb://localhost/ccshare" {
			t.Errorf("%s: %s %+v %v", p, at, v, ok)
		}
	}
	if _, _, ok := r.VolumeAt("/etc"); ok {
		t.Error("a folder on no drive of its own has one")
	}
}
