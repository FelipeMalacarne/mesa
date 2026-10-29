//go:build desktop

package main

/*
#cgo linux pkg-config: gtk+-3.0
#cgo !webkit2_41 pkg-config: webkit2gtk-4.0
#cgo webkit2_41 pkg-config: webkit2gtk-4.1
#include <gtk/gtk.h>
#include <webkit2/webkit2.h>

static WebKitWebView *mesaFindWebView(GtkWidget *widget) {
	if (WEBKIT_IS_WEB_VIEW(widget)) {
		return WEBKIT_WEB_VIEW(widget);
	}
	if (!GTK_IS_CONTAINER(widget)) {
		return NULL;
	}
	GList *children = gtk_container_get_children(GTK_CONTAINER(widget));
	WebKitWebView *found = NULL;
	for (GList *l = children; l != NULL && found == NULL; l = l->next) {
		found = mesaFindWebView(GTK_WIDGET(l->data));
	}
	g_list_free(children);
	return found;
}

// The level travels in the pointer as thousandths, so nothing is allocated.
static gboolean mesaApplyZoom(gpointer data) {
	double level = GPOINTER_TO_INT(data) / 1000.0;
	GList *windows = gtk_window_list_toplevels();
	for (GList *l = windows; l != NULL; l = l->next) {
		WebKitWebView *webview = mesaFindWebView(GTK_WIDGET(l->data));
		if (webview != NULL) {
			webkit_web_view_set_zoom_level(webview, level);
		}
	}
	g_list_free(windows);
	return G_SOURCE_REMOVE;
}

static void mesaSetZoom(double level) {
	g_idle_add(mesaApplyZoom, GINT_TO_POINTER((int)(level * 1000 + 0.5)));
}
*/
import "C"

// nativeZoom says the View menu carries Zoom In / Zoom Out / Actual Size.
const nativeZoom = true

func setZoom(level float64) { C.mesaSetZoom(C.double(level)) }
