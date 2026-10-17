package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"

	"simplefs/internal/auth"
	"simplefs/internal/i18n"
	"simplefs/internal/storage"
)

// The /api/v1 endpoints are the machine-friendly interface used by agents and scripts:
// JSON responses, raw streaming uploads and meaningful HTTP status codes.
func (h *WebApp) registerAPIRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/whoami", withDeadline(h.apiWhoAmI))
	mux.HandleFunc("GET /api/v1/files/{path...}", h.apiGetFile)
	mux.HandleFunc("HEAD /api/v1/files/{path...}", h.apiGetFile)
	mux.HandleFunc("PUT /api/v1/files/{path...}", h.apiPutFile)
	mux.HandleFunc("POST /api/v1/files/{path...}", h.apiPostFiles)
	mux.HandleFunc("DELETE /api/v1/files/{path...}", withDeadline(h.apiDeleteFile))
	mux.HandleFunc("POST /api/v1/folders/{path...}", withDeadline(h.apiCreateFolder))
	mux.HandleFunc("GET /api/v1/recent", withDeadline(h.apiRecent))
	mux.HandleFunc("GET /api/v1/search", withDeadline(h.apiSearch))
	mux.HandleFunc("GET /api/v1/stamp", withDeadline(h.apiStamp))
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	_ = encoder.Encode(value)
}

func apiError(w http.ResponseWriter, err error) {
	status, _ := errorStatus(err)
	writeJSON(w, status, map[string]string{"error": userFacingErrorMessage(i18n.LangEN, err)})
}

func boolQuery(r *http.Request, key string, fallback bool) bool {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		if _, present := r.URL.Query()[key]; present {
			return true
		}
		return fallback
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return fallback
	}
	return value
}

func intQuery(r *http.Request, key string, fallback int) int {
	value, err := strconv.Atoi(r.URL.Query().Get(key))
	if err != nil {
		return fallback
	}
	return value
}

func (h *WebApp) apiWhoAmI(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"role":         auth.RoleFrom(r.Context()).String(),
		"auth_enabled": h.config.Auth.Enabled(),
		"max_upload":   h.config.MaxUploadBytes,
	})
}

func (h *WebApp) apiGetFile(w http.ResponseWriter, r *http.Request) {
	relativePath := r.PathValue("path")

	if boolQuery(r, "stat", false) {
		entry, err := h.fileStore.Stat(relativePath, boolQuery(r, "checksum", false))
		if err != nil {
			apiError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, entry)
		return
	}

	self, entries, err := h.fileStore.List(relativePath)
	if err != nil {
		apiError(w, err)
		return
	}
	if self.Type == "dir" {
		if r.URL.Query().Get("archive") == "zip" {
			h.streamFolder(w, r, relativePath, i18n.LangEN)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"path": self.Path, "entries": entries})
		return
	}
	h.serveStoredFile(w, r, relativePath, i18n.LangEN, !boolQuery(r, "inline", false))
}

func conflictFromQuery(r *http.Request, fallback storage.ConflictPolicy) storage.ConflictPolicy {
	if boolQuery(r, "overwrite", false) {
		return storage.ConflictOverwrite
	}
	return storage.ParseConflictPolicy(r.URL.Query().Get("conflict"), fallback)
}

// apiPutFile stores the raw request body at the given path: curl -T file URL.
func (h *WebApp) apiPutFile(w http.ResponseWriter, r *http.Request) {
	relativePath := strings.Trim(r.PathValue("path"), "/")
	if relativePath == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "a file path is required"})
		return
	}
	h.limitUpload(w, r)

	result, err := h.fileStore.WriteFile(extractParentPath(relativePath), baseName(relativePath), r.Body, storage.WriteOptions{
		Conflict:      conflictFromQuery(r, storage.ConflictFail),
		CreateParents: boolQuery(r, "parents", true),
	})
	if err != nil {
		apiError(w, err)
		return
	}
	status := http.StatusCreated
	if result.Overwritten {
		status = http.StatusOK
	}
	writeJSON(w, status, result)
}

type uploadFailure struct {
	Name  string `json:"name"`
	Error string `json:"error"`
	cause error
}

type multipartUploadReport struct {
	Files  []storage.WriteResult `json:"files"`
	Errors []uploadFailure       `json:"errors"`
}

func (report *multipartUploadReport) recordFailure(name string, err error) {
	report.Errors = append(report.Errors, uploadFailure{Name: name, Error: userFacingErrorMessage(i18n.LangEN, err), cause: err})
}

func (report multipartUploadReport) status() int {
	if len(report.Files) == 0 && len(report.Errors) == 0 {
		return http.StatusBadRequest
	}
	if len(report.Files) == 0 {
		status, _ := errorStatus(report.Errors[0].cause)
		return status
	}
	if len(report.Errors) > 0 {
		return http.StatusMultiStatus
	}
	return http.StatusCreated
}

func (h *WebApp) apiPostFiles(w http.ResponseWriter, r *http.Request) {
	directory := strings.Trim(r.PathValue("path"), "/")
	h.limitUpload(w, r)
	writeOptions := storage.WriteOptions{Conflict: conflictFromQuery(r, storage.ConflictRename)}
	if boolQuery(r, "parents", true) {
		if err := h.fileStore.CreateFolderPath(directory); err != nil {
			apiError(w, err)
			return
		}
	}

	reader, err := r.MultipartReader()
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "expected a multipart/form-data body"})
		return
	}

	report := multipartUploadReport{Files: []storage.WriteResult{}, Errors: []uploadFailure{}}
	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			report.recordFailure("", err)
			break
		}
		h.storeMultipartFile(directory, part, writeOptions, &report)
	}
	writeJSON(w, report.status(), report)
}

func (h *WebApp) storeMultipartFile(directory string, part *multipart.Part, writeOptions storage.WriteOptions, report *multipartUploadReport) {
	defer part.Close()
	if part.FileName() == "" {
		return
	}
	name := uploadedFileName(part)
	result, err := h.fileStore.WriteFile(directory, name, part, writeOptions)
	if err != nil {
		report.recordFailure(name, err)
		return
	}
	report.Files = append(report.Files, result)
}

func (h *WebApp) apiDeleteFile(w http.ResponseWriter, r *http.Request) {
	relativePath := r.PathValue("path")
	permanent := boolQuery(r, "permanent", false)
	var err error
	if permanent {
		err = h.fileStore.DeleteItemPermanently(relativePath)
	} else {
		err = h.fileStore.DeleteItem(relativePath)
	}
	if err != nil {
		apiError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"path": strings.Trim(relativePath, "/"), "trashed": !permanent})
}

func (h *WebApp) apiCreateFolder(w http.ResponseWriter, r *http.Request) {
	relativePath := strings.Trim(r.PathValue("path"), "/")
	if relativePath == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "a folder path is required"})
		return
	}
	if err := h.fileStore.CreateFolderPath(relativePath); err != nil {
		apiError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"path": relativePath})
}

func (h *WebApp) apiRecent(w http.ResponseWriter, r *http.Request) {
	entries, truncated, err := h.fileStore.Recent(intQuery(r, "limit", storage.DefaultRecentLimit))
	if err != nil {
		apiError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"entries": entries, "truncated": truncated})
}

func (h *WebApp) apiSearch(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	entries, truncated, err := h.fileStore.Search(r.URL.Query().Get("path"), query, intQuery(r, "limit", storage.MaxSearchResults))
	if err != nil {
		apiError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"query": query, "entries": entries, "truncated": truncated})
}

func (h *WebApp) apiStamp(w http.ResponseWriter, r *http.Request) {
	stamp, err := h.fileStore.Stamp(storage.StampScope(r.URL.Query().Get("scope")), r.URL.Query().Get("path"))
	if err != nil {
		if status, _ := errorStatus(err); status == http.StatusInternalServerError {
			log.Printf("stamp error: %v", err)
		}
		apiError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"stamp": stamp})
}
