package i18n

import (
	"fmt"
	"net/http"
	"strings"
	"time"
)

const (
	LangES = "es"
	LangEN = "en"
)

var defaultLang = LangES

var translations = map[string]map[string]string{
	LangES: {
		"app.title":                            "simplefs - Explorador de Archivos",
		"header.search_placeholder":            "Buscar archivos y carpetas...",
		"header.theme_toggle":                  "Cambiar Tema (Claro / Oscuro)",
		"header.new_folder":                    "Carpeta",
		"header.paste":                         "Pegar",
		"header.lang_switch":                   "Cambiar idioma",
		"nav.home":                             "Inicio",
		"breadcrumb.home":                      "Inicio",
		"breadcrumb.aria":                      "Ruta de navegación",
		"folders.title":                        "Carpetas (%d)",
		"folders.item_count":                   "%d elementos",
		"folders.items":                        "%d elementos",
		"folders.open_aria":                    "Abrir carpeta %s",
		"folders.delete_title":                 "Eliminar carpeta",
		"folders.delete_confirm":               "¿Seguro que deseas eliminar la carpeta '%s' y todo su contenido?",
		"files.title":                          "Archivos (%d)",
		"sort.label":                           "Ordenar por",
		"sort.name_asc":                        "Nombre (A-Z)",
		"sort.name_desc":                       "Nombre (Z-A)",
		"sort.modified_desc":                   "Modificado (Más reciente)",
		"sort.modified_asc":                    "Modificado (Más antiguo)",
		"sort.created_desc":                    "Creado (Más reciente)",
		"sort.created_asc":                     "Creado (Más antiguo)",
		"sort.size_desc":                       "Tamaño (Mayor primero)",
		"sort.size_asc":                        "Tamaño (Menor primero)",
		"files.sort_by":                        "Ordenar por",
		"files.sort.name_asc":                  "Nombre (A-Z)",
		"files.sort.name_desc":                 "Nombre (Z-A)",
		"files.sort.modified_desc":             "Modificado (Más reciente)",
		"files.sort.modified_asc":              "Modificado (Más antiguo)",
		"files.sort.created_desc":              "Creado (Más reciente)",
		"files.sort.created_asc":               "Creado (Más antiguo)",
		"files.sort.size_desc":                 "Tamaño (Mayor primero)",
		"files.sort.size_asc":                  "Tamaño (Menor primero)",
		"view.list":                            "Vista en Lista",
		"view.grid":                            "Vista en Cuadrícula",
		"files.view.list":                      "Vista en Lista",
		"files.view.grid":                      "Vista en Cuadrícula",
		"table.name":                           "Nombre",
		"table.created":                        "Creado",
		"table.modified":                       "Modificado",
		"table.size":                           "Tamaño",
		"table.actions":                        "Acciones",
		"files.table.name":                     "Nombre",
		"files.table.created":                  "Creado",
		"files.table.modified":                 "Modificado",
		"files.table.size":                     "Tamaño",
		"files.table.actions":                  "Acciones",
		"empty.search_title":                   "No se encontraron resultados para \"%s\"",
		"empty.search_desc":                    "Intenta buscar con otros términos o limpia la barra de búsqueda.",
		"empty.folder_title":                   "Esta carpeta está vacía",
		"empty.folder_desc":                    "Usa el botón flotante (+) o arrastra archivos aquí para comenzar a subir contenido.",
		"empty.no_files":                       "No hay archivos en este nivel.",
		"files.empty.search_title":             "No se encontraron resultados para \"%s\"",
		"files.empty.search_desc":              "Intenta buscar con otros términos o limpia la barra de búsqueda.",
		"files.empty.folder_title":             "Esta carpeta está vacía",
		"files.empty.folder_desc":              "Usa el botón flotante (+) o arrastra archivos aquí para comenzar a subir contenido.",
		"files.empty.no_files":                 "No hay archivos en este nivel.",
		"files.actions.preview":                "Previsualizar %s",
		"files.actions.details":                "Detalles",
		"files.actions.download":               "Descargar",
		"files.actions.delete":                 "Eliminar",
		"files.actions.delete_confirm":         "¿Seguro que deseas eliminar '%s'?",
		"files.delete_confirm":                 "¿Seguro que deseas eliminar '%s'?",
		"details.title":                        "Detalles del Archivo",
		"details.size":                         "Tamaño",
		"details.created":                      "Creado",
		"details.modified":                     "Modificado",
		"details.location":                     "Ubicación",
		"details.close":                        "Cerrar",
		"details.download":                     "Descargar",
		"preview.back":                         "Volver",
		"preview.lines":                        "Líneas: %d",
		"preview.copy":                         "Copiar",
		"preview.copied":                       "¡Copiado!",
		"preview.copy_content":                 "Copiar contenido",
		"preview.print":                        "Imprimir",
		"preview.download":                     "Descargar",
		"preview.close":                        "Cerrar",
		"preview.video_unsupported":            "Tu navegador no soporta reproducción de video.",
		"preview.audio_unsupported":            "Tu navegador no soporta reproducción de audio.",
		"preview.zoom_out":                     "Reducir zoom",
		"preview.zoom_in":                      "Aumentar zoom",
		"preview.fit_screen":                   "Ajustar a pantalla",
		"preview.rotate":                       "Rotar 90°",
		"preview.unsupported":                  "Este tipo de archivo no cuenta con vista previa en línea.",
		"preview.download_to_view":             "Descargar Archivo",
		"modal.folder.title":                   "Nueva Carpeta",
		"modal.folder.placeholder":             "Nombre de la carpeta...",
		"modal.folder.cancel":                  "Cancelar",
		"modal.folder.create":                  "Crear Carpeta",
		"modal.clipboard.title_image":          "Imagen del Portapapeles",
		"modal.clipboard.title_text":           "Guardar desde Portapapeles",
		"modal.clipboard.filename":             "Nombre del archivo",
		"modal.clipboard.filename_placeholder": "ej: notas.txt o captura.png",
		"modal.clipboard.content":              "Contenido",
		"modal.clipboard.paste_btn":            "Pegar desde portapapeles",
		"modal.clipboard.save_btn":             "Guardar Archivo",
		"fab.actions":                          "Acciones",
		"fab.upload":                           "Subir Archivos",
		"fab.paste":                            "Pegar Portapapeles",
		"fab.new_folder":                       "Nueva Carpeta",
		"drag.title":                           "Suelta los archivos aquí para subirlos",
		"drag.desc":                            "Se guardarán en la carpeta actual",
		"toast.copied":                         "¡Copiado al portapapeles!",
		"toast.uploading":                      "Subiendo archivo...",
	},
	LangEN: {
		"app.title":                            "simplefs - File Manager",
		"header.search_placeholder":            "Search files and folders...",
		"header.theme_toggle":                  "Toggle Theme (Light / Dark)",
		"header.new_folder":                    "Folder",
		"header.paste":                         "Paste",
		"header.lang_switch":                   "Switch language",
		"nav.home":                             "Home",
		"breadcrumb.home":                      "Home",
		"breadcrumb.aria":                      "Navigation breadcrumbs",
		"folders.title":                        "Folders (%d)",
		"folders.item_count":                   "%d items",
		"folders.items":                        "%d items",
		"folders.open_aria":                    "Open folder %s",
		"folders.delete_title":                 "Delete folder",
		"folders.delete_confirm":               "Are you sure you want to delete folder '%s' and all its contents?",
		"files.title":                          "Files (%d)",
		"sort.label":                           "Sort by",
		"sort.name_asc":                        "Name (A-Z)",
		"sort.name_desc":                       "Name (Z-A)",
		"sort.modified_desc":                   "Modified (Newest)",
		"sort.modified_asc":                    "Modified (Oldest)",
		"sort.created_desc":                    "Created (Newest)",
		"sort.created_asc":                     "Created (Oldest)",
		"sort.size_desc":                       "Size (Largest first)",
		"sort.size_asc":                        "Size (Smallest first)",
		"files.sort_by":                        "Sort by",
		"files.sort.name_asc":                  "Name (A-Z)",
		"files.sort.name_desc":                 "Name (Z-A)",
		"files.sort.modified_desc":             "Modified (Newest)",
		"files.sort.modified_asc":              "Modified (Oldest)",
		"files.sort.created_desc":              "Created (Newest)",
		"files.sort.created_asc":               "Created (Oldest)",
		"files.sort.size_desc":                 "Size (Largest first)",
		"files.sort.size_asc":                  "Size (Smallest first)",
		"view.list":                            "List View",
		"view.grid":                            "Grid View",
		"files.view.list":                      "List View",
		"files.view.grid":                      "Grid View",
		"table.name":                           "Name",
		"table.created":                        "Created",
		"table.modified":                       "Modified",
		"table.size":                           "Size",
		"table.actions":                        "Actions",
		"files.table.name":                     "Name",
		"files.table.created":                  "Created",
		"files.table.modified":                 "Modified",
		"files.table.size":                     "Size",
		"files.table.actions":                  "Actions",
		"empty.search_title":                   "No results found for \"%s\"",
		"empty.search_desc":                    "Try searching with other terms or clear the search bar.",
		"empty.folder_title":                   "This folder is empty",
		"empty.folder_desc":                    "Use the floating button (+) or drag files here to start uploading content.",
		"empty.no_files":                       "No files in this folder level.",
		"files.empty.search_title":             "No results found for \"%s\"",
		"files.empty.search_desc":              "Try searching with other terms or clear the search bar.",
		"files.empty.folder_title":             "This folder is empty",
		"files.empty.folder_desc":              "Use the floating button (+) or drag files here to start uploading content.",
		"files.empty.no_files":                 "No files in this folder level.",
		"files.actions.preview":                "Preview %s",
		"files.actions.details":                "Details",
		"files.actions.download":               "Download",
		"files.actions.delete":                 "Delete",
		"files.actions.delete_confirm":         "Are you sure you want to delete '%s'?",
		"files.delete_confirm":                 "Are you sure you want to delete '%s'?",
		"details.title":                        "File Details",
		"details.size":                         "Size",
		"details.created":                      "Created",
		"details.modified":                     "Modified",
		"details.location":                     "Location",
		"details.close":                        "Close",
		"details.download":                     "Download",
		"preview.back":                         "Back",
		"preview.lines":                        "Lines: %d",
		"preview.copy":                         "Copy",
		"preview.copied":                       "Copied!",
		"preview.copy_content":                 "Copy content",
		"preview.print":                        "Print",
		"preview.download":                     "Download",
		"preview.close":                        "Close",
		"preview.video_unsupported":            "Your browser does not support video playback.",
		"preview.audio_unsupported":            "Your browser does not support audio playback.",
		"preview.zoom_out":                     "Zoom out",
		"preview.zoom_in":                      "Zoom in",
		"preview.fit_screen":                   "Fit to screen",
		"preview.rotate":                       "Rotate 90°",
		"preview.unsupported":                  "This file type does not support online preview.",
		"preview.download_to_view":             "Download File",
		"modal.folder.title":                   "New Folder",
		"modal.folder.placeholder":             "Folder name...",
		"modal.folder.cancel":                  "Cancel",
		"modal.folder.create":                  "Create Folder",
		"modal.clipboard.title_image":          "Clipboard Image",
		"modal.clipboard.title_text":           "Save from Clipboard",
		"modal.clipboard.filename":             "File name",
		"modal.clipboard.filename_placeholder": "e.g.: notes.txt or screenshot.png",
		"modal.clipboard.content":              "Content",
		"modal.clipboard.paste_btn":            "Paste from clipboard",
		"modal.clipboard.save_btn":             "Save File",
		"fab.actions":                          "Actions",
		"fab.upload":                           "Upload Files",
		"fab.paste":                            "Paste Clipboard",
		"fab.new_folder":                       "New Folder",
		"drag.title":                           "Drop files here to upload",
		"drag.desc":                            "They will be saved in the current folder",
		"toast.copied":                         "Copied to clipboard!",
		"toast.uploading":                      "Uploading file...",
	},
}

// NormalizeLang returns "es" or "en", defaulting to "es"
func NormalizeLang(lang string) string {
	lang = strings.ToLower(strings.TrimSpace(lang))
	if strings.HasPrefix(lang, LangEN) {
		return LangEN
	}
	if strings.HasPrefix(lang, LangES) {
		return LangES
	}
	return defaultLang
}

// ResolveLang extracts the preferred language from query params, cookies or headers
func ResolveLang(r *http.Request) string {
	if queryLang := r.URL.Query().Get("lang"); queryLang != "" {
		return NormalizeLang(queryLang)
	}

	if formLang := r.FormValue("lang"); formLang != "" {
		return NormalizeLang(formLang)
	}

	if cookie, err := r.Cookie("lang"); err == nil && cookie.Value != "" {
		return NormalizeLang(cookie.Value)
	}

	acceptLang := r.Header.Get("Accept-Language")
	if acceptLang != "" {
		parts := strings.Split(acceptLang, ",")
		for _, part := range parts {
			tag := strings.TrimSpace(strings.Split(part, ";")[0])
			if strings.HasPrefix(strings.ToLower(tag), "es") {
				return LangES
			}
			if strings.HasPrefix(strings.ToLower(tag), "en") {
				return LangEN
			}
		}
	}

	return defaultLang
}

// T translates a key into the given language, optionally formatting with args
func T(lang string, key string, args ...any) string {
	normLang := NormalizeLang(lang)
	dict, ok := translations[normLang]
	if !ok {
		dict = translations[defaultLang]
	}

	templateStr, exists := dict[key]
	if !exists {
		if defaultDict, defOk := translations[defaultLang]; defOk {
			if defStr, defExists := defaultDict[key]; defExists {
				templateStr = defStr
			} else {
				templateStr = key
			}
		} else {
			templateStr = key
		}
	}

	if len(args) > 0 {
		if strings.Contains(templateStr, "%") {
			return fmt.Sprintf(templateStr, args...)
		}
	}
	return templateStr
}

var monthsES = []string{
	"Ene", "Feb", "Mar", "Abr", "May", "Jun", "Jul", "Ago", "Sep", "Oct", "Nov", "Dic",
}

var monthsEN = []string{
	"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec",
}

// FormatDate formats a time.Time object according to the selected language
func FormatDate(t time.Time, lang string) string {
	normLang := NormalizeLang(lang)
	monthIdx := int(t.Month()) - 1
	if monthIdx < 0 || monthIdx >= 12 {
		monthIdx = 0
	}

	if normLang == LangES {
		return fmt.Sprintf("%02d %s %d", t.Day(), monthsES[monthIdx], t.Year())
	}
	return fmt.Sprintf("%02d %s %d", t.Day(), monthsEN[monthIdx], t.Year())
}

// FormatDateTime formats date and time
func FormatDateTime(t time.Time, lang string) string {
	datePart := FormatDate(t, lang)
	return fmt.Sprintf("%s, %02d:%02d", datePart, t.Hour(), t.Minute())
}
