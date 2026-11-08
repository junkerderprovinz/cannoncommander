//go:build gtk3

package main

/*
#cgo pkg-config: gtk+-3.0
#include <gtk/gtk.h>
#include <stdlib.h>
#include <string.h>

enum { GNOME_FILES = 1, URI_LIST, KDE_CUT, TEXT };

typedef struct {
	char *gnome, *uris, *kde, *text;
} clip_files;

static void get_files(GtkClipboard *cb, GtkSelectionData *sel, guint info, gpointer data) {
	clip_files *f = data;
	const char *s = NULL;
	switch (info) {
	case GNOME_FILES: s = f->gnome; break;
	case URI_LIST: s = f->uris; break;
	case KDE_CUT: s = f->kde; break;
	default:
		gtk_selection_data_set_text(sel, f->text, -1);
		return;
	}
	gtk_selection_data_set(sel, gtk_selection_data_get_target(sel), 8, (const guchar *)s, strlen(s));
}

static void clear_files(GtkClipboard *cb, gpointer data) {
	clip_files *f = data;
	g_free(f->gnome);
	g_free(f->uris);
	g_free(f->kde);
	g_free(f->text);
	g_free(f);
}

static gboolean put_files(const char *gnome, const char *uris, const char *kde, const char *text) {
	static const GtkTargetEntry files[] = {
		{"x-special/gnome-copied-files", 0, GNOME_FILES},
		{"text/uri-list", 0, URI_LIST},
		{"application/x-kde-cutselection", 0, KDE_CUT},
	};
	GtkTargetList *list = gtk_target_list_new(files, G_N_ELEMENTS(files));
	gtk_target_list_add_text_targets(list, TEXT);
	gint n;
	GtkTargetEntry *targets = gtk_target_table_new_from_list(list, &n);
	clip_files *f = g_new0(clip_files, 1);
	f->gnome = g_strdup(gnome);
	f->uris = g_strdup(uris);
	f->kde = g_strdup(kde);
	f->text = g_strdup(text);
	GtkClipboard *cb = gtk_clipboard_get(GDK_SELECTION_CLIPBOARD);
	gboolean ok = gtk_clipboard_set_with_data(cb, targets, n, get_files, clear_files, f);
	if (ok) {
		gtk_clipboard_set_can_store(cb, NULL, 0);
	} else {
		clear_files(cb, f);
	}
	gtk_target_table_free(targets, n);
	gtk_target_list_unref(list);
	return ok;
}

// read_target returns what the clipboard holds as target, or NULL. The
// caller frees it.
static char *read_target(const char *target) {
	GtkClipboard *cb = gtk_clipboard_get(GDK_SELECTION_CLIPBOARD);
	GtkSelectionData *sel = gtk_clipboard_wait_for_contents(cb, gdk_atom_intern(target, FALSE));
	if (sel == NULL) {
		return NULL;
	}
	char *out = NULL;
	gint len = gtk_selection_data_get_length(sel);
	if (len >= 0) {
		out = g_strndup((const char *)gtk_selection_data_get_data(sel), len);
	}
	gtk_selection_data_free(sel);
	return out;
}

static void clear_clipboard(void) {
	gtk_clipboard_clear(gtk_clipboard_get(GDK_SELECTION_CLIPBOARD));
}
*/
import "C"

import (
	"errors"
	"unsafe"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/junkerderprovinz/cannoncommander/backend/pkg/desktop"
)

var _ desktop.Clipboard = (*shell)(nil)

// readTarget reads one format off the clipboard. GTK only works on its own
// thread, where waiting for the program that holds the clipboard keeps the
// window answering.
func readTarget(target string) (string, bool) {
	name := C.CString(target)
	defer C.free(unsafe.Pointer(name))
	var out *C.char
	application.InvokeSync(func() { out = C.read_target(name) })
	if out == nil {
		return "", false
	}
	defer C.g_free(C.gpointer(unsafe.Pointer(out)))
	return C.GoString(out), true
}

// ClipboardFiles reads the list GNOME, Cinnamon, Xfce and their kin share,
// and the URI list KDE marks a cut beside.
func (*shell) ClipboardFiles() ([]string, bool, error) {
	if raw, ok := readTarget("x-special/gnome-copied-files"); ok {
		paths, cut := parseGnomeFiles(raw)
		return paths, cut, nil
	}
	raw, ok := readTarget("text/uri-list")
	if !ok {
		return nil, false, nil
	}
	mark, _ := readTarget("application/x-kde-cutselection")
	return parseURIList(raw), mark == "1", nil
}

func (*shell) CopyFiles(paths []string, cut bool) error {
	gnome, uris, kde, text := clipFormats(paths, cut)
	cs := []*C.char{C.CString(gnome), C.CString(uris), C.CString(kde), C.CString(text)}
	defer func() {
		for _, c := range cs {
			C.free(unsafe.Pointer(c))
		}
	}()
	var ok C.gboolean
	application.InvokeSync(func() { ok = C.put_files(cs[0], cs[1], cs[2], cs[3]) })
	if ok == 0 {
		return errors.New("the clipboard did not take the files")
	}
	return nil
}

// ClearClipboard empties the clipboard while the app holds it. What another
// program holds stays, which that program clears itself after a move.
func (*shell) ClearClipboard() error {
	application.InvokeSync(func() { C.clear_clipboard() })
	return nil
}
