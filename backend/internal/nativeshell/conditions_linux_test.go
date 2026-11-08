package nativeshell

import (
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
)

// conditionFixture is a desktop with a system disk, a USB stick, an
// encrypted volume on the stick, an sshfs mount and a second stick whose
// mount point has a space, each holding one picture. gsettings and the
// session bus answer from small tables, and the programs that ran are
// noted in calls.
func conditionFixture(t *testing.T) (f *fixture, files map[string]string, calls *[]string) {
	f = newFixture(t)
	f.installed = append(f.installed, "gsettings")
	work, err := filepath.EvalSymlinks(filepath.Dir(f.files("a.png")[0]))
	if err != nil {
		t.Fatal(err)
	}
	files = map[string]string{"disk": filepath.Join(work, "a.png")}
	for where, name := range map[string]string{"usb": "usb/b.png", "crypt": "crypt/c.png", "remote": "remote/d.png", "stick": "my stick/e.png"} {
		files[where] = f.files(name)[0]
	}
	file := func(s string) *fstest.MapFile { return &fstest.MapFile{Data: []byte(s)} }
	f.root = fstest.MapFS{
		"proc/self/mountinfo": file("22 1 8:2 / / rw,relatime shared:1 - ext4 /dev/sda2 rw\n" +
			"40 22 8:17 / " + work + "/usb rw,nosuid - vfat /dev/sdb1 rw\n" +
			"41 22 253:0 / " + work + "/crypt rw - ext4 /dev/mapper/luks-1 rw\n" +
			"42 22 0:50 / " + work + "/remote rw - fuse.sshfs host:/srv rw\n" +
			"43 22 8:33 / " + work + "/my\\040stick rw - exfat /dev/sdc1 rw\n"),
		"sys/dev/block/8:2/uevent":       file("MAJOR=8\nMINOR=2\nDEVNAME=sda2\nDEVTYPE=partition\n"),
		"sys/dev/block/8:17/uevent":      file("MAJOR=8\nMINOR=17\nDEVNAME=sdb1\nDEVTYPE=partition\n"),
		"sys/dev/block/253:0/uevent":     file("MAJOR=253\nMINOR=0\nDEVNAME=dm-0\nDEVTYPE=disk\n"),
		"sys/dev/block/8:33/uevent":      file("DEVNAME=sdc1\n"),
		"sys/block/sda/removable":        file("0\n"),
		"sys/block/sda/sda2/partition":   file("2\n"),
		"sys/block/sdb/removable":        file("1\n"),
		"sys/block/sdb/sdb1/partition":   file("1\n"),
		"sys/block/sdb/sdb2/partition":   file("2\n"),
		"sys/block/dm-0/removable":       file("0\n"),
		"sys/block/dm-0/slaves/sdb2/dev": file("8:18\n"),
		"sys/block/sdc/removable":        file("1\n"),
		"sys/block/sdc/sdc1/partition":   file("1\n"),
	}
	f.owners = []string{"org.example.Running"}
	settings := map[string]string{
		"org.test flag": "true", "org.test off": "false", "org.test count": "5", "org.test big": "uint32 7",
		"org.test name": "'Mint Y'", "org.test scale": "1.25", "org.test small": "byte 0x05",
	}
	calls = new([]string)
	f.output = func(program string, args ...string) ([]byte, error) {
		*calls = append(*calls, strings.Join(append([]string{program}, args...), "|"))
		switch {
		case program == "/usr/bin/gsettings" && len(args) == 3:
			if v, ok := settings[args[1]+" "+args[2]]; ok {
				return []byte(v + "\n"), nil
			}
			return nil, errors.New("no such key")
		case strings.HasSuffix(program, "yes") || strings.HasSuffix(program, "/check.sh"):
			return nil, nil
		}
		return nil, errors.New("exit status 1")
	}
	return f, files, calls
}

func TestEachNemoConditionHoldsWhereNemoWouldShowTheAction(t *testing.T) {
	f, files, _ := conditionFixture(t)
	k := f.desk()
	cases := []struct {
		condition, file string
		want            bool
	}{
		{"desktop", "disk", false},
		{"sidebar-only", "disk", false},
		{"sidebar-allow", "disk", true},
		{"removable", "disk", false},
		{"removable", "usb", true},
		{"removable", "crypt", true},
		{"removable", "stick", true},
		{"removable", "remote", false},
		{"filesystem ext4", "disk", true},
		{"filesystem ext4", "crypt", true},
		{"filesystem ext4", "usb", false},
		{"filesystem vfat", "usb", true},
		{"filesystem exfat", "stick", true},
		{"filesystem sshfs", "remote", true},
		{"filesystem fuse.sshfs", "remote", true},
		{"filesystem", "disk", false},
		{"dbus org.example.Running", "disk", true},
		{"dbus org.example.Gone", "disk", false},
		{"dbus", "disk", false},
		{"gsettings org.test flag", "disk", true},
		{"gsettings org.test off", "disk", false},
		{"gsettings org.test missing", "disk", false},
		{"gsettings org.test count", "disk", false},
		{"gsettings org.test", "disk", false},
		{"gsettings org.test count i gt 3", "disk", true},
		{"gsettings org.test count i lt 3", "disk", false},
		{"gsettings org.test count i eq 5", "disk", true},
		{"gsettings org.test count u eq 5", "disk", false},
		{"gsettings org.test count i ge 3", "disk", false},
		{"gsettings org.test big u eq 7", "disk", true},
		{"gsettings org.test big u eq uint32 7", "disk", true},
		{"gsettings org.test big i eq 7", "disk", false},
		{"gsettings org.test small y eq 5", "disk", true},
		{"gsettings org.test name s eq 'Mint Y'", "disk", true},
		{"gsettings org.test name s ne 'Mint Y'", "disk", false},
		{"gsettings org.test name s lt 'Z'", "disk", true},
		{"gsettings org.test name s eq Mint", "disk", false},
		{"gsettings org.test scale d gt 1", "disk", true},
		{"gsettings org.test flag b eq true", "disk", true},
		{"gsettings org.test flag b ne true", "disk", false},
		{"gsettings org.test flag as eq ['x']", "disk", false},
		{"exec yes", "disk", true},
		{"exec no", "disk", false},
		{"exec <check.sh %f>", "disk", true},
		{"exec", "disk", false},
		{"Desktop", "disk", false},
		{"something-new", "disk", false},
	}
	dir := filepath.Join(f.system, "nemo", "actions")
	for _, c := range cases {
		sel, err := k.selection([]string{files[c.file]})
		if err != nil {
			t.Fatal(err)
		}
		if got := k.nemoCondition(c.condition, map[string]string{}, dir, sel); got != c.want {
			t.Errorf("%q on %s: got %v", c.condition, c.file, got)
		}
	}
}

func TestAnExecConditionRunsItsProgramWithTheFilesFilledIn(t *testing.T) {
	f, files, calls := conditionFixture(t)
	k := f.desk()
	dir := filepath.Join(f.system, "nemo", "actions")
	sel, err := k.selection([]string{files["stick"]})
	if err != nil {
		t.Fatal(err)
	}
	if !k.nemoCondition("exec yes %f", map[string]string{"Quote": "double"}, dir, sel) {
		t.Fatal("the condition failed")
	}
	// Only a command wrapped in brackets as a whole is next to the action.
	if k.nemoCondition("exec <check.sh> --for %F", map[string]string{}, dir, sel) {
		t.Fatal("a half bracketed command ran from the action's folder")
	}
	if want := []string{"yes|e.png", "<check.sh>|--for|" + files["stick"]}; !slices.Equal(*calls, want) {
		t.Errorf("calls %q", *calls)
	}
	*calls = nil
	if !k.nemoCondition("exec <check.sh --for %F>", map[string]string{}, dir, sel) {
		t.Fatal("the condition failed")
	}
	if want := filepath.Join(dir, "check.sh") + "|--for|" + files["stick"]; !slices.Equal(*calls, []string{want}) {
		t.Errorf("calls %q, want %s", *calls, want)
	}
	f.output = nil
	if f.desk().nemoCondition("exec yes", map[string]string{}, dir, sel) {
		t.Error("a condition held with no program to ask")
	}
}

func TestAllConditionsOfANemoActionMustHoldAndRunLast(t *testing.T) {
	f, files, calls := conditionFixture(t)
	actions := filepath.Join(f.system, "nemo", "actions")
	write := func(name, conditions string) {
		f.write(filepath.Join(actions, name+".nemo_action"), "[Nemo Action]\nName="+name+"\nExec=x %F\nSelection=s\nExtensions=png;\nConditions="+conditions+"\n")
	}
	write("running", "dbus org.example.Running;sidebar-allow;")
	write("both", "dbus org.example.Running;removable;")
	write("stick", "removable;filesystem vfat;exec yes;")
	f.write(filepath.Join(actions, "text.nemo_action"), "[Nemo Action]\nName=text\nExec=x %F\nSelection=s\nExtensions=txt;\nConditions=exec yes;\n")
	k := f.desk()
	cases := map[string]string{"disk": "nemo:running.nemo_action", "usb": "nemo:both.nemo_action nemo:running.nemo_action nemo:stick.nemo_action"}
	for where, want := range cases {
		sel, err := k.selection([]string{files[where]})
		if err != nil {
			t.Fatal(err)
		}
		var got []string
		for _, a := range k.nemo(sel) {
			got = append(got, a.Key)
		}
		slices.Sort(got)
		if strings.Join(got, " ") != want {
			t.Errorf("%s: got %q", where, got)
		}
	}
	for _, c := range *calls {
		if !strings.HasSuffix(c, "yes") {
			t.Errorf("ran %s", c)
		}
	}
	if len(*calls) != 1 {
		t.Errorf("exec ran %d times, not only for the stick", len(*calls))
	}
}

func TestWithoutGsettingsItsConditionsFail(t *testing.T) {
	f, files, calls := conditionFixture(t)
	f.installed = []string{"present"}
	k := f.desk()
	sel, err := k.selection([]string{files["disk"]})
	if err != nil {
		t.Fatal(err)
	}
	if k.nemoCondition("gsettings org.test flag", map[string]string{}, "", sel) || len(*calls) != 0 {
		t.Errorf("calls %q", *calls)
	}
}

func TestAMountPointIsUnescaped(t *testing.T) {
	cases := map[string]string{`/media/my\040stick`: "/media/my stick", `/a\011b\\c`: "/a\tb\\\\c", `/x\0`: `/x\0`, `/plain`: "/plain"}
	for in, want := range cases {
		if got := unescapeMount(in); got != want {
			t.Errorf("%s: got %q", in, got)
		}
	}
}
