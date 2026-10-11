package handlers

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"simplefs/internal/auth"
	"simplefs/internal/i18n"
	"simplefs/internal/models"
	"simplefs/internal/storage"
)

type loginPageData struct {
	Lang  string
	Next  string
	Error string
}

func (h *Handler) LoginPage(w http.ResponseWriter, r *http.Request) {
	if !h.config.Auth.Enabled() || auth.RoleFrom(r.Context()) != auth.RoleNone {
		http.Redirect(w, r, auth.SafeRedirectTarget(r.URL.Query().Get("next")), http.StatusSeeOther)
		return
	}
	lang := h.resolveLang(w, r, readParams(r))
	h.render(w, http.StatusOK, "login.html", loginPageData{Lang: lang, Next: auth.SafeRedirectTarget(r.URL.Query().Get("next"))})
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxFormRequestBytes)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	lang := h.resolveLang(w, r, readParams(r))
	next := auth.SafeRedirectTarget(r.PostForm.Get("next"))
	if !h.config.Auth.Enabled() {
		http.Redirect(w, r, next, http.StatusSeeOther)
		return
	}

	cookie, err := h.config.Auth.Login(r, r.PostForm.Get("token"))
	if err != nil {
		status := http.StatusUnauthorized
		message := i18n.T(lang, "login.invalid")
		if errors.Is(err, auth.ErrTooManyAttempts) {
			status = http.StatusTooManyRequests
			message = i18n.T(lang, "login.too_many")
		}
		h.render(w, status, "login.html", loginPageData{Lang: lang, Next: next, Error: message})
		return
	}
	http.SetCookie(w, cookie)
	http.Redirect(w, r, next, http.StatusSeeOther)
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, auth.LogoutCookie(r))
	if r.Header.Get("HX-Request") != "" {
		w.Header().Set("HX-Redirect", "/login")
		w.WriteHeader(http.StatusNoContent)
		return
	}
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (h *Handler) directoryPage(r *http.Request, params viewParams, lang string) (models.PageData, int) {
	pageData, err := h.storageService.GetDirectoryPage(params.Path, params.Query, params.View, params.Sort, params.Order, lang)
	status := http.StatusOK
	if err != nil {
		if !errors.Is(err, storage.ErrNotFound) && params.Path == "" {
			log.Printf("root directory error: %v", err)
		}
		normalizedSortBy, normalizedSortOrder := storage.NormalizeSortParams(params.Sort, params.Order)
		pageData = models.PageData{
			Path:         filepath.ToSlash(params.Path),
			Query:        params.Query,
			SortBy:       normalizedSortBy,
			SortOrder:    normalizedSortOrder,
			Breadcrumbs:  storage.BuildBreadcrumbs(params.Path),
			ViewMode:     storage.NormalizeViewMode(params.View),
			Lang:         i18n.NormalizeLang(lang),
			NotFound:     true,
			NotFoundPath: params.Path,
		}
		status = http.StatusNotFound
	}
	h.decorate(r, &pageData)
	return pageData, status
}

func (h *Handler) Index(w http.ResponseWriter, r *http.Request) {
	params := readParams(r)
	lang := h.resolveLang(w, r, params)
	pageData, status := h.directoryPage(r, params, lang)
	h.render(w, status, "index.html", pageData)
}

func (h *Handler) Files(w http.ResponseWriter, r *http.Request) {
	params := readParams(r)
	lang := h.resolveLang(w, r, params)
	pageData, _ := h.directoryPage(r, params, lang)
	h.render(w, http.StatusOK, "file_list.html", pageData)
}

func (h *Handler) recentPage(w http.ResponseWriter, r *http.Request) (models.PageData, bool) {
	params := readParams(r)
	lang := h.resolveLang(w, r, params)
	pageData, err := h.storageService.GetRecentPage(params.View, lang, storage.DefaultRecentLimit)
	if err != nil {
		h.httpError(w, lang, err)
		return models.PageData{}, false
	}
	h.decorate(r, &pageData)
	return pageData, true
}

func (h *Handler) RecentPage(w http.ResponseWriter, r *http.Request) {
	if pageData, ok := h.recentPage(w, r); ok {
		h.render(w, http.StatusOK, "index.html", pageData)
	}
}

func (h *Handler) ApiRecent(w http.ResponseWriter, r *http.Request) {
	if pageData, ok := h.recentPage(w, r); ok {
		h.render(w, http.StatusOK, "file_list.html", pageData)
	}
}

func (h *Handler) renderFileList(w http.ResponseWriter, r *http.Request, params viewParams, lang string) {
	pageData, _ := h.directoryPage(r, params, lang)
	h.render(w, http.StatusOK, "file_list.html", pageData)
}

func (h *Handler) FileDetails(w http.ResponseWriter, r *http.Request) {
	params := readParams(r)
	lang := h.resolveLang(w, r, params)
	detailsData, err := h.storageService.GetFileDetails(params.Path, lang)
	if err != nil {
		h.httpError(w, lang, err)
		return
	}
	h.render(w, http.StatusOK, "details_modal.html", detailsData)
}

func (h *Handler) Preview(w http.ResponseWriter, r *http.Request) {
	params := readParams(r)
	lang := h.resolveLang(w, r, params)
	previewData, err := h.storageService.GetFilePreview(params.Path, lang)
	if err != nil {
		h.httpError(w, lang, err)
		return
	}
	h.render(w, http.StatusOK, "preview_modal.html", previewData)
}

func (h *Handler) limitUpload(w http.ResponseWriter, r *http.Request) {
	if h.config.MaxUploadBytes > 0 {
		r.Body = http.MaxBytesReader(w, r.Body, h.config.MaxUploadBytes)
	}
}

func readFieldValue(part *multipart.Part) string {
	data, _ := io.ReadAll(io.LimitReader(part, maxFormFieldBytes))
	return strings.TrimSpace(string(data))
}

// applyField fills a param from a multipart text field unless the URL already set it.
func applyField(params *viewParams, name, value string) {
	targets := map[string]*string{
		"path": &params.Path, "view": &params.View, "sort": &params.Sort, "order": &params.Order, "lang": &params.Lang,
	}
	if target, ok := targets[name]; ok && *target == "" {
		*target = value
	}
}

type uploadOutcome struct {
	saved      []string
	failed     []string
	firstError error
	fatal      error
}

// streamUpload processes a multipart body part by part so files go straight to
// disk without being buffered in memory or written twice.
func (h *Handler) streamUpload(r *http.Request, params *viewParams, handleFile func(dir string, part *multipart.Part) (string, error)) uploadOutcome {
	var outcome uploadOutcome
	reader, err := r.MultipartReader()
	if err != nil {
		outcome.fatal = err
		return outcome
	}
	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			outcome.fatal = err
			break
		}
		if part.FileName() == "" {
			applyField(params, part.FormName(), readFieldValue(part))
			part.Close()
			continue
		}
		name := filepath.Base(strings.ReplaceAll(part.FileName(), "\\", "/"))
		savedName, err := handleFile(params.Path, part)
		part.Close()
		if err != nil {
			var maxBytesError *http.MaxBytesError
			if errors.As(err, &maxBytesError) {
				outcome.fatal = err
				break
			}
			if outcome.firstError == nil {
				outcome.firstError = err
			}
			outcome.failed = append(outcome.failed, fmt.Sprintf("%s (%s)", name, errorMessage(i18n.NormalizeLang(params.Lang), err)))
			continue
		}
		outcome.saved = append(outcome.saved, savedName)
	}
	return outcome
}

func (h *Handler) finishUpload(w http.ResponseWriter, r *http.Request, params viewParams, lang string, outcome uploadOutcome, successKey string) {
	if outcome.fatal != nil && len(outcome.saved) == 0 && len(outcome.failed) == 0 {
		var maxBytesError *http.MaxBytesError
		if errors.As(outcome.fatal, &maxBytesError) {
			h.httpError(w, lang, outcome.fatal)
			return
		}
		http.Error(w, i18n.T(lang, "error.upload_invalid"), http.StatusBadRequest)
		return
	}
	if len(outcome.saved) == 0 && outcome.firstError != nil {
		status, _ := errorStatus(outcome.firstError)
		http.Error(w, strings.Join(outcome.failed, ", "), status)
		return
	}
	switch {
	case len(outcome.failed) > 0 || outcome.fatal != nil:
		message := i18n.T(lang, "toast.upload_partial", len(outcome.saved), len(outcome.saved)+len(outcome.failed))
		if len(outcome.failed) > 0 {
			message += ": " + strings.Join(outcome.failed, ", ")
		}
		setToast(w, "error", message)
	case len(outcome.saved) == 1:
		setToast(w, "success", i18n.T(lang, successKey, outcome.saved[0]))
	case len(outcome.saved) > 1:
		setToast(w, "success", i18n.T(lang, "toast.uploaded_many", len(outcome.saved)))
	}
	h.renderFileList(w, r, params, lang)
}

func (h *Handler) Upload(w http.ResponseWriter, r *http.Request) {
	h.limitUpload(w, r)
	params := readParams(r)
	outcome := h.streamUpload(r, &params, func(dir string, part *multipart.Part) (string, error) {
		result, err := h.storageService.WriteFile(dir, filepath.Base(strings.ReplaceAll(part.FileName(), "\\", "/")), part, storage.WriteOptions{Conflict: storage.ConflictRename})
		return result.Name, err
	})
	lang := h.resolveLang(w, r, params)
	h.finishUpload(w, r, params, lang, outcome, "toast.uploaded_one")
}

func (h *Handler) UploadZip(w http.ResponseWriter, r *http.Request) {
	h.limitUpload(w, r)
	params := readParams(r)
	outcome := h.streamUpload(r, &params, func(dir string, part *multipart.Part) (string, error) {
		if !strings.HasSuffix(strings.ToLower(part.FileName()), ".zip") {
			return "", storage.ErrNotAZipArchive
		}
		return h.storageService.ExtractZipFile(dir, part.FileName(), part)
	})
	lang := h.resolveLang(w, r, params)
	h.finishUpload(w, r, params, lang, outcome, "toast.extracted")
}

func (h *Handler) parseForm(w http.ResponseWriter, r *http.Request, limit int64) bool {
	r.Body = http.MaxBytesReader(w, r.Body, limit)
	if err := r.ParseForm(); err != nil {
		lang := i18n.ResolveLang(r)
		h.httpError(w, lang, err)
		return false
	}
	return true
}

func (h *Handler) ExtractZip(w http.ResponseWriter, r *http.Request) {
	if !h.parseForm(w, r, maxFormRequestBytes) {
		return
	}
	params := readParams(r)
	lang := h.resolveLang(w, r, params)
	zipPath := params.Path
	parentPath := extractParentPath(zipPath)

	folderName, err := h.storageService.ExtractExistingZip(parentPath, baseName(zipPath))
	if err != nil {
		h.httpError(w, lang, err)
		return
	}
	params.Path = parentPath
	setToast(w, "success", i18n.T(lang, "toast.extracted", folderName))
	h.renderFileList(w, r, params, lang)
}

func (h *Handler) Folder(w http.ResponseWriter, r *http.Request) {
	if !h.parseForm(w, r, maxFormRequestBytes) {
		return
	}
	params := readParams(r)
	lang := h.resolveLang(w, r, params)
	folderName := r.PostForm.Get("name")

	if err := h.storageService.CreateFolder(params.Path, folderName); err != nil {
		h.httpError(w, lang, err)
		return
	}
	setToast(w, "success", i18n.T(lang, "toast.folder_created", strings.TrimSpace(folderName)))
	h.renderFileList(w, r, params, lang)
}

func (h *Handler) CreateFile(w http.ResponseWriter, r *http.Request) {
	if !h.parseForm(w, r, maxClipboardRequestBytes) {
		return
	}
	params := readParams(r)
	lang := h.resolveLang(w, r, params)
	filename := r.PostForm.Get("filename")
	rawContent := r.PostForm.Get("content")

	contentBytes := []byte(rawContent)
	if r.PostForm.Get("is_base64") == "true" {
		encoded := rawContent
		if commaIndex := strings.Index(rawContent, ","); commaIndex != -1 {
			encoded = rawContent[commaIndex+1:]
		}
		decoded, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			http.Error(w, i18n.T(lang, "error.invalid_base64"), http.StatusBadRequest)
			return
		}
		contentBytes = decoded
	}

	result, err := h.storageService.WriteFile(params.Path, filename, bytes.NewReader(contentBytes), storage.WriteOptions{Conflict: storage.ConflictRename})
	if err != nil {
		h.httpError(w, lang, err)
		return
	}
	setToast(w, "success", i18n.T(lang, "toast.file_created", result.Name))
	h.renderFileList(w, r, params, lang)
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	params := readParams(r)
	lang := h.resolveLang(w, r, params)
	target := params.Path

	if err := h.storageService.DeleteItem(target); err != nil {
		h.httpError(w, lang, err)
		return
	}
	params.Path = extractParentPath(target)
	setToast(w, "success", i18n.T(lang, "toast.trashed", baseName(target)))
	h.renderFileList(w, r, params, lang)
}

func (h *Handler) acquireZipSlot() bool {
	select {
	case h.zipSlots <- struct{}{}:
		return true
	default:
		return false
	}
}

func (h *Handler) releaseZipSlot() {
	<-h.zipSlots
}

func (h *Handler) streamFolder(w http.ResponseWriter, r *http.Request, relativePath, lang string) {
	folderPath, archiveFilename, err := h.storageService.GetFolderDownloadInfo(relativePath)
	if err != nil {
		h.httpError(w, lang, err)
		return
	}
	if !h.acquireZipSlot() {
		w.Header().Set("Retry-After", "10")
		http.Error(w, i18n.T(lang, "error.busy"), http.StatusTooManyRequests)
		return
	}
	defer h.releaseZipSlot()

	setAttachment(w, archiveFilename)
	w.Header().Set("Content-Type", "application/zip")
	if err := h.storageService.StreamFolderZip(folderPath, w); err != nil {
		log.Printf("stream folder zip error: %v", err)
	}
}

func (h *Handler) DownloadFolder(w http.ResponseWriter, r *http.Request) {
	params := readParams(r)
	h.streamFolder(w, r, params.Path, h.resolveLang(w, r, params))
}

func setAttachment(w http.ResponseWriter, filename string) {
	fallback := strings.Map(func(r rune) rune {
		if r < 32 || r > 126 || r == '"' || r == '\\' {
			return '_'
		}
		return r
	}, filename)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"; filename*=UTF-8''%s", fallback, url.PathEscape(filename)))
}

// serveStoredFile streams a file with Range and conditional request support.
// Types outside the inline allowlist are forced to download inside a sandbox CSP.
func (h *Handler) serveStoredFile(w http.ResponseWriter, r *http.Request, relativePath, lang string, forceDownload bool) {
	filePath, filename, mimeType, isDangerousType, err := h.storageService.GetDownloadFile(relativePath)
	if err != nil {
		h.httpError(w, lang, err)
		return
	}

	file, err := os.Open(filePath)
	if err != nil {
		h.httpError(w, lang, storage.ErrNotFound)
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		h.httpError(w, lang, storage.ErrNotFound)
		return
	}

	if forceDownload || isDangerousType {
		setAttachment(w, filename)
		w.Header().Set("Content-Security-Policy", "default-src 'none'; sandbox")
	} else if strings.HasPrefix(mimeType, "application/pdf") {
		// The browser PDF viewer cannot run under the page CSP; PDFs are not scriptable in our origin.
		w.Header().Del("Content-Security-Policy")
	} else {
		w.Header().Set("Content-Security-Policy", "default-src 'none'; img-src 'self'; media-src 'self'; style-src 'unsafe-inline'")
	}

	if mimeType != "" {
		w.Header().Set("Content-Type", mimeType)
	}
	w.Header().Set("Cache-Control", "private, no-cache")
	w.Header().Set("ETag", fmt.Sprintf("\"%x-%x\"", info.ModTime().UnixNano(), info.Size()))
	http.ServeContent(w, r, filename, info.ModTime(), file)
}

func (h *Handler) Download(w http.ResponseWriter, r *http.Request) {
	params := readParams(r)
	lang := h.resolveLang(w, r, params)
	if r.URL.Query().Get("archive") == "zip" {
		h.streamFolder(w, r, params.Path, lang)
		return
	}
	if _, _, err := h.storageService.GetFolderDownloadInfo(params.Path); err == nil {
		h.streamFolder(w, r, params.Path, lang)
		return
	}
	forceDownload, _ := strconv.ParseBool(r.URL.Query().Get("download"))
	h.serveStoredFile(w, r, params.Path, lang, forceDownload)
}

func (h *Handler) trashParams(w http.ResponseWriter, r *http.Request) (viewParams, string, string) {
	_ = r.ParseForm()
	params := readParams(r)
	lang := h.resolveLang(w, r, params)
	name := r.URL.Query().Get("name")
	if name == "" && r.PostForm != nil {
		name = r.PostForm.Get("name")
	}
	return params, lang, name
}

func (h *Handler) renderTrashList(w http.ResponseWriter, r *http.Request, params viewParams, lang string) {
	pageData, err := h.storageService.GetTrashPage(params.View, params.Sort, params.Order, lang)
	if err != nil {
		h.httpError(w, lang, err)
		return
	}
	h.decorate(r, &pageData)
	h.render(w, http.StatusOK, "file_list.html", pageData)
}

func (h *Handler) Trash(w http.ResponseWriter, r *http.Request) {
	params := readParams(r)
	lang := h.resolveLang(w, r, params)
	pageData, err := h.storageService.GetTrashPage(params.View, params.Sort, params.Order, lang)
	if err != nil {
		h.httpError(w, lang, err)
		return
	}
	h.decorate(r, &pageData)
	h.render(w, http.StatusOK, "index.html", pageData)
}

func (h *Handler) ApiTrash(w http.ResponseWriter, r *http.Request) {
	params := readParams(r)
	h.renderTrashList(w, r, params, h.resolveLang(w, r, params))
}

func (h *Handler) ApiTrashRestore(w http.ResponseWriter, r *http.Request) {
	params, lang, name := h.trashParams(w, r)
	if err := h.storageService.RestoreFromTrash(name); err != nil {
		h.httpError(w, lang, err)
		return
	}
	setToast(w, "success", i18n.T(lang, "toast.restored", name))
	h.renderTrashList(w, r, params, lang)
}

func (h *Handler) ApiTrashDelete(w http.ResponseWriter, r *http.Request) {
	params, lang, name := h.trashParams(w, r)
	if err := h.storageService.DeletePermanentFromTrash(name); err != nil {
		h.httpError(w, lang, err)
		return
	}
	setToast(w, "success", i18n.T(lang, "toast.deleted", name))
	h.renderTrashList(w, r, params, lang)
}

func (h *Handler) ApiTrashEmpty(w http.ResponseWriter, r *http.Request) {
	params, lang, _ := h.trashParams(w, r)
	if err := h.storageService.EmptyTrash(); err != nil {
		h.httpError(w, lang, err)
		return
	}
	setToast(w, "success", i18n.T(lang, "toast.trash_emptied"))
	h.renderTrashList(w, r, params, lang)
}
