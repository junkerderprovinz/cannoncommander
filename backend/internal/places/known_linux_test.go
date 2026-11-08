package places

import (
	"slices"
	"testing"
)

func TestUserDirsFollowsTheDesktopsOwnNames(t *testing.T) {
	data := `# written by xdg-user-dirs-update
XDG_DESKTOP_DIR="$HOME/Schreibtisch"
XDG_DOWNLOAD_DIR="$HOME/Downloads"
XDG_PICTURES_DIR="/data/Bilder"
XDG_MUSIC_DIR="$HOME/"
XDG_VIDEOS_DIR="Videos"
`
	got := userDirs(data, "/home/jdp")
	want := []folder{
		{"home", "/home/jdp"},
		{"desktop", "/home/jdp/Schreibtisch"},
		{"documents", "/home/jdp/Documents"},
		{"downloads", "/home/jdp/Downloads"},
		{"pictures", "/data/Bilder"},
		{"videos", "/home/jdp/Videos"},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("got %+v", got)
	}
}
