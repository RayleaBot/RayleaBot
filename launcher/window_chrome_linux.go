//go:build linux && gtk3

package main

/*
#cgo pkg-config: gtk+-3.0
#include <gtk/gtk.h>
#include <gdk/gdkwayland.h>
#include <stdint.h>

extern void launcherWindowStateChanged(uintptr_t handle, gboolean maximised);
extern void releaseLauncherWindowState(uintptr_t handle);

static gboolean launcherWindowStateEvent(GtkWidget *widget, GdkEventWindowState *event, gpointer data) {
	if (event->changed_mask & GDK_WINDOW_STATE_MAXIMIZED) {
		launcherWindowStateChanged((uintptr_t)data, (event->new_window_state & GDK_WINDOW_STATE_MAXIMIZED) != 0);
	}
	return FALSE;
}

static void launcherWindowStateDestroyed(gpointer data, GClosure *closure) {
	releaseLauncherWindowState((uintptr_t)data);
}

static void observeLauncherWindowState(void *window, uintptr_t handle) {
	g_signal_connect_data(window, "window-state-event", G_CALLBACK(launcherWindowStateEvent),
		(gpointer)handle, launcherWindowStateDestroyed, 0);
}

static char *setLauncherWindowIcon(void *handle, const void *data, gsize size) {
	GError *error = NULL;
	GInputStream *stream = g_memory_input_stream_new_from_data(data, size, NULL);
	// GTK silently discards oversized _NET_WM_ICON data on X11.
	GdkPixbuf *icon = gdk_pixbuf_new_from_stream_at_scale(stream, 256, 256, TRUE, NULL, &error);
	g_object_unref(stream);
	if (icon != NULL) {
		gtk_window_set_icon(GTK_WINDOW(handle), icon);
		g_object_unref(icon);
	}
	if (error != NULL) {
		char *message = g_strdup(error->message);
		g_error_free(error);
		return message;
	}
	return NULL;
}

static void prepareLauncherWaylandWindow(void *handle) {
	GtkWidget *widget = GTK_WIDGET(handle);
	if (!GDK_IS_WAYLAND_DISPLAY(gtk_widget_get_display(widget))) {
		return;
	}

	// An explicit titlebar requests client-side decorations on Wayland.
	// Keep it hidden because the renderer already draws the titlebar.
	GtkWidget *titlebar = gtk_box_new(GTK_ORIENTATION_HORIZONTAL, 0);
	gtk_widget_set_no_show_all(titlebar, TRUE);
	gtk_window_set_titlebar(GTK_WINDOW(widget), titlebar);
}
*/
import "C"

import (
	"log/slog"
	"runtime/cgo"
	"unsafe"

	"github.com/wailsapp/wails/v3/pkg/application"
)

func prepareLauncherWindow(window *application.WebviewWindow) {
	application.InvokeSync(func() {
		if native := window.NativeWindow(); native != nil {
			// Wails beta.9's GTK3 backend does not emit the common maximise events.
			C.observeLauncherWindowState(native, C.uintptr_t(cgo.NewHandle(window)))
			// Use a bounded native icon while retaining the full-resolution desktop asset.
			if message := C.setLauncherWindowIcon(native, unsafe.Pointer(&appIcon[0]), C.gsize(len(appIcon))); message != nil {
				slog.Warn("无法设置启动器窗口图标", "error", C.GoString(message))
				C.g_free(C.gpointer(message))
			}
			C.prepareLauncherWaylandWindow(native)
		}
	})
}

//export launcherWindowStateChanged
func launcherWindowStateChanged(handle C.uintptr_t, maximised C.gboolean) {
	window := cgo.Handle(handle).Value().(*application.WebviewWindow)
	window.EmitEvent("launcher:maximized-change", maximised != 0)
}

//export releaseLauncherWindowState
func releaseLauncherWindowState(handle C.uintptr_t) {
	cgo.Handle(handle).Delete()
}
