package handlers

import (
	"embed"
	"encoding/base64"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"

	"simplefs/internal/i18n"
	"simplefs/internal/models"
	"simplefs/internal/storage"
)

const maxUploadMemoryBytes = 32 << 20
const maxUploadRequestBytes = 100 << 20

type Handler struct {
	storageService *storage.Service
	templateEngine *template.Template
}

func NewHandler(storageService *storage.Service, templateEngine *template.Template) *Handler {
	return &Handler{
		storageService: storageService,
		templateEngine: templateEngine,
	}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux, embeddedAssets embed.FS) {
	mux.Handle("GET /static/", http.FileServer(http.FS(embeddedAssets)))
	mux.HandleFunc("GET /favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		data, err := embeddedAssets.ReadFile("static/favicon.svg")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "image/svg+xml")
		w.Header().Set("Cache-Control", "public, max-age=86400")
		_, _ = w.Write(data)
	})
	mux.HandleFunc("GET /", h.Index)
	mux.HandleFunc("GET /api/files", h.Files)
	mux.HandleFunc("GET /api/file-details", h.FileDetails)
	mux.HandleFunc("POST /api/upload", h.Upload)
	mux.HandleFunc("POST /api/upload-zip", h.UploadZip)
	mux.HandleFunc("POST /api/extract-zip", h.ExtractZip)
	mux.HandleFunc("POST /api/folder", h.Folder)
	mux.HandleFunc("POST /api/create-file", h.CreateFile)
	mux.HandleFunc("DELETE /api/delete", h.Delete)
	mux.HandleFunc("GET /trash", h.Trash)
	mux.HandleFunc("GET /api/trash", h.ApiTrash)
	mux.HandleFunc("POST /api/trash/restore", h.ApiTrashRestore)
	mux.HandleFunc("DELETE /api/trash/delete", h.ApiTrashDelete)
	mux.HandleFunc("DELETE /api/trash/empty", h.ApiTrashEmpty)
	mux.HandleFunc("GET /api/preview", h.Preview)
	mux.HandleFunc("GET /api/download-folder", h.DownloadFolder)
	mux.HandleFunc("GET /download", h.Download)
}

func (h *Handler) setLangCookie(w http.ResponseWriter, r *http.Request) string {
	lang := i18n.ResolveLang(r)
	if queryLang := r.URL.Query().Get("lang"); queryLang != "" {
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

func (h *Handler) Index(w http.ResponseWriter, r *http.Request) {
	lang := h.setLangCookie(w, r)
	relativePath := r.URL.Query().Get("path")
	viewMode := r.URL.Query().Get("view")
	sortBy := r.URL.Query().Get("sort")
	sortOrder := r.URL.Query().Get("order")

	pageData, err := h.storageService.GetDirectoryPage(relativePath, "", viewMode, sortBy, sortOrder, lang)
	isNotFound := false
	if err != nil {
		if relativePath != "" {
			normalizedSortBy, normalizedSortOrder := storage.NormalizeSortParams(sortBy, sortOrder)
			if viewMode == "" {
				viewMode = models.ViewModeList
			}
			pageData = models.PageData{
				Path:         filepath.ToSlash(relativePath),
				SortBy:       normalizedSortBy,
				SortOrder:    normalizedSortOrder,
				Breadcrumbs:  storage.BuildBreadcrumbs(relativePath),
				ViewMode:     viewMode,
				Lang:         i18n.NormalizeLang(lang),
				NotFound:     true,
				NotFoundPath: relativePath,
			}
			isNotFound = true
		} else {
			log.Printf("root directory error: %v", err)
			http.Error(w, "Storage directory unavailable", http.StatusInternalServerError)
			return
		}
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if isNotFound {
		w.WriteHeader(http.StatusNotFound)
	}
	if err := h.templateEngine.ExecuteTemplate(w, "index.html", pageData); err != nil {
		log.Printf("template render error: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

func (h *Handler) Files(w http.ResponseWriter, r *http.Request) {
	lang := h.setLangCookie(w, r)
	relativePath := r.URL.Query().Get("path")
	searchQuery := r.URL.Query().Get("query")
	viewMode := r.URL.Query().Get("view")
	sortBy := r.URL.Query().Get("sort")
	sortOrder := r.URL.Query().Get("order")

	pageData, err := h.storageService.GetDirectoryPage(relativePath, searchQuery, viewMode, sortBy, sortOrder, lang)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			normalizedSortBy, normalizedSortOrder := storage.NormalizeSortParams(sortBy, sortOrder)
			if viewMode == "" {
				viewMode = models.ViewModeList
			}
			pageData = models.PageData{
				Path:         filepath.ToSlash(relativePath),
				Query:        searchQuery,
				SortBy:       normalizedSortBy,
				SortOrder:    normalizedSortOrder,
				Breadcrumbs:  storage.BuildBreadcrumbs(relativePath),
				ViewMode:     viewMode,
				Lang:         i18n.NormalizeLang(lang),
				NotFound:     true,
				NotFoundPath: relativePath,
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			if err := h.templateEngine.ExecuteTemplate(w, "file_list.html", pageData); err != nil {
				log.Printf("template render error: %v", err)
				http.Error(w, "Internal server error", http.StatusInternalServerError)
			}
			return
		}
		log.Printf("get directory error: %v", err)
		http.Error(w, "Invalid path or directory not accessible", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.templateEngine.ExecuteTemplate(w, "file_list.html", pageData); err != nil {
		log.Printf("template render error: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

func (h *Handler) FileDetails(w http.ResponseWriter, r *http.Request) {
	lang := h.setLangCookie(w, r)
	relativePath := r.URL.Query().Get("path")
	detailsData, err := h.storageService.GetFileDetails(relativePath, lang)
	if err != nil {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.templateEngine.ExecuteTemplate(w, "details_modal.html", detailsData); err != nil {
		log.Printf("template render error: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

func (h *Handler) Upload(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadRequestBytes)
	if err := r.ParseMultipartForm(maxUploadMemoryBytes); err != nil {
		http.Error(w, "Upload payload too large or invalid form", http.StatusBadRequest)
		return
	}
	defer func() {
		if r.MultipartForm != nil {
			_ = r.MultipartForm.RemoveAll()
		}
	}()

	lang := h.setLangCookie(w, r)
	relativePath := r.FormValue("path")
	viewMode := r.FormValue("view")
	sortBy := r.FormValue("sort")
	sortOrder := r.FormValue("order")

	files := r.MultipartForm.File["files"]
	for _, fileHeader := range files {
		fileStream, err := fileHeader.Open()
		if err != nil {
			continue
		}

		if err := h.storageService.SaveUploadedFile(relativePath, fileHeader.Filename, fileStream); err != nil {
			log.Printf("save uploaded file error: %v", err)
		}
		fileStream.Close()
	}

	h.renderFileList(w, relativePath, "", viewMode, sortBy, sortOrder, lang)
}

func (h *Handler) UploadZip(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadRequestBytes)
	if err := r.ParseMultipartForm(maxUploadMemoryBytes); err != nil {
		http.Error(w, "Upload payload too large or invalid form", http.StatusBadRequest)
		return
	}
	defer func() {
		if r.MultipartForm != nil {
			_ = r.MultipartForm.RemoveAll()
		}
	}()

	lang := h.setLangCookie(w, r)
	relativePath := r.FormValue("path")
	viewMode := r.FormValue("view")
	sortBy := r.FormValue("sort")
	sortOrder := r.FormValue("order")

	files := r.MultipartForm.File["zip_file"]
	if len(files) == 0 {
		files = r.MultipartForm.File["files"]
	}
	if len(files) == 0 {
		http.Error(w, "No zip file provided", http.StatusBadRequest)
		return
	}

	for _, fileHeader := range files {
		if !strings.HasSuffix(strings.ToLower(fileHeader.Filename), ".zip") {
			http.Error(w, "Only .zip files are allowed for archive extraction", http.StatusBadRequest)
			return
		}

		fileStream, err := fileHeader.Open()
		if err != nil {
			http.Error(w, "Failed to read uploaded file", http.StatusBadRequest)
			return
		}

		_, err = h.storageService.ExtractZipFile(relativePath, fileHeader.Filename, fileStream)
		fileStream.Close()
		if err != nil {
			log.Printf("secure zip extraction error: %v", err)
			http.Error(w, fmt.Sprintf("Error extracting ZIP: %v", err), http.StatusBadRequest)
			return
		}
	}

	h.renderFileList(w, relativePath, "", viewMode, sortBy, sortOrder, lang)
}

func (h *Handler) ExtractZip(w http.ResponseWriter, r *http.Request) {
	lang := h.setLangCookie(w, r)
	relativePath := r.URL.Query().Get("path")
	if relativePath == "" {
		relativePath = r.FormValue("path")
	}
	viewMode := r.FormValue("view")
	if viewMode == "" {
		viewMode = r.URL.Query().Get("view")
	}
	sortBy := r.FormValue("sort")
	if sortBy == "" {
		sortBy = r.URL.Query().Get("sort")
	}
	sortOrder := r.FormValue("order")
	if sortOrder == "" {
		sortOrder = r.URL.Query().Get("order")
	}

	parentPath := extractParentPath(relativePath)
	filename := filepath.Base(relativePath)

	_, err := h.storageService.ExtractExistingZip(parentPath, filename)
	if err != nil {
		log.Printf("extract existing zip error: %v", err)
		http.Error(w, fmt.Sprintf("Error extracting ZIP: %v", err), http.StatusBadRequest)
		return
	}

	h.renderFileList(w, parentPath, "", viewMode, sortBy, sortOrder, lang)
}

func (h *Handler) Folder(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	lang := h.setLangCookie(w, r)
	relativePath := r.FormValue("path")
	viewMode := r.FormValue("view")
	sortBy := r.FormValue("sort")
	sortOrder := r.FormValue("order")
	folderName := r.FormValue("name")

	if err := h.storageService.CreateFolder(relativePath, folderName); err != nil {
		log.Printf("create folder error: %v", err)
		http.Error(w, "Failed to create folder", http.StatusBadRequest)
		return
	}

	h.renderFileList(w, relativePath, "", viewMode, sortBy, sortOrder, lang)
}

func (h *Handler) CreateFile(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 10<<20)
	lang := h.setLangCookie(w, r)
	relativePath := r.FormValue("path")
	viewMode := r.FormValue("view")
	sortBy := r.FormValue("sort")
	sortOrder := r.FormValue("order")
	filename := r.FormValue("filename")
	rawContent := r.FormValue("content")
	isBase64Content := r.FormValue("is_base64") == "true"

	var contentBytes []byte
	if isBase64Content {
		cleanedBase64 := rawContent
		if commaIndex := strings.Index(rawContent, ","); commaIndex != -1 {
			cleanedBase64 = rawContent[commaIndex+1:]
		}

		decodedBytes, err := base64.StdEncoding.DecodeString(cleanedBase64)
		if err != nil {
			http.Error(w, "Invalid base64 payload", http.StatusBadRequest)
			return
		}
		contentBytes = decodedBytes
	} else {
		contentBytes = []byte(rawContent)
	}

	if err := h.storageService.CreateFile(relativePath, filename, contentBytes); err != nil {
		log.Printf("create file error: %v", err)
		http.Error(w, "Failed to create file", http.StatusBadRequest)
		return
	}

	h.renderFileList(w, relativePath, "", viewMode, sortBy, sortOrder, lang)
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	lang := h.setLangCookie(w, r)
	relativePath := r.URL.Query().Get("path")
	viewMode := r.URL.Query().Get("view")
	sortBy := r.URL.Query().Get("sort")
	sortOrder := r.URL.Query().Get("order")

	if err := h.storageService.DeleteItem(relativePath); err != nil {
		log.Printf("delete item error: %v", err)
		http.Error(w, "Cannot delete item", http.StatusForbidden)
		return
	}

	parentPath := extractParentPath(relativePath)
	h.renderFileList(w, parentPath, "", viewMode, sortBy, sortOrder, lang)
}

func (h *Handler) Preview(w http.ResponseWriter, r *http.Request) {
	lang := h.setLangCookie(w, r)
	relativePath := r.URL.Query().Get("path")
	previewData, err := h.storageService.GetFilePreview(relativePath, lang)
	if err != nil {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.templateEngine.ExecuteTemplate(w, "preview_modal.html", previewData); err != nil {
		log.Printf("template render error: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

func (h *Handler) DownloadFolder(w http.ResponseWriter, r *http.Request) {
	relativePath := r.URL.Query().Get("path")
	folderPath, archiveFilename, err := h.storageService.GetFolderDownloadInfo(relativePath)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			http.Error(w, "Folder not found", http.StatusNotFound)
			return
		}
		http.Error(w, "Invalid folder path or access denied", http.StatusBadRequest)
		return
	}

	encodedFilename := url.PathEscape(archiveFilename)
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q; filename*=UTF-8''%s", archiveFilename, encodedFilename))
	w.Header().Set("X-Content-Type-Options", "nosniff")

	if err := h.storageService.StreamFolderZip(folderPath, w); err != nil {
		log.Printf("stream folder zip error: %v", err)
	}
}

func (h *Handler) Download(w http.ResponseWriter, r *http.Request) {
	relativePath := r.URL.Query().Get("path")

	// If explicitly requested as zip archive, or if it is a directory, stream folder as zip
	if r.URL.Query().Get("archive") == "zip" {
		h.DownloadFolder(w, r)
		return
	}

	filePath, filename, mimeType, isDangerousType, err := h.storageService.GetDownloadFile(relativePath)
	if err != nil {
		// Check if the requested path is a directory
		if _, _, folderErr := h.storageService.GetFolderDownloadInfo(relativePath); folderErr == nil {
			h.DownloadFolder(w, r)
			return
		}
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	forceAttachment := r.URL.Query().Get("download") == "true" || isDangerousType
	if forceAttachment {
		encodedFilename := url.PathEscape(filename)
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q; filename*=UTF-8''%s", filename, encodedFilename))
		w.Header().Set("Content-Security-Policy", "default-src 'none'; sandbox")
	}

	if mimeType != "" {
		w.Header().Set("Content-Type", mimeType)
	}

	http.ServeFile(w, r, filePath)
}

func (h *Handler) renderFileList(w http.ResponseWriter, path, query, view, sort, order, lang string) {
	pageData, err := h.storageService.GetDirectoryPage(path, query, view, sort, order, lang)
	if err != nil {
		log.Printf("render file list error: %v", err)
		http.Error(w, "Failed to load directory", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = h.templateEngine.ExecuteTemplate(w, "file_list.html", pageData)
}

func extractParentPath(relativePath string) string {
	cleanRel := strings.TrimSpace(relativePath)
	if cleanRel == "" || cleanRel == "." {
		return ""
	}
	parent := strings.ReplaceAll(cleanRel, "\\", "/")
	lastSlash := strings.LastIndex(parent, "/")
	if lastSlash == -1 {
		return ""
	}
	return parent[:lastSlash]
}

func (h *Handler) Trash(w http.ResponseWriter, r *http.Request) {
	lang := h.setLangCookie(w, r)
	viewMode := r.URL.Query().Get("view")
	sortBy := r.URL.Query().Get("sort")
	sortOrder := r.URL.Query().Get("order")

	pageData, err := h.storageService.GetTrashPage(viewMode, sortBy, sortOrder, lang)
	if err != nil {
		log.Printf("get trash page error: %v", err)
		http.Error(w, "Trash unavailable", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.templateEngine.ExecuteTemplate(w, "index.html", pageData); err != nil {
		log.Printf("template render error: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

func (h *Handler) ApiTrash(w http.ResponseWriter, r *http.Request) {
	lang := h.setLangCookie(w, r)
	viewMode := r.URL.Query().Get("view")
	sortBy := r.URL.Query().Get("sort")
	sortOrder := r.URL.Query().Get("order")

	h.renderTrashList(w, viewMode, sortBy, sortOrder, lang)
}

func (h *Handler) ApiTrashRestore(w http.ResponseWriter, r *http.Request) {
	lang := h.setLangCookie(w, r)
	name := r.URL.Query().Get("name")
	if name == "" {
		name = r.FormValue("name")
	}
	viewMode := r.URL.Query().Get("view")
	if viewMode == "" {
		viewMode = r.FormValue("view")
	}
	sortBy := r.URL.Query().Get("sort")
	if sortBy == "" {
		sortBy = r.FormValue("sort")
	}
	sortOrder := r.URL.Query().Get("order")
	if sortOrder == "" {
		sortOrder = r.FormValue("order")
	}

	if err := h.storageService.RestoreFromTrash(name); err != nil {
		log.Printf("restore from trash error: %v", err)
		http.Error(w, "Failed to restore item", http.StatusBadRequest)
		return
	}

	h.renderTrashList(w, viewMode, sortBy, sortOrder, lang)
}

func (h *Handler) ApiTrashDelete(w http.ResponseWriter, r *http.Request) {
	lang := h.setLangCookie(w, r)
	name := r.URL.Query().Get("name")
	if name == "" {
		name = r.FormValue("name")
	}
	viewMode := r.URL.Query().Get("view")
	if viewMode == "" {
		viewMode = r.FormValue("view")
	}
	sortBy := r.URL.Query().Get("sort")
	if sortBy == "" {
		sortBy = r.FormValue("sort")
	}
	sortOrder := r.URL.Query().Get("order")
	if sortOrder == "" {
		sortOrder = r.FormValue("order")
	}

	if err := h.storageService.DeletePermanentFromTrash(name); err != nil {
		log.Printf("delete permanent from trash error: %v", err)
		http.Error(w, "Failed to delete item", http.StatusBadRequest)
		return
	}

	h.renderTrashList(w, viewMode, sortBy, sortOrder, lang)
}

func (h *Handler) ApiTrashEmpty(w http.ResponseWriter, r *http.Request) {
	lang := h.setLangCookie(w, r)
	viewMode := r.URL.Query().Get("view")
	if viewMode == "" {
		viewMode = r.FormValue("view")
	}
	sortBy := r.URL.Query().Get("sort")
	if sortBy == "" {
		sortBy = r.FormValue("sort")
	}
	sortOrder := r.URL.Query().Get("order")
	if sortOrder == "" {
		sortOrder = r.FormValue("order")
	}

	if err := h.storageService.EmptyTrash(); err != nil {
		log.Printf("empty trash error: %v", err)
		http.Error(w, "Failed to empty trash", http.StatusInternalServerError)
		return
	}

	h.renderTrashList(w, viewMode, sortBy, sortOrder, lang)
}

func (h *Handler) renderTrashList(w http.ResponseWriter, view, sort, order, lang string) {
	pageData, err := h.storageService.GetTrashPage(view, sort, order, lang)
	if err != nil {
		log.Printf("render trash list error: %v", err)
		http.Error(w, "Failed to load trash", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = h.templateEngine.ExecuteTemplate(w, "file_list.html", pageData)
}
