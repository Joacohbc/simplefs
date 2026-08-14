package handlers

import (
	"embed"
	"encoding/base64"
	"fmt"
	"html/template"
	"net/http"
	"strings"

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
	mux.HandleFunc("GET /", h.Index)
	mux.HandleFunc("GET /api/files", h.Files)
	mux.HandleFunc("GET /api/file-details", h.FileDetails)
	mux.HandleFunc("POST /api/upload", h.Upload)
	mux.HandleFunc("POST /api/folder", h.Folder)
	mux.HandleFunc("POST /api/create-file", h.CreateFile)
	mux.HandleFunc("DELETE /api/delete", h.Delete)
	mux.HandleFunc("GET /api/preview", h.Preview)
	mux.HandleFunc("GET /download", h.Download)
}

func (h *Handler) Index(w http.ResponseWriter, r *http.Request) {
	relativePath := r.URL.Query().Get("path")
	viewMode := r.URL.Query().Get("view")
	sortBy := r.URL.Query().Get("sort")
	sortOrder := r.URL.Query().Get("order")

	pageData, err := h.storageService.GetDirectoryPage(relativePath, "", viewMode, sortBy, sortOrder)
	if err != nil {
		http.Error(w, "Directory not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.templateEngine.ExecuteTemplate(w, "index.html", pageData); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (h *Handler) Files(w http.ResponseWriter, r *http.Request) {
	relativePath := r.URL.Query().Get("path")
	searchQuery := r.URL.Query().Get("query")
	viewMode := r.URL.Query().Get("view")
	sortBy := r.URL.Query().Get("sort")
	sortOrder := r.URL.Query().Get("order")

	pageData, err := h.storageService.GetDirectoryPage(relativePath, searchQuery, viewMode, sortBy, sortOrder)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.templateEngine.ExecuteTemplate(w, "file_list.html", pageData); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (h *Handler) FileDetails(w http.ResponseWriter, r *http.Request) {
	relativePath := r.URL.Query().Get("path")
	detailsData, err := h.storageService.GetFileDetails(relativePath)
	if err != nil {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.templateEngine.ExecuteTemplate(w, "details_modal.html", detailsData); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (h *Handler) Upload(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadRequestBytes)
	if err := r.ParseMultipartForm(maxUploadMemoryBytes); err != nil {
		http.Error(w, "Upload payload too large or invalid form", http.StatusBadRequest)
		return
	}

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

		_ = h.storageService.SaveUploadedFile(relativePath, fileHeader.Filename, fileStream)
		fileStream.Close()
	}

	h.renderFileList(w, relativePath, "", viewMode, sortBy, sortOrder)
}

func (h *Handler) Folder(w http.ResponseWriter, r *http.Request) {
	relativePath := r.FormValue("path")
	viewMode := r.FormValue("view")
	sortBy := r.FormValue("sort")
	sortOrder := r.FormValue("order")
	folderName := r.FormValue("name")

	if err := h.storageService.CreateFolder(relativePath, folderName); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.renderFileList(w, relativePath, "", viewMode, sortBy, sortOrder)
}

func (h *Handler) CreateFile(w http.ResponseWriter, r *http.Request) {
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
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.renderFileList(w, relativePath, "", viewMode, sortBy, sortOrder)
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	relativePath := r.URL.Query().Get("path")
	viewMode := r.URL.Query().Get("view")
	sortBy := r.URL.Query().Get("sort")
	sortOrder := r.URL.Query().Get("order")

	if err := h.storageService.DeleteItem(relativePath); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	parentPath := extractParentPath(relativePath)
	h.renderFileList(w, parentPath, "", viewMode, sortBy, sortOrder)
}

func (h *Handler) Preview(w http.ResponseWriter, r *http.Request) {
	relativePath := r.URL.Query().Get("path")
	previewData, err := h.storageService.GetFilePreview(relativePath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.templateEngine.ExecuteTemplate(w, "preview_modal.html", previewData); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (h *Handler) Download(w http.ResponseWriter, r *http.Request) {
	relativePath := r.URL.Query().Get("path")
	filePath, filename, mimeType, isDangerousType, err := h.storageService.GetDownloadFile(relativePath)
	if err != nil {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	if mimeType != "" {
		w.Header().Set("Content-Type", mimeType)
	}

	forceAttachment := r.URL.Query().Get("download") == "true" || isDangerousType
	if forceAttachment {
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	}

	http.ServeFile(w, r, filePath)
}

func (h *Handler) renderFileList(w http.ResponseWriter, path, query, view, sort, order string) {
	pageData, err := h.storageService.GetDirectoryPage(path, query, view, sort, order)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
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
