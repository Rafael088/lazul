#include "aura.h"

#include <gtk/gtk.h>
#include <gtk-layer-shell.h>
#include <webkit2/webkit2.h>

struct Aura {
  GtkWidget *ventana;
  GtkWidget *web;
  char *html;
  char *pendiente;
  double x, y; /* posición pendiente, 0..1 en el monitor */
  gboolean listo;
  guint fuente_crear;
  guint fuente_aplicar;
  GMutex mutex;
};

/* Aplica el último estado pendiente. Se ejecuta en el hilo de GTK. */
static gboolean aplicar_idle(gpointer datos) {
  Aura *aura = datos;
  g_mutex_lock(&aura->mutex);
  aura->fuente_aplicar = 0;
  char *js = aura->pendiente;
  aura->pendiente = NULL;
  double x = aura->x, y = aura->y;
  g_mutex_unlock(&aura->mutex);

  if (js != NULL && aura->web != NULL) {
    /* Margen para centrar la superficie en la mano, acotado al monitor; lo
       que no se pudo mover va como desplazamiento dentro de la página. */
    int ancho = 0, alto = 0;
    GdkDisplay *pantalla = gtk_widget_get_display(aura->ventana);
    GdkMonitor *monitor = NULL;
    GdkWindow *gw = gtk_widget_get_window(aura->ventana);
    if (gw != NULL) monitor = gdk_display_get_monitor_at_window(pantalla, gw);
    if (monitor == NULL) monitor = gdk_display_get_primary_monitor(pantalla);
    if (monitor == NULL) monitor = gdk_display_get_monitor(pantalla, 0);
    if (monitor != NULL) {
      GdkRectangle g;
      gdk_monitor_get_geometry(monitor, &g);
      ancho = g.width;
      alto = g.height;
    }
    int mitad = LAZUL_AURA_LADO / 2;
    int px = (int)(x * ancho), py = (int)(y * alto);
    int ml = CLAMP(px - mitad, 0, MAX(0, ancho - LAZUL_AURA_LADO));
    int mt = CLAMP(py - mitad, 0, MAX(0, alto - LAZUL_AURA_LADO));
    gtk_layer_set_margin(GTK_WINDOW(aura->ventana), GTK_LAYER_SHELL_EDGE_LEFT, ml);
    gtk_layer_set_margin(GTK_WINDOW(aura->ventana), GTK_LAYER_SHELL_EDGE_TOP, mt);
    char *llamada = g_strdup_printf("window.aura && window.aura(%s, %d, %d)", js, px - ml, py - mt);
    webkit_web_view_evaluate_javascript(WEBKIT_WEB_VIEW(aura->web), llamada, -1,
                                        NULL, NULL, NULL, NULL, NULL);
    g_free(llamada);
  }
  g_free(js);
  return G_SOURCE_REMOVE;
}

/* programa_aplicar: deja un solo idle pendiente con el último estado. */
static void programa_aplicar(Aura *aura) {
  g_mutex_lock(&aura->mutex);
  gboolean hace_falta = aura->listo && aura->pendiente != NULL &&
                        aura->fuente_aplicar == 0;
  if (hace_falta) {
    aura->fuente_aplicar = g_idle_add(aplicar_idle, aura);
  }
  g_mutex_unlock(&aura->mutex);
}

static void al_cargar(WebKitWebView *web, WebKitLoadEvent evento, gpointer datos) {
  (void)web;
  Aura *aura = datos;
  if (evento != WEBKIT_LOAD_FINISHED) {
    return;
  }
  g_mutex_lock(&aura->mutex);
  aura->listo = TRUE;
  g_mutex_unlock(&aura->mutex);
  programa_aplicar(aura);
}

/* El layer-shell sí respeta la región de entrada: vacía = clics pasan.
   GTK reescribe la región al realizar la ventana, así que se aplica al
   mapear (después de realize) y otra vez en un idle, que es cuando ya
   nadie la pisa. Es el mismo camino que usa waybar para su passthrough. */
static void aplicar_entrada_vacia(GtkWidget *widget) {
  cairo_region_t *vacia = cairo_region_create();
  gtk_widget_input_shape_combine_region(widget, vacia);
  cairo_region_destroy(vacia);
}

static gboolean entrada_idle(gpointer datos) {
  aplicar_entrada_vacia(GTK_WIDGET(datos));
  return G_SOURCE_REMOVE;
}

static void al_mapear(GtkWidget *widget, gpointer datos) {
  (void)datos;
  aplicar_entrada_vacia(widget);
  g_idle_add(entrada_idle, widget);
}

static gboolean crear_idle(gpointer datos) {
  Aura *aura = datos;
  g_mutex_lock(&aura->mutex);
  aura->fuente_crear = 0;
  g_mutex_unlock(&aura->mutex);

  /* Sin layer-shell (X11, compositores sin el protocolo) no hay forma de
     quedar encima sin robar foco: se apaga el aura y no se abre ventana. */
  if (!gtk_layer_is_supported()) {
    return G_SOURCE_REMOVE;
  }

  GtkWidget *ventana = gtk_window_new(GTK_WINDOW_TOPLEVEL);
  gtk_window_set_decorated(GTK_WINDOW(ventana), FALSE);
  gtk_window_set_resizable(GTK_WINDOW(ventana), FALSE);
  gtk_window_set_skip_taskbar_hint(GTK_WINDOW(ventana), TRUE);
  gtk_window_set_skip_pager_hint(GTK_WINDOW(ventana), TRUE);
  gtk_window_set_accept_focus(GTK_WINDOW(ventana), FALSE);
  gtk_window_set_focus_on_map(GTK_WINDOW(ventana), FALSE);
  gtk_widget_set_app_paintable(ventana, TRUE);

  GdkScreen *pantalla = gtk_widget_get_screen(ventana);
  GdkVisual *visual = gdk_screen_get_rgba_visual(pantalla);
  if (visual != NULL) {
    gtk_widget_set_visual(ventana, visual);
  }

  gtk_layer_init_for_window(GTK_WINDOW(ventana));
  gtk_layer_set_namespace(GTK_WINDOW(ventana), "lazul-aura");
  gtk_layer_set_layer(GTK_WINDOW(ventana), GTK_LAYER_SHELL_LAYER_OVERLAY);
  /* Superficie chica que se mueve con la mano (márgenes del layer-shell).
     Una a pantalla completa se repintaba entera en cada cuadro y, con
     WebKit en la NVIDIA, se copiaba a la Intel: 210 % de CPU (18-nov). */
  gtk_layer_set_anchor(GTK_WINDOW(ventana), GTK_LAYER_SHELL_EDGE_TOP, TRUE);
  gtk_layer_set_anchor(GTK_WINDOW(ventana), GTK_LAYER_SHELL_EDGE_LEFT, TRUE);
  gtk_widget_set_size_request(ventana, LAZUL_AURA_LADO, LAZUL_AURA_LADO);
  gtk_layer_set_exclusive_zone(GTK_WINDOW(ventana), -1);
  gtk_layer_set_keyboard_mode(GTK_WINDOW(ventana),
                              GTK_LAYER_SHELL_KEYBOARD_MODE_NONE);
  g_signal_connect(ventana, "map", G_CALLBACK(al_mapear), aura);

  GtkWidget *web = webkit_web_view_new();
  GdkRGBA transparente = {0, 0, 0, 0};
  webkit_web_view_set_background_color(WEBKIT_WEB_VIEW(web), &transparente);
  g_signal_connect(web, "load-changed", G_CALLBACK(al_cargar), aura);

  gtk_container_add(GTK_CONTAINER(ventana), web);
  aura->ventana = ventana;
  aura->web = web;
  gtk_widget_show_all(ventana);
  aplicar_entrada_vacia(ventana);

  if (aura->html != NULL) {
    webkit_web_view_load_html(WEBKIT_WEB_VIEW(web), aura->html, NULL);
  }
  return G_SOURCE_REMOVE;
}

Aura *lazul_aura_crear(const char *html) {
  Aura *aura = g_new0(Aura, 1);
  g_mutex_init(&aura->mutex);
  aura->html = g_strdup(html);
  aura->fuente_crear = g_idle_add(crear_idle, aura);
  return aura;
}

void lazul_aura_actualizar(Aura *aura, const char *json, double x, double y) {
  if (aura == NULL) {
    return;
  }
  char *js = g_strdup(json);
  g_mutex_lock(&aura->mutex);
  g_free(aura->pendiente);
  aura->pendiente = js;
  aura->x = x;
  aura->y = y;
  g_mutex_unlock(&aura->mutex);
  programa_aplicar(aura);
}

static gboolean cerrar_idle(gpointer datos) {
  Aura *aura = datos;
  g_mutex_lock(&aura->mutex);
  if (aura->fuente_crear != 0) {
    g_source_remove(aura->fuente_crear);
    aura->fuente_crear = 0;
  }
  if (aura->fuente_aplicar != 0) {
    g_source_remove(aura->fuente_aplicar);
    aura->fuente_aplicar = 0;
  }
  g_mutex_unlock(&aura->mutex);

  if (aura->ventana != NULL) {
    gtk_widget_destroy(aura->ventana);
  }
  g_mutex_lock(&aura->mutex);
  g_free(aura->pendiente);
  g_mutex_unlock(&aura->mutex);
  g_mutex_clear(&aura->mutex);
  g_free(aura->html);
  g_free(aura);
  return G_SOURCE_REMOVE;
}

void lazul_aura_cerrar(Aura *aura) {
  if (aura == NULL) {
    return;
  }
  /* Destruir en el hilo de GTK: Cerrar puede llamarse desde otro hilo. */
  g_idle_add(cerrar_idle, aura);
}
