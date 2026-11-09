//go:build gtk3

#include <gtk/gtk.h>
#include "_cgo_export.h"

typedef struct {
	gchar **uris;
	gulong give, end;
} drag;

static void give_uris(GtkWidget *w, GdkDragContext *ctx, GtkSelectionData *sel, guint info, guint time, gpointer data) {
	gtk_selection_data_set_uris(sel, ((drag *)data)->uris);
}

static void forget(GtkWidget *w, drag *d) {
	g_signal_handler_disconnect(w, d->give);
	g_signal_handler_disconnect(w, d->end);
	g_strfreev(d->uris);
	g_free(d);
}

static void end_drag(GtkWidget *w, GdkDragContext *ctx, gpointer data) {
	forget(w, data);
	dragEnded();
}

// drag_files starts a drag of uris from window at the pointer, which must
// still be down. The URI targets include the file transfer portal's formats
// where a portal runs, and gtk_selection_data_set_uris registers the files
// with it when a program asks for them.
int drag_files(void *window, char **uris) {
	GtkWidget *w = window;
	drag *d = g_new0(drag, 1);
	d->uris = g_strdupv(uris);
	d->give = g_signal_connect(w, "drag-data-get", G_CALLBACK(give_uris), d);
	d->end = g_signal_connect(w, "drag-end", G_CALLBACK(end_drag), d);
	GtkTargetList *targets = gtk_target_list_new(NULL, 0);
	gtk_target_list_add_uri_targets(targets, 0);
	GdkDragContext *ctx = gtk_drag_begin_with_coordinates(w, targets, GDK_ACTION_COPY | GDK_ACTION_MOVE | GDK_ACTION_LINK, 1, NULL, -1, -1);
	gtk_target_list_unref(targets);
	if (ctx == NULL) {
		forget(w, d);
		return 0;
	}
	return 1;
}
