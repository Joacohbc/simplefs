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

const (
	zipBusyRetryAfterSeconds         = "10"
	attachmentContentSecurityPolicy  = "default-src 'none'; sandbox"
	inlineMediaContentSecurityPolicy = "default-src 'none'; img-src 'self'; media-src 'self'; style-src 'unsafe-inline'"
)

type loginPageData struct {
	Lang  string
	Next  string
	Error string
}

func (h *WebApp) LoginPage(w http.ResponseWriter, r *http.Request) {
	if !h.config.Auth.Enabled() || auth.RoleFrom(r.Context()) != auth.RoleNone {
		http.Redirect(w, r, auth.SafeRedirectTarget(r.URL.Query().Get("next")), http.StatusSeeOther)
		return
	}
	lang := h.resolveLang(w, r, readParams(r))
	h.render(w, http.StatusOK, "login.html", loginPageData{Lang: lang, Next: auth.SafeRedirectTarget(r.URL.Query().Get("next"))})
}

func (h *WebApp) Login(w http.ResponseWriter, r *http.Request) {
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

func (h *WebApp) Logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, auth.LogoutCookie(r))
	if r.Header.Get("HX-Request") != "" {
		w.Header().Set("HX-Redirect", "/login")
		w.WriteHeader(http.StatusNoContent)
		return
	}
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (h *WebApp) directoryPage(r *http.Request, params viewParams, lang string) (models.PageData, int) {
	pageData, err := h.fileStore.GetDirectoryPage(params.Path, params.Query, params.View, params.Sort, params.Order, lang)
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

func (h *WebApp) Index(w http.ResponseWriter, r *http.Request) {
	params := readParams(r)
	lang := h.resolveLang(w, r, params)
	pageData, status := h.directoryPage(r, params, lang)
	h.render(w, status, "index.html", pageData)
}

func (h *WebApp) Files(w http.ResponseWriter, r *http.Request) {
	params := readParams(r)
	lang := h.resolveLang(w, r, params)
	pageData, _ := h.directoryPage(r, params, lang)
	h.render(w, http.StatusOK, "file_list.html", pageData)
}

func (h *WebApp) recentPage(w http.ResponseWriter, r *http.Request) (models.PageData, bool) {
	params := readParams(r)
	lang := h.resolveLang(w, r, params)
	pageData, err := h.fileStore.GetRecentPage(params.View, lang, storage.DefaultRecentLimit)
	if err != nil {
		h.httpError(w, lang, err)
		return models.PageData{}, false
	}
	h.decorate(r, &pageData)
	return pageData, true
}

func (h *WebApp) RecentPage(w http.ResponseWriter, r *http.Request) {
	if pageData, ok := h.recentPage(w, r); ok {
		h.render(w, http.StatusOK, "index.html", pageData)
	}
}

func (h *WebApp) ApiRecent(w http.ResponseWriter, r *http.Request) {
	if pageData, ok := h.recentPage(w, r); ok {
		h.render(w, http.StatusOK, "file_list.html", pageData)
	}
}

func (h *WebApp) renderFileList(w http.ResponseWriter, r *http.Request, params viewParams, lang string) {
	pageData, _ := h.directoryPage(r, params, lang)
	h.render(w, http.StatusOK, "file_list.html", pageData)
}

func (h *WebApp) FileDetails(w http.ResponseWriter, r *http.Request) {
	params := readParams(r)
	lang := h.resolveLang(w, r, params)
	detailsData, err := h.fileStore.GetFileDetails(params.Path, lang)
	if err != nil {
		h.httpError(w, lang, err)
		return
	}
	h.render(w, http.StatusOK, "details_modal.html", detailsData)
}

func (h *WebApp) Preview(w http.ResponseWriter, r *http.Request) {
	params := readParams(r)
	lang := h.resolveLang(w, r, params)
	previewData, err := h.fileStore.GetFilePreview(params.Path, lang)
	if err != nil {
		h.httpError(w, lang, err)
		return
	}
	h.render(w, http.StatusOK, "preview_modal.html", previewData)
}

func (h *WebApp) limitUpload(w http.ResponseWriter, r *http.Request) {
	if h.config.MaxUploadBytes > 0 {
		r.Body = http.MaxBytesReader(w, r.Body, h.config.MaxUploadBytes)
	}
}

func readFieldValue(part *multipart.Part) string {
	data, _ := io.ReadAll(io.LimitReader(part, maxFormFieldBytes))
	return strings.TrimSpace(string(data))
}

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

func (outcome *uploadOutcome) recordFailure(name string, err error, lang string) {
	if outcome.firstError == nil {
		outcome.firstError = err
	}
	outcome.failed = append(outcome.failed, fmt.Sprintf("%s (%s)", name, userFacingErrorMessage(lang, err)))
}

func (outcome uploadOutcome) processedNothing() bool {
	return len(outcome.saved) == 0 && len(outcome.failed) == 0
}

func (outcome uploadOutcome) savedNothing() bool {
	return len(outcome.saved) == 0 && outcome.firstError != nil
}

func (outcome uploadOutcome) isPartial() bool {
	return len(outcome.failed) > 0 || outcome.fatal != nil
}

func uploadedFileName(part *multipart.Part) string {
	return filepath.Base(strings.ReplaceAll(part.FileName(), "\\", "/"))
}

func isRequestTooLarge(err error) bool {
	var maxBytesError *http.MaxBytesError
	return errors.As(err, &maxBytesError)
}

type uploadPartHandler func(directory string, part *multipart.Part) (savedName string, err error)

// streamUpload processes a multipart body part by part so files go straight to
// disk without being buffered in memory or written twice.
func (h *WebApp) streamUpload(r *http.Request, params *viewParams, handleFile uploadPartHandler) uploadOutcome {
	var outcome uploadOutcome
	reader, err := r.MultipartReader()
	if err != nil {
		outcome.fatal = err
		return outcome
	}
	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			return outcome
		}
		if err != nil {
			outcome.fatal = err
			return outcome
		}
		if shouldStop := processUploadPart(part, params, handleFile, &outcome); shouldStop {
			return outcome
		}
	}
}

func processUploadPart(part *multipart.Part, params *viewParams, handleFile uploadPartHandler, outcome *uploadOutcome) (shouldStop bool) {
	defer part.Close()
	if part.FileName() == "" {
		applyField(params, part.FormName(), readFieldValue(part))
		return false
	}
	savedName, err := handleFile(params.Path, part)
	if isRequestTooLarge(err) {
		outcome.fatal = err
		return true
	}
	if err != nil {
		outcome.recordFailure(uploadedFileName(part), err, i18n.NormalizeLang(params.Lang))
		return false
	}
	outcome.saved = append(outcome.saved, savedName)
	return false
}

func uploadToast(outcome uploadOutcome, lang, successKey string) (level, message string) {
	if outcome.isPartial() {
		message = i18n.T(lang, "toast.upload_partial", len(outcome.saved), len(outcome.saved)+len(outcome.failed))
		if len(outcome.failed) > 0 {
			message += ": " + strings.Join(outcome.failed, ", ")
		}
		return toastError, message
	}
	if len(outcome.saved) == 1 {
		return toastSuccess, i18n.T(lang, successKey, outcome.saved[0])
	}
	return toastSuccess, i18n.T(lang, "toast.uploaded_many", len(outcome.saved))
}

func (h *WebApp) finishUpload(w http.ResponseWriter, r *http.Request, params viewParams, lang string, outcome uploadOutcome, successKey string) {
	if outcome.processedNothing() && isRequestTooLarge(outcome.fatal) {
		h.httpError(w, lang, outcome.fatal)
		return
	}
	if outcome.processedNothing() {
		http.Error(w, i18n.T(lang, "error.upload_invalid"), http.StatusBadRequest)
		return
	}
	if outcome.savedNothing() {
		status, _ := errorStatus(outcome.firstError)
		http.Error(w, strings.Join(outcome.failed, ", "), status)
		return
	}
	level, message := uploadToast(outcome, lang, successKey)
	setToast(w, level, message)
	h.renderFileList(w, r, params, lang)
}

func (h *WebApp) Upload(w http.ResponseWriter, r *http.Request) {
	h.limitUpload(w, r)
	params := readParams(r)
	outcome := h.streamUpload(r, &params, func(directory string, part *multipart.Part) (string, error) {
		result, err := h.fileStore.WriteFile(directory, uploadedFileName(part), part, storage.WriteOptions{Conflict: storage.ConflictRename})
		return result.Name, err
	})
	lang := h.resolveLang(w, r, params)
	h.finishUpload(w, r, params, lang, outcome, "toast.uploaded_one")
}

func (h *WebApp) UploadZip(w http.ResponseWriter, r *http.Request) {
	h.limitUpload(w, r)
	params := readParams(r)
	outcome := h.streamUpload(r, &params, func(directory string, part *multipart.Part) (string, error) {
		if !strings.HasSuffix(strings.ToLower(part.FileName()), ".zip") {
			return "", storage.ErrNotAZipArchive
		}
		return h.fileStore.ExtractZipFile(directory, part.FileName(), part)
	})
	lang := h.resolveLang(w, r, params)
	h.finishUpload(w, r, params, lang, outcome, "toast.extracted")
}

func (h *WebApp) parseForm(w http.ResponseWriter, r *http.Request, limit int64) bool {
	r.Body = http.MaxBytesReader(w, r.Body, limit)
	if err := r.ParseForm(); err != nil {
		lang := i18n.ResolveLang(r)
		h.httpError(w, lang, err)
		return false
	}
	return true
}

func (h *WebApp) ExtractZip(w http.ResponseWriter, r *http.Request) {
	if !h.parseForm(w, r, maxFormRequestBytes) {
		return
	}
	params := readParams(r)
	lang := h.resolveLang(w, r, params)
	zipPath := params.Path
	parentPath := extractParentPath(zipPath)

	folderName, err := h.fileStore.ExtractExistingZip(parentPath, baseName(zipPath))
	if err != nil {
		h.httpError(w, lang, err)
		return
	}
	params.Path = parentPath
	setToast(w, toastSuccess, i18n.T(lang, "toast.extracted", folderName))
	h.renderFileList(w, r, params, lang)
}

func (h *WebApp) Folder(w http.ResponseWriter, r *http.Request) {
	if !h.parseForm(w, r, maxFormRequestBytes) {
		return
	}
	params := readParams(r)
	lang := h.resolveLang(w, r, params)
	folderName := r.PostForm.Get("name")

	if err := h.fileStore.CreateFolder(params.Path, folderName); err != nil {
		h.httpError(w, lang, err)
		return
	}
	setToast(w, toastSuccess, i18n.T(lang, "toast.folder_created", strings.TrimSpace(folderName)))
	h.renderFileList(w, r, params, lang)
}

func (h *WebApp) CreateFile(w http.ResponseWriter, r *http.Request) {
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

	result, err := h.fileStore.WriteFile(params.Path, filename, bytes.NewReader(contentBytes), storage.WriteOptions{Conflict: storage.ConflictRename})
	if err != nil {
		h.httpError(w, lang, err)
		return
	}
	setToast(w, toastSuccess, i18n.T(lang, "toast.file_created", result.Name))
	h.renderFileList(w, r, params, lang)
}

func (h *WebApp) Delete(w http.ResponseWriter, r *http.Request) {
	params := readParams(r)
	lang := h.resolveLang(w, r, params)
	target := params.Path

	if err := h.fileStore.DeleteItem(target); err != nil {
		h.httpError(w, lang, err)
		return
	}
	params.Path = extractParentPath(target)
	setToast(w, toastSuccess, i18n.T(lang, "toast.trashed", baseName(target)))
	h.renderFileList(w, r, params, lang)
}

func (h *WebApp) acquireZipSlot() bool {
	select {
	case h.zipSlots <- struct{}{}:
		return true
	default:
		return false
	}
}

func (h *WebApp) releaseZipSlot() {
	<-h.zipSlots
}

func (h *WebApp) streamFolder(w http.ResponseWriter, r *http.Request, relativePath, lang string) {
	folderPath, archiveFilename, err := h.fileStore.GetFolderDownloadInfo(relativePath)
	if err != nil {
		h.httpError(w, lang, err)
		return
	}
	if !h.acquireZipSlot() {
		w.Header().Set("Retry-After", zipBusyRetryAfterSeconds)
		http.Error(w, i18n.T(lang, "error.busy"), http.StatusTooManyRequests)
		return
	}
	defer h.releaseZipSlot()

	setAttachment(w, archiveFilename)
	w.Header().Set("Content-Type", "application/zip")
	if err := h.fileStore.StreamFolderZip(folderPath, w); err != nil {
		log.Printf("stream folder zip error: %v", err)
	}
}

func (h *WebApp) DownloadFolder(w http.ResponseWriter, r *http.Request) {
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

// Types outside the inline allowlist are forced to download inside a sandbox CSP.
func (h *WebApp) serveStoredFile(w http.ResponseWriter, r *http.Request, relativePath, lang string, forceDownload bool) {
	filePath, filename, mimeType, isDangerousType, err := h.fileStore.GetDownloadFile(relativePath)
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
		w.Header().Set("Content-Security-Policy", attachmentContentSecurityPolicy)
	} else if strings.HasPrefix(mimeType, "application/pdf") {
		// The browser PDF viewer cannot run under the page CSP; PDFs are not scriptable in our origin.
		w.Header().Del("Content-Security-Policy")
	} else {
		w.Header().Set("Content-Security-Policy", inlineMediaContentSecurityPolicy)
	}

	if mimeType != "" {
		w.Header().Set("Content-Type", mimeType)
	}
	w.Header().Set("Cache-Control", "private, no-cache")
	w.Header().Set("ETag", fmt.Sprintf("\"%x-%x\"", info.ModTime().UnixNano(), info.Size()))
	http.ServeContent(w, r, filename, info.ModTime(), file)
}

func (h *WebApp) Download(w http.ResponseWriter, r *http.Request) {
	params := readParams(r)
	lang := h.resolveLang(w, r, params)
	if r.URL.Query().Get("archive") == "zip" {
		h.streamFolder(w, r, params.Path, lang)
		return
	}
	if _, _, err := h.fileStore.GetFolderDownloadInfo(params.Path); err == nil {
		h.streamFolder(w, r, params.Path, lang)
		return
	}
	forceDownload, _ := strconv.ParseBool(r.URL.Query().Get("download"))
	h.serveStoredFile(w, r, params.Path, lang, forceDownload)
}

func (h *WebApp) trashParams(w http.ResponseWriter, r *http.Request) (viewParams, string, string) {
	_ = r.ParseForm()
	params := readParams(r)
	lang := h.resolveLang(w, r, params)
	name := r.URL.Query().Get("name")
	if name == "" && r.PostForm != nil {
		name = r.PostForm.Get("name")
	}
	return params, lang, name
}

func (h *WebApp) renderTrashList(w http.ResponseWriter, r *http.Request, params viewParams, lang string) {
	pageData, err := h.fileStore.GetTrashPage(params.View, params.Sort, params.Order, lang)
	if err != nil {
		h.httpError(w, lang, err)
		return
	}
	h.decorate(r, &pageData)
	h.render(w, http.StatusOK, "file_list.html", pageData)
}

func (h *WebApp) Trash(w http.ResponseWriter, r *http.Request) {
	params := readParams(r)
	lang := h.resolveLang(w, r, params)
	pageData, err := h.fileStore.GetTrashPage(params.View, params.Sort, params.Order, lang)
	if err != nil {
		h.httpError(w, lang, err)
		return
	}
	h.decorate(r, &pageData)
	h.render(w, http.StatusOK, "index.html", pageData)
}

func (h *WebApp) ApiTrash(w http.ResponseWriter, r *http.Request) {
	params := readParams(r)
	h.renderTrashList(w, r, params, h.resolveLang(w, r, params))
}

func (h *WebApp) ApiTrashRestore(w http.ResponseWriter, r *http.Request) {
	params, lang, name := h.trashParams(w, r)
	if err := h.fileStore.RestoreFromTrash(name); err != nil {
		h.httpError(w, lang, err)
		return
	}
	setToast(w, toastSuccess, i18n.T(lang, "toast.restored", name))
	h.renderTrashList(w, r, params, lang)
}

func (h *WebApp) ApiTrashDelete(w http.ResponseWriter, r *http.Request) {
	params, lang, name := h.trashParams(w, r)
	if err := h.fileStore.DeletePermanentFromTrash(name); err != nil {
		h.httpError(w, lang, err)
		return
	}
	setToast(w, toastSuccess, i18n.T(lang, "toast.deleted", name))
	h.renderTrashList(w, r, params, lang)
}

func (h *WebApp) ApiTrashEmpty(w http.ResponseWriter, r *http.Request) {
	params, lang, _ := h.trashParams(w, r)
	if err := h.fileStore.EmptyTrash(); err != nil {
		h.httpError(w, lang, err)
		return
	}
	setToast(w, toastSuccess, i18n.T(lang, "toast.trash_emptied"))
	h.renderTrashList(w, r, params, lang)
}
