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
		"a11y.skip":                            "Saltar al contenido",
		"app.title":                            "simplefs - Explorador de Archivos",
		"auth.logout":                          "Cerrar sesión",
		"auth.read_only":                       "Acceso de solo lectura",
		"banner.desc":                          "Los cambios se reflejan automáticamente; arrastra archivos aquí para subirlos.",
		"banner.items_count":                   "%d elementos",
		"banner.title":                         "Explorador de Archivos",
		"brand.tagline":                        "Intercambio de archivos",
		"breadcrumb.aria":                      "Ruta de navegación",
		"breadcrumb.home":                      "Inicio",
		"clipboard.image_prefix":               "captura_",
		"clipboard.preview":                    "Vista previa",
		"clipboard.preview_placeholder":        "// La vista previa aparecerá aquí al pegar contenido...",
		"clipboard.stats_image":                "Dimensiones: %d x %d px | %s | %s KB",
		"clipboard.stats_text":                 "Líneas: %d | Caracteres: %d | Tamaño: %d B",
		"connect.copy":                         "Copiar",
		"connect.desc":                         "Cualquier herramienta que hable HTTP puede subir y bajar archivos con la API /api/v1 usando el token de acceso.",
		"connect.footer":                       "Con un token de solo lectura el agente puede descargar pero no escribir. Añade ?overwrite=true al PUT para reemplazar (la versión anterior va a la papelera).",
		"connect.step_download":                "3. Descargar un archivo",
		"connect.step_list":                    "4. Listar una carpeta o los cambios recientes (JSON)",
		"connect.step_token":                   "1. Variables (el token se obtiene con «simplefs token»)",
		"connect.step_upload":                  "2. Enviar un archivo (crea las carpetas que falten)",
		"connect.title":                        "Conectar un agente o script",
		"details.close":                        "Cerrar",
		"details.copy_link":                    "Copiar enlace",
		"details.download":                     "Descargar",
		"details.location":                     "Ubicación",
		"details.modified":                     "Modificado",
		"details.size":                         "Tamaño",
		"drag.desc":                            "Se guardarán en la carpeta actual",
		"drag.title":                           "Suelta los archivos aquí para subirlos",
		"empty.folder_desc":                    "Arrastra archivos aquí o usa el botón (+) para subir contenido.",
		"empty.folder_desc_readonly":           "Todavía no hay archivos en esta carpeta.",
		"empty.folder_title":                   "Esta carpeta está vacía",
		"empty.not_found_back_home":            "Volver al inicio",
		"empty.not_found_desc":                 "La carpeta '%s' no existe o fue eliminada.",
		"empty.not_found_title":                "Carpeta no encontrada",
		"empty.search_desc":                    "Prueba con otros términos o limpia la búsqueda.",
		"empty.search_title":                   "No se encontraron resultados para \"%s\"",
		"error.busy":                           "Hay demasiadas descargas ZIP en curso; inténtalo en unos segundos.",
		"error.cannot_delete_root":             "No se puede eliminar la carpeta raíz.",
		"error.conflict":                       "Ya existe un archivo o carpeta con ese nombre.",
		"error.forbidden":                      "Acceso denegado.",
		"error.generic":                        "No se pudo completar la operación.",
		"error.invalid_base64":                 "Contenido base64 no válido.",
		"error.invalid_name":                   "Nombre no válido.",
		"error.network":                        "No hay conexión con el servidor.",
		"error.not_found":                      "El archivo o carpeta no existe.",
		"error.too_large":                      "El archivo supera el tamaño máximo permitido.",
		"error.upload_invalid":                 "La subida no es válida.",
		"error.zip":                            "ZIP rechazado",
		"fab.actions":                          "Acciones",
		"fab.new_folder":                       "Nueva Carpeta",
		"fab.paste":                            "Pegar Portapapeles",
		"fab.upload":                           "Subir Archivos",
		"fab.upload_zip":                       "Subir ZIP (descomprimir)",
		"files.actions.copy_curl":              "Copiar comando curl",
		"files.actions.delete":                 "Eliminar",
		"files.actions.details":                "Detalles",
		"files.actions.download":               "Descargar",
		"files.actions.extract_confirm":        "¿Seguro que deseas descomprimir '%s' en una carpeta?",
		"files.actions.extract_zip":            "Descomprimir ZIP",
		"files.actions.preview":                "Previsualizar %s",
		"files.delete_confirm":                 "¿Seguro que deseas eliminar '%s'?",
		"files.heading":                        "Archivos",
		"files.table.actions":                  "Acciones",
		"files.table.location":                 "Ubicación",
		"files.table.modified":                 "Modificado",
		"files.table.name":                     "Nombre",
		"files.table.size":                     "Tamaño",
		"filter.all":                           "Todos",
		"filter.code":                          "Código",
		"filter.docs":                          "Documentos",
		"filter.label":                         "Filtrar por tipo",
		"filter.media":                         "Multimedia",
		"folders.actions.download_zip":         "Descargar carpeta '%s' como ZIP",
		"folders.delete_confirm":               "¿Seguro que deseas eliminar la carpeta '%s' y todo su contenido?",
		"folders.download_zip":                 "Descargar como ZIP",
		"folders.heading":                      "Carpetas",
		"folders.info":                         "Información de la carpeta",
		"folders.item_count":                   "%d elementos",
		"folders.open_aria":                    "Abrir carpeta %s",
		"folders.title":                        "Carpetas (%d)",
		"header.lang_switch":                   "Cambiar idioma",
		"header.new_folder":                    "Carpeta",
		"header.paste":                         "Pegar",
		"header.search_placeholder":            "Buscar archivos y carpetas...",
		"header.theme_toggle":                  "Cambiar tema (claro / oscuro)",
		"login.help":                           "El token aparece en el log del primer arranque o con «simplefs token» (docker exec simplefs /simplefs token).",
		"login.invalid":                        "Token incorrecto.",
		"login.submit":                         "Entrar",
		"login.subtitle":                       "Introduce el token de acceso para continuar.",
		"login.title":                          "Iniciar sesión",
		"login.token":                          "Token de acceso",
		"login.too_many":                       "Demasiados intentos fallidos. Espera unos minutos.",
		"modal.clipboard.content":              "Contenido",
		"modal.clipboard.filename":             "Nombre del archivo",
		"modal.clipboard.filename_placeholder": "ej: notas.txt o captura.png",
		"modal.clipboard.paste_btn":            "Pegar desde portapapeles",
		"modal.clipboard.save_btn":             "Guardar Archivo",
		"modal.clipboard.title_image":          "Imagen del Portapapeles",
		"modal.clipboard.title_text":           "Guardar desde Portapapeles",
		"modal.folder.cancel":                  "Cancelar",
		"modal.folder.create":                  "Crear Carpeta",
		"modal.folder.label":                   "Nombre",
		"modal.folder.placeholder":             "Nombre de la carpeta...",
		"modal.folder.title":                   "Nueva Carpeta",
		"nav.all_files":                        "Todos los archivos",
		"nav.close_menu":                       "Cerrar navegación",
		"nav.connect":                          "Conectar agente",
		"nav.navigation":                       "Navegación",
		"nav.open_menu":                        "Abrir navegación",
		"nav.recents":                          "Recientes",
		"nav.trash":                            "Papelera",
		"preview.audio_unsupported":            "Tu navegador no soporta reproducción de audio.",
		"preview.back":                         "Volver",
		"preview.copy":                         "Copiar",
		"preview.download_to_view":             "Descargar Archivo",
		"preview.fit_screen":                   "Ajustar a pantalla",
		"preview.lines":                        "Líneas: %d",
		"preview.print":                        "Imprimir",
		"preview.read_error":                   "[Error al leer el archivo]",
		"preview.rotate":                       "Rotar 90°",
		"preview.too_large":                    "[Archivo demasiado grande para vista previa en texto; descárgalo para verlo]",
		"preview.unsupported":                  "Este tipo de archivo no cuenta con vista previa en línea.",
		"preview.video_unsupported":            "Tu navegador no soporta reproducción de video.",
		"preview.zoom_in":                      "Aumentar zoom",
		"preview.zoom_out":                     "Reducir zoom",
		"recent.desc":                          "Los últimos archivos modificados en todo el almacenamiento.",
		"recent.empty":                         "Aún no hay archivos",
		"recent.empty_desc":                    "Los archivos que subas o recibas aparecerán aquí.",
		"recent.title":                         "Recientes",
		"search.clear":                         "Limpiar búsqueda",
		"search.results_for":                   "Resultados para «%s» (incluye subcarpetas)",
		"search.truncated":                     "Se muestran solo los primeros resultados; refina la búsqueda para ver más.",
		"sort.label":                           "Ordenar por",
		"sort.modified:asc":                    "Modificado (más antiguo)",
		"sort.modified:desc":                   "Modificado (más reciente)",
		"sort.name:asc":                        "Nombre (A-Z)",
		"sort.name:desc":                       "Nombre (Z-A)",
		"sort.size:asc":                        "Tamaño (menor primero)",
		"sort.size:desc":                       "Tamaño (mayor primero)",
		"toast.copied":                         "¡Copiado al portapapeles!",
		"toast.copy_failed":                    "No se pudo copiar al portapapeles.",
		"toast.deleted":                        "«%s» eliminado definitivamente",
		"toast.extracted":                      "ZIP extraído en «%s»",
		"toast.file_created":                   "Archivo «%s» guardado",
		"toast.folder_created":                 "Carpeta «%s» creada",
		"toast.restored":                       "«%s» restaurado",
		"toast.trash_emptied":                  "Papelera vaciada",
		"toast.trashed":                        "«%s» movido a la papelera",
		"toast.upload_partial":                 "Se subieron %d de %d archivos",
		"toast.uploaded_many":                  "%d archivos subidos",
		"toast.uploaded_one":                   "«%s» subido",
		"toast.uploading":                      "Subiendo...",
		"trash.delete_permanent":               "Eliminar definitivamente",
		"trash.delete_permanent_confirm":       "¿Seguro que deseas eliminar definitivamente '%s'? Esta acción no se puede deshacer.",
		"trash.desc":                           "Elementos eliminados que pueden ser restaurados o eliminados permanentemente.",
		"trash.empty":                          "La papelera está vacía",
		"trash.empty_btn":                      "Vaciar papelera",
		"trash.empty_confirm":                  "¿Seguro que deseas vaciar la papelera? Esta acción no se puede deshacer.",
		"trash.empty_desc":                     "Los archivos y carpetas que elimines aparecerán aquí.",
		"trash.original_location":              "Ubicación: %s",
		"trash.restore":                        "Restaurar",
		"trash.title":                          "Papelera",
		"view.grid":                            "Vista en cuadrícula",
		"view.label":                           "Modo de vista",
		"view.list":                            "Vista en lista",
	},
	LangEN: {
		"a11y.skip":                            "Skip to content",
		"app.title":                            "simplefs - File Manager",
		"auth.logout":                          "Sign out",
		"auth.read_only":                       "Read-only access",
		"banner.desc":                          "Changes appear automatically; drop files here to upload them.",
		"banner.items_count":                   "%d items",
		"banner.title":                         "File Explorer",
		"brand.tagline":                        "File exchange",
		"breadcrumb.aria":                      "Navigation breadcrumbs",
		"breadcrumb.home":                      "Home",
		"clipboard.image_prefix":               "screenshot_",
		"clipboard.preview":                    "Preview",
		"clipboard.preview_placeholder":        "// The preview appears here once you paste content...",
		"clipboard.stats_image":                "Dimensions: %d x %d px | %s | %s KB",
		"clipboard.stats_text":                 "Lines: %d | Characters: %d | Size: %d B",
		"connect.copy":                         "Copy",
		"connect.desc":                         "Any HTTP client can upload and download files through the /api/v1 API using the access token.",
		"connect.footer":                       "A read-only token can download but not write. Add ?overwrite=true to the PUT to replace a file (the previous version goes to the trash).",
		"connect.step_download":                "3. Download a file",
		"connect.step_list":                    "4. List a folder or recent changes (JSON)",
		"connect.step_token":                   "1. Variables (get the token with \"simplefs token\")",
		"connect.step_upload":                  "2. Send a file (missing folders are created)",
		"connect.title":                        "Connect an agent or script",
		"details.close":                        "Close",
		"details.copy_link":                    "Copy link",
		"details.download":                     "Download",
		"details.location":                     "Location",
		"details.modified":                     "Modified",
		"details.size":                         "Size",
		"drag.desc":                            "They will be saved in the current folder",
		"drag.title":                           "Drop files here to upload",
		"empty.folder_desc":                    "Drop files here or use the (+) button to upload content.",
		"empty.folder_desc_readonly":           "There are no files in this folder yet.",
		"empty.folder_title":                   "This folder is empty",
		"empty.not_found_back_home":            "Back to Home",
		"empty.not_found_desc":                 "The folder '%s' does not exist or was deleted.",
		"empty.not_found_title":                "Folder not found",
		"empty.search_desc":                    "Try other terms or clear the search.",
		"empty.search_title":                   "No results found for \"%s\"",
		"error.busy":                           "Too many ZIP downloads in progress; try again in a few seconds.",
		"error.cannot_delete_root":             "The root folder cannot be deleted.",
		"error.conflict":                       "A file or folder with that name already exists.",
		"error.forbidden":                      "Access denied.",
		"error.generic":                        "The operation could not be completed.",
		"error.invalid_base64":                 "Invalid base64 content.",
		"error.invalid_name":                   "Invalid name.",
		"error.network":                        "Cannot reach the server.",
		"error.not_found":                      "File or folder not found.",
		"error.too_large":                      "The file exceeds the maximum allowed size.",
		"error.upload_invalid":                 "Invalid upload.",
		"error.zip":                            "ZIP rejected",
		"fab.actions":                          "Actions",
		"fab.new_folder":                       "New Folder",
		"fab.paste":                            "Paste Clipboard",
		"fab.upload":                           "Upload Files",
		"fab.upload_zip":                       "Upload ZIP (extract)",
		"files.actions.copy_curl":              "Copy curl command",
		"files.actions.delete":                 "Delete",
		"files.actions.details":                "Details",
		"files.actions.download":               "Download",
		"files.actions.extract_confirm":        "Are you sure you want to extract '%s' into a folder?",
		"files.actions.extract_zip":            "Extract ZIP",
		"files.actions.preview":                "Preview %s",
		"files.delete_confirm":                 "Are you sure you want to delete '%s'?",
		"files.heading":                        "Files",
		"files.table.actions":                  "Actions",
		"files.table.location":                 "Location",
		"files.table.modified":                 "Modified",
		"files.table.name":                     "Name",
		"files.table.size":                     "Size",
		"filter.all":                           "All",
		"filter.code":                          "Code",
		"filter.docs":                          "Documents",
		"filter.label":                         "Filter by type",
		"filter.media":                         "Media",
		"folders.actions.download_zip":         "Download folder '%s' as ZIP",
		"folders.delete_confirm":               "Are you sure you want to delete folder '%s' and all its contents?",
		"folders.download_zip":                 "Download as ZIP",
		"folders.heading":                      "Folders",
		"folders.info":                         "Folder information",
		"folders.item_count":                   "%d items",
		"folders.open_aria":                    "Open folder %s",
		"folders.title":                        "Folders (%d)",
		"header.lang_switch":                   "Switch language",
		"header.new_folder":                    "Folder",
		"header.paste":                         "Paste",
		"header.search_placeholder":            "Search files and folders...",
		"header.theme_toggle":                  "Toggle theme (light / dark)",
		"login.help":                           "The token is printed on first start or with \"simplefs token\" (docker exec simplefs /simplefs token).",
		"login.invalid":                        "Invalid token.",
		"login.submit":                         "Sign in",
		"login.subtitle":                       "Enter the access token to continue.",
		"login.title":                          "Sign in",
		"login.token":                          "Access token",
		"login.too_many":                       "Too many failed attempts. Wait a few minutes.",
		"modal.clipboard.content":              "Content",
		"modal.clipboard.filename":             "File name",
		"modal.clipboard.filename_placeholder": "e.g.: notes.txt or screenshot.png",
		"modal.clipboard.paste_btn":            "Paste from clipboard",
		"modal.clipboard.save_btn":             "Save File",
		"modal.clipboard.title_image":          "Clipboard Image",
		"modal.clipboard.title_text":           "Save from Clipboard",
		"modal.folder.cancel":                  "Cancel",
		"modal.folder.create":                  "Create Folder",
		"modal.folder.label":                   "Name",
		"modal.folder.placeholder":             "Folder name...",
		"modal.folder.title":                   "New Folder",
		"nav.all_files":                        "All files",
		"nav.close_menu":                       "Close navigation",
		"nav.connect":                          "Connect agent",
		"nav.navigation":                       "Navigation",
		"nav.open_menu":                        "Open navigation",
		"nav.recents":                          "Recent",
		"nav.trash":                            "Trash",
		"preview.audio_unsupported":            "Your browser does not support audio playback.",
		"preview.back":                         "Back",
		"preview.copy":                         "Copy",
		"preview.download_to_view":             "Download File",
		"preview.fit_screen":                   "Fit to screen",
		"preview.lines":                        "Lines: %d",
		"preview.print":                        "Print",
		"preview.read_error":                   "[Error reading file]",
		"preview.rotate":                       "Rotate 90°",
		"preview.too_large":                    "[File too large for inline text preview; download it instead]",
		"preview.unsupported":                  "This file type does not support online preview.",
		"preview.video_unsupported":            "Your browser does not support video playback.",
		"preview.zoom_in":                      "Zoom in",
		"preview.zoom_out":                     "Zoom out",
		"recent.desc":                          "The most recently modified files across the whole storage.",
		"recent.empty":                         "No files yet",
		"recent.empty_desc":                    "Files you upload or receive will show up here.",
		"recent.title":                         "Recent",
		"search.clear":                         "Clear search",
		"search.results_for":                   "Results for \"%s\" (including subfolders)",
		"search.truncated":                     "Only the first results are shown; refine the search to see more.",
		"sort.label":                           "Sort by",
		"sort.modified:asc":                    "Modified (oldest)",
		"sort.modified:desc":                   "Modified (newest)",
		"sort.name:asc":                        "Name (A-Z)",
		"sort.name:desc":                       "Name (Z-A)",
		"sort.size:asc":                        "Size (smallest first)",
		"sort.size:desc":                       "Size (largest first)",
		"toast.copied":                         "Copied to clipboard!",
		"toast.copy_failed":                    "Could not copy to the clipboard.",
		"toast.deleted":                        "\"%s\" permanently deleted",
		"toast.extracted":                      "ZIP extracted into \"%s\"",
		"toast.file_created":                   "File \"%s\" saved",
		"toast.folder_created":                 "Folder \"%s\" created",
		"toast.restored":                       "\"%s\" restored",
		"toast.trash_emptied":                  "Trash emptied",
		"toast.trashed":                        "\"%s\" moved to trash",
		"toast.upload_partial":                 "%d of %d files uploaded",
		"toast.uploaded_many":                  "%d files uploaded",
		"toast.uploaded_one":                   "\"%s\" uploaded",
		"toast.uploading":                      "Uploading...",
		"trash.delete_permanent":               "Delete permanently",
		"trash.delete_permanent_confirm":       "Are you sure you want to permanently delete '%s'? This action cannot be undone.",
		"trash.desc":                           "Deleted items that can be restored or permanently removed.",
		"trash.empty":                          "Trash is empty",
		"trash.empty_btn":                      "Empty trash",
		"trash.empty_confirm":                  "Are you sure you want to empty the trash? This action cannot be undone.",
		"trash.empty_desc":                     "Files and folders you delete will appear here.",
		"trash.original_location":              "Location: %s",
		"trash.restore":                        "Restore",
		"trash.title":                          "Trash",
		"view.grid":                            "Grid view",
		"view.label":                           "View mode",
		"view.list":                            "List view",
	},
}

func isSpanishTag(tag string) bool {
	return tag == LangES || strings.HasPrefix(tag, "es-") || strings.HasPrefix(tag, "es_")
}

func isEnglishTag(tag string) bool {
	return tag == LangEN || strings.HasPrefix(tag, "en-") || strings.HasPrefix(tag, "en_")
}

func NormalizeLang(lang string) string {
	trimmed := strings.ToLower(strings.TrimSpace(lang))
	if isEnglishTag(trimmed) {
		return LangEN
	}
	if isSpanishTag(trimmed) {
		return LangES
	}
	return defaultLang
}

func ResolveLang(r *http.Request) string {
	if queryLang := r.URL.Query().Get("lang"); queryLang != "" {
		return NormalizeLang(queryLang)
	}

	if r.PostForm != nil {
		if formLang := r.PostForm.Get("lang"); formLang != "" {
			return NormalizeLang(formLang)
		}
	}

	if cookie, err := r.Cookie("lang"); err == nil && cookie.Value != "" {
		return NormalizeLang(cookie.Value)
	}

	acceptLang := r.Header.Get("Accept-Language")
	if acceptLang == "" {
		return defaultLang
	}

	parts := strings.Split(acceptLang, ",")
	for _, part := range parts {
		tag := strings.TrimSpace(strings.Split(part, ";")[0])
		tagLower := strings.ToLower(tag)
		if isSpanishTag(tagLower) {
			return LangES
		}
		if isEnglishTag(tagLower) {
			return LangEN
		}
	}

	return defaultLang
}

func T(lang string, key string, args ...any) string {
	normLang := NormalizeLang(lang)
	dict := translations[normLang]

	templateStr := key
	if val, exists := dict[key]; exists {
		templateStr = val
	} else if defaultVal, defaultExists := translations[defaultLang][key]; defaultExists {
		templateStr = defaultVal
	}

	if len(args) > 0 && strings.Contains(templateStr, "%") {
		return fmt.Sprintf(templateStr, args...)
	}
	return templateStr
}

var monthsES = []string{
	"Ene", "Feb", "Mar", "Abr", "May", "Jun", "Jul", "Ago", "Sep", "Oct", "Nov", "Dic",
}

var monthsEN = []string{
	"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec",
}

func FormatDate(t time.Time, lang string) string {
	normLang := NormalizeLang(lang)
	monthIdx := int(t.Month()) - 1
	if normLang == LangES {
		return fmt.Sprintf("%02d %s %d", t.Day(), monthsES[monthIdx], t.Year())
	}
	return fmt.Sprintf("%02d %s %d", t.Day(), monthsEN[monthIdx], t.Year())
}

func FormatDateTime(t time.Time, lang string) string {
	datePart := FormatDate(t, lang)
	return fmt.Sprintf("%s, %02d:%02d", datePart, t.Hour(), t.Minute())
}
