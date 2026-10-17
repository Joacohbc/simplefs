package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strings"
	"time"
	"unicode"
	"unicode/utf16"

	"simplefs/internal/assets"
	"simplefs/internal/auth"
	"simplefs/internal/i18n"
	"simplefs/internal/models"
	"simplefs/internal/storage"
)

const (
	maxClipboardRequestBytes = 64 << 20
	maxFormRequestBytes      = 1 << 20
	maxFormFieldBytes        = 4 << 10
	uiRequestTimeout         = 60 * time.Second
	defaultZipConcurrency    = 2
)

type Config struct {
	Auth           *auth.Authenticator
	Assets         *assets.Bundle
	MaxUploadBytes int64
	ZipConcurrency int
}

type WebApp struct {
	fileStore      *storage.FileStore
	templateEngine *template.Template
	config         Config
	zipSlots       chan struct{}
}

func NewWebApp(fileStore *storage.FileStore, templateEngine *template.Template, config Config) *WebApp {
	if config.Auth == nil {
		config.Auth = auth.Disabled()
	}
	if config.ZipConcurrency <= 0 {
		config.ZipConcurrency = defaultZipConcurrency
	}
	return &WebApp{
		fileStore:      fileStore,
		templateEngine: templateEngine,
		config:         config,
		zipSlots:       make(chan struct{}, config.ZipConcurrency),
	}
}

// withDeadline bounds quick UI requests; transfer endpoints are registered without it
// so large uploads and downloads are never cut off by a global timeout.
func withDeadline(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		controller := http.NewResponseController(w)
		deadline := time.Now().Add(uiRequestTimeout)
		_ = controller.SetReadDeadline(deadline)
		_ = controller.SetWriteDeadline(deadline)
		next(w, r)
	}
}

func (h *WebApp) RegisterRoutes(mux *http.ServeMux) {
	if h.config.Assets != nil {
		mux.Handle("GET /static/", h.config.Assets.Handler())
		mux.HandleFunc("GET /favicon.ico", func(w http.ResponseWriter, r *http.Request) {
			h.config.Assets.ServeFile(w, r, "favicon.svg")
		})
		mux.HandleFunc("GET /apple-touch-icon.png", func(w http.ResponseWriter, r *http.Request) {
			h.config.Assets.ServeFile(w, r, "apple-touch-icon.png")
		})
	}
	mux.HandleFunc("GET /healthz", h.Healthz)
	mux.HandleFunc("GET /login", withDeadline(h.LoginPage))
	mux.HandleFunc("POST /login", withDeadline(h.Login))
	mux.HandleFunc("POST /logout", withDeadline(h.Logout))

	mux.HandleFunc("GET /{$}", withDeadline(h.Index))
	mux.HandleFunc("GET /recent", withDeadline(h.RecentPage))
	mux.HandleFunc("GET /trash", withDeadline(h.Trash))
	mux.HandleFunc("GET /api/files", withDeadline(h.Files))
	mux.HandleFunc("GET /api/recent", withDeadline(h.ApiRecent))
	mux.HandleFunc("GET /api/trash", withDeadline(h.ApiTrash))
	mux.HandleFunc("GET /api/file-details", withDeadline(h.FileDetails))
	mux.HandleFunc("GET /api/preview", withDeadline(h.Preview))
	mux.HandleFunc("POST /api/folder", withDeadline(h.Folder))
	mux.HandleFunc("POST /api/create-file", withDeadline(h.CreateFile))
	mux.HandleFunc("DELETE /api/delete", withDeadline(h.Delete))
	mux.HandleFunc("POST /api/extract-zip", withDeadline(h.ExtractZip))
	mux.HandleFunc("POST /api/trash/restore", withDeadline(h.ApiTrashRestore))
	mux.HandleFunc("DELETE /api/trash/delete", withDeadline(h.ApiTrashDelete))
	mux.HandleFunc("DELETE /api/trash/empty", withDeadline(h.ApiTrashEmpty))

	mux.HandleFunc("POST /api/upload", h.Upload)
	mux.HandleFunc("POST /api/upload-zip", h.UploadZip)
	mux.HandleFunc("GET /api/download-folder", h.DownloadFolder)
	mux.HandleFunc("GET /download", h.Download)

	h.registerAPIRoutes(mux)

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
}

func (h *WebApp) Healthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write([]byte("ok\n"))
}

type viewParams struct {
	Path  string
	Query string
	View  string
	Sort  string
	Order string
	Lang  string
}

// readParams only looks at the URL and at an already parsed form, so it never
// consumes a streaming multipart body.
func readParams(r *http.Request) viewParams {
	paramValue := func(key string) string {
		if v := r.URL.Query().Get(key); v != "" {
			return v
		}
		if r.PostForm != nil {
			return r.PostForm.Get(key)
		}
		return ""
	}
	view := paramValue("view")
	if view == "" {
		if cookie, err := r.Cookie("view"); err == nil {
			view = cookie.Value
		}
	}
	return viewParams{
		Path:  paramValue("path"),
		Query: paramValue("query"),
		View:  view,
		Sort:  paramValue("sort"),
		Order: paramValue("order"),
		Lang:  paramValue("lang"),
	}
}

func (h *WebApp) resolveLang(w http.ResponseWriter, r *http.Request, params viewParams) string {
	lang := i18n.ResolveLang(r)
	if params.Lang != "" {
		lang = i18n.NormalizeLang(params.Lang)
	}
	if r.URL.Query().Get("lang") != "" {
		http.SetCookie(w, &http.Cookie{
			Name:     "lang",
			Value:    lang,
			Path:     "/",
			MaxAge:   365 * 24 * 3600,
			SameSite: http.SameSiteLaxMode,
			HttpOnly: true,
		})
	}
	return lang
}

func (h *WebApp) decorate(r *http.Request, page *models.PageData) {
	page.ReadOnly = auth.RoleFrom(r.Context()) == auth.RoleRead
	page.AuthEnabled = h.config.Auth.Enabled()
}

func (h *WebApp) render(w http.ResponseWriter, status int, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	if status != http.StatusOK {
		w.WriteHeader(status)
	}
	if err := h.templateEngine.ExecuteTemplate(w, name, data); err != nil {
		log.Printf("template render error (%s): %v", name, err)
	}
}

const (
	toastSuccess = "success"
	toastError   = "error"
)

type toast struct {
	Level   string `json:"level"`
	Message string `json:"message"`
}

// Non-ASCII characters are escaped so the header survives Latin-1 decoding in browsers.
func setToast(w http.ResponseWriter, level, message string) {
	payload, err := json.Marshal(map[string]toast{"simplefs:toast": {Level: level, Message: message}})
	if err != nil {
		return
	}
	w.Header().Set("HX-Trigger", asciiJSON(payload))
}

func asciiJSON(payload []byte) string {
	var builder strings.Builder
	for _, character := range string(payload) {
		if character <= unicode.MaxASCII {
			builder.WriteRune(character)
			continue
		}
		for _, codeUnit := range utf16.Encode([]rune{character}) {
			fmt.Fprintf(&builder, `\u%04x`, codeUnit)
		}
	}
	return builder.String()
}

func errorStatus(err error) (int, string) {
	switch {
	case isRequestTooLarge(err):
		return http.StatusRequestEntityTooLarge, "error.too_large"
	case errors.Is(err, storage.ErrNotFound):
		return http.StatusNotFound, "error.not_found"
	case errors.Is(err, storage.ErrInvalidName):
		return http.StatusBadRequest, "error.invalid_name"
	case errors.Is(err, storage.ErrAlreadyExists), errors.Is(err, storage.ErrIsDirectory), errors.Is(err, storage.ErrNotADirectory):
		return http.StatusConflict, "error.conflict"
	case errors.Is(err, storage.ErrCannotDeleteRoot):
		return http.StatusBadRequest, "error.cannot_delete_root"
	case errors.Is(err, storage.ErrInvalidPath), errors.Is(err, storage.ErrAccessDenied), errors.Is(err, storage.ErrProtectedItem),
		errors.Is(err, storage.ErrSymlinkForbidden), errors.Is(err, storage.ErrSymlinkOverwrite):
		return http.StatusForbidden, "error.forbidden"
	case isZipError(err):
		return http.StatusBadRequest, "error.zip"
	}
	return http.StatusInternalServerError, "error.generic"
}

func isZipError(err error) bool {
	for _, zipErr := range []error{
		storage.ErrZipEmpty, storage.ErrZipTooManyFiles, storage.ErrZipTotalSizeLimit, storage.ErrZipSingleFileSizeLimit,
		storage.ErrZipCompressionRatio, storage.ErrZipPathTraversal, storage.ErrZipSymlinkForbidden,
		storage.ErrZipRestrictedSegment, storage.ErrZipInvalidName, storage.ErrNotAZipArchive,
	} {
		if errors.Is(err, zipErr) {
			return true
		}
	}
	return false
}

func userFacingErrorMessage(lang string, err error) string {
	status, key := errorStatus(err)
	if key == "error.zip" {
		return i18n.T(lang, key) + ": " + err.Error()
	}
	if status == http.StatusInternalServerError {
		log.Printf("internal error: %v", err)
	}
	return i18n.T(lang, key)
}

func (h *WebApp) httpError(w http.ResponseWriter, lang string, err error) {
	status, _ := errorStatus(err)
	http.Error(w, userFacingErrorMessage(lang, err), status)
}

func extractParentPath(relativePath string) string {
	cleanRel := strings.Trim(strings.ReplaceAll(strings.TrimSpace(relativePath), "\\", "/"), "/")
	if cleanRel == "" || cleanRel == "." {
		return ""
	}
	lastSlash := strings.LastIndex(cleanRel, "/")
	if lastSlash == -1 {
		return ""
	}
	return cleanRel[:lastSlash]
}

func baseName(relativePath string) string {
	cleanRel := strings.Trim(strings.ReplaceAll(strings.TrimSpace(relativePath), "\\", "/"), "/")
	if lastSlash := strings.LastIndex(cleanRel, "/"); lastSlash != -1 {
		return cleanRel[lastSlash+1:]
	}
	return cleanRel
}
