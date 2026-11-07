//go:build linux

package main

/*
#cgo pkg-config: gtk+-3.0 webkit2gtk-4.1
#include <gtk/gtk.h>
#include <webkit2/webkit2.h>

// Wails v2 no atiende «permission-request» de WebKitGTK, así que getUserMedia
// sale con NotAllowedError. Este parche busca el WebKitWebView de la ventana,
// enciende el media stream y concede solo los permisos de cámara/micrófono.
static gboolean permiso(WebKitWebView *v, WebKitPermissionRequest *r, gpointer d) {
	if (WEBKIT_IS_USER_MEDIA_PERMISSION_REQUEST(r) || WEBKIT_IS_DEVICE_INFO_PERMISSION_REQUEST(r)) {
		webkit_permission_request_allow(r);
		return TRUE;
	}
	return FALSE;
}

static WebKitWebView *buscar(GtkWidget *w) {
	if (WEBKIT_IS_WEB_VIEW(w)) return WEBKIT_WEB_VIEW(w);
	if (!GTK_IS_CONTAINER(w)) return NULL;
	GList *hijos = gtk_container_get_children(GTK_CONTAINER(w));
	WebKitWebView *res = NULL;
	for (GList *l = hijos; l && !res; l = l->next) res = buscar(GTK_WIDGET(l->data));
	g_list_free(hijos);
	return res;
}

static volatile int parcheado = 0;

static gboolean parchear(gpointer d) {
	GList *ventanas = gtk_window_list_toplevels();
	for (GList *l = ventanas; l; l = l->next) {
		WebKitWebView *v = buscar(GTK_WIDGET(l->data));
		if (v) {
			WebKitSettings *s = webkit_web_view_get_settings(v);
			webkit_settings_set_enable_media_stream(s, TRUE);
			g_signal_connect(v, "permission-request", G_CALLBACK(permiso), NULL);
			parcheado = 1;
			break;
		}
	}
	g_list_free(ventanas);
	return parcheado ? G_SOURCE_REMOVE : G_SOURCE_CONTINUE;
}

static void programar(void) { g_timeout_add(20, parchear, NULL); }
static int listo(void) { return parcheado; }
*/
import "C"

import "time"

// parcheWebview deja programado el parche en el hilo de GTK (corre cuando arranca el bucle).
func parcheWebview() { C.programar() }

// parcheListo espera a que el parche se haya aplicado (máx. 5 s).
func parcheListo() {
	for i := 0; i < 500 && C.listo() == 0; i++ {
		time.Sleep(10 * time.Millisecond)
	}
}
