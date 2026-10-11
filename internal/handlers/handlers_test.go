package handlers_test

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"simplefs/internal/assets"
	"simplefs/internal/auth"
	"simplefs/internal/handlers"
	"simplefs/internal/storage"
	"simplefs/web"
)

type testServer struct {
	mux  http.Handler
	root string
}

func newTestServer(t *testing.T, authenticator *auth.Authenticator, maxUpload int64) *testServer {
	t.Helper()
	root := t.TempDir()
	staticAssets, err := assets.New(web.Assets, "static")
	if err != nil {
		t.Fatal(err)
	}
	templates, err := handlers.ParseTemplates(web.Assets, staticAssets)
	if err != nil {
		t.Fatalf("failed to parse templates: %v", err)
	}
	if authenticator == nil {
		authenticator = auth.Disabled()
	}
	handler := handlers.NewHandler(storage.NewService(root), templates, handlers.Config{
		Auth:           authenticator,
		Assets:         staticAssets,
		MaxUploadBytes: maxUpload,
	})
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return &testServer{mux: authenticator.Middleware(mux), root: root}
}

func (s *testServer) do(t *testing.T, method, target string, body io.Reader, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, target, body)
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, req)
	return rec
}

func (s *testServer) write(t *testing.T, path, content string) {
	t.Helper()
	full := filepath.Join(s.root, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func (s *testServer) exists(path string) bool {
	_, err := os.Stat(filepath.Join(s.root, filepath.FromSlash(path)))
	return err == nil
}

func multipartBody(t *testing.T, field string, files map[string]string, fields map[string]string) (*bytes.Buffer, string) {
	t.Helper()
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	for key, value := range fields {
		_ = writer.WriteField(key, value)
	}
	for name, content := range files {
		part, err := writer.CreateFormFile(field, name)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = part.Write([]byte(content))
	}
	_ = writer.Close()
	return body, writer.FormDataContentType()
}

func zipBytes(t *testing.T, files map[string]string) []byte {
	t.Helper()
	buffer := &bytes.Buffer{}
	writer := zip.NewWriter(buffer)
	for name, content := range files {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = entry.Write([]byte(content))
	}
	_ = writer.Close()
	return buffer.Bytes()
}

func decodeJSON(t *testing.T, rec *httptest.ResponseRecorder, target any) {
	t.Helper()
	if err := json.Unmarshal(rec.Body.Bytes(), target); err != nil {
		t.Fatalf("invalid JSON %q: %v", rec.Body.String(), err)
	}
}

func toastMessage(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	header := rec.Header().Get("HX-Trigger")
	if header == "" {
		return ""
	}
	for _, r := range header {
		if r > 127 {
			t.Errorf("HX-Trigger must be ASCII, got %q", header)
		}
	}
	var payload map[string]struct {
		Level   string `json:"level"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal([]byte(header), &payload); err != nil {
		t.Fatalf("invalid HX-Trigger %q: %v", header, err)
	}
	return payload["simplefs:toast"].Level + ":" + payload["simplefs:toast"].Message
}

func TestPagesRender(t *testing.T) {
	server := newTestServer(t, nil, 0)
	server.write(t, "docs/readme.md", "# hi")
	server.write(t, "photo.png", "png")

	for _, target := range []string{"/", "/?path=docs&view=grid", "/recent", "/trash", "/api/files?path=docs", "/api/recent", "/api/trash", "/api/files?query=read"} {
		rec := server.do(t, http.MethodGet, target, nil, nil)
		if rec.Code != http.StatusOK {
			t.Errorf("%s: status %d", target, rec.Code)
		}
		if !strings.Contains(rec.Body.String(), `id="active-view"`) {
			t.Errorf("%s: missing view state", target)
		}
	}

	rec := server.do(t, http.MethodGet, "/", nil, nil)
	if strings.Contains(rec.Body.String(), "onclick=") || strings.Contains(rec.Body.String(), "<script>") {
		t.Error("pages must not contain inline scripts (strict CSP)")
	}

	rec = server.do(t, http.MethodGet, "/?path=missing", nil, nil)
	if rec.Code != http.StatusNotFound {
		t.Errorf("missing folder should return 404, got %d", rec.Code)
	}

	rec = server.do(t, http.MethodGet, "/api/files?query=readme", nil, nil)
	if !strings.Contains(rec.Body.String(), "/docs") {
		t.Error("recursive search results should show the parent folder")
	}

	rec = server.do(t, http.MethodGet, "/does-not-exist", nil, nil)
	if rec.Code != http.StatusNotFound {
		t.Errorf("unknown route should 404, got %d", rec.Code)
	}
}

func TestStaticAndHealth(t *testing.T) {
	server := newTestServer(t, auth.New("secret", ""), 0)
	for _, target := range []string{"/healthz", "/favicon.ico", "/static/favicon.svg"} {
		if rec := server.do(t, http.MethodGet, target, nil, nil); rec.Code != http.StatusOK {
			t.Errorf("%s should be public, got %d", target, rec.Code)
		}
	}
}

func TestUIUploadStreamsAndRenames(t *testing.T) {
	server := newTestServer(t, nil, 0)
	server.write(t, "inbox/a.txt", "old")

	body, contentType := multipartBody(t, "files", map[string]string{"a.txt": "new", "b ñ.txt": "b"}, nil)
	rec := server.do(t, http.MethodPost, "/api/upload?path=inbox&lang=es", body, map[string]string{"Content-Type": contentType})
	if rec.Code != http.StatusOK {
		t.Fatalf("upload failed: %d %s", rec.Code, rec.Body.String())
	}
	if !server.exists("inbox/a (1).txt") || !server.exists("inbox/b ñ.txt") {
		t.Error("expected renamed upload and second file")
	}
	if content, _ := os.ReadFile(filepath.Join(server.root, "inbox", "a.txt")); string(content) != "old" {
		t.Error("UI upload must not overwrite existing files")
	}
	if toast := toastMessage(t, rec); !strings.HasPrefix(toast, "success:") {
		t.Errorf("expected success toast, got %q", toast)
	}

	body, contentType = multipartBody(t, "files", map[string]string{".trash": "x"}, map[string]string{"path": ""})
	rec = server.do(t, http.MethodPost, "/api/upload", body, map[string]string{"Content-Type": contentType})
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), ".trash") {
		t.Errorf("rejected uploads must fail with the reason, got %d %q", rec.Code, rec.Body.String())
	}

	body, contentType = multipartBody(t, "files", map[string]string{"ok.txt": "ok", ".simplefs": "x"}, nil)
	rec = server.do(t, http.MethodPost, "/api/upload?path=inbox", body, map[string]string{"Content-Type": contentType})
	if toast := toastMessage(t, rec); rec.Code != http.StatusOK || !strings.HasPrefix(toast, "error:") {
		t.Errorf("partial uploads must swap the list and report failures, got %d %q", rec.Code, toast)
	}

	rec = server.do(t, http.MethodPost, "/api/upload", strings.NewReader("not multipart"), map[string]string{"Content-Type": "text/plain"})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid body, got %d", rec.Code)
	}
}

func TestUploadSizeLimit(t *testing.T) {
	server := newTestServer(t, nil, 1024)
	body, contentType := multipartBody(t, "files", map[string]string{"big.bin": strings.Repeat("x", 4096)}, nil)
	rec := server.do(t, http.MethodPost, "/api/upload", body, map[string]string{"Content-Type": contentType})
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("expected 413, got %d", rec.Code)
	}
	if server.exists("big.bin") {
		t.Error("oversized upload must not be stored")
	}

	rec = server.do(t, http.MethodPut, "/api/v1/files/big.bin", strings.NewReader(strings.Repeat("x", 4096)), nil)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("expected 413 from API, got %d", rec.Code)
	}
}

func TestUploadAndExtractZip(t *testing.T) {
	server := newTestServer(t, nil, 0)
	body, contentType := multipartBody(t, "zip_file", map[string]string{"project.zip": string(zipBytes(t, map[string]string{".env": "K=V", "src/main.go": "package main"}))}, nil)
	rec := server.do(t, http.MethodPost, "/api/upload-zip?path=", body, map[string]string{"Content-Type": contentType})
	if rec.Code != http.StatusOK || !server.exists("project/.env") || !server.exists("project/src/main.go") {
		t.Fatalf("zip upload failed: %d %s", rec.Code, toastMessage(t, rec))
	}

	body, contentType = multipartBody(t, "zip_file", map[string]string{"evil.zip": string(zipBytes(t, map[string]string{"../evil.txt": "x"}))}, nil)
	rec = server.do(t, http.MethodPost, "/api/upload-zip", body, map[string]string{"Content-Type": contentType})
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "Zip Slip") {
		t.Errorf("zip slip must be rejected with 400, got %d %q", rec.Code, rec.Body.String())
	}

	server.write(t, "archive.zip", string(zipBytes(t, map[string]string{"a.txt": "a"})))
	rec = server.do(t, http.MethodPost, "/api/extract-zip?path=archive.zip", nil, nil)
	if rec.Code != http.StatusOK || !server.exists("archive/a.txt") {
		t.Errorf("extract existing zip failed: %d", rec.Code)
	}
}

func TestFormActions(t *testing.T) {
	server := newTestServer(t, nil, 0)
	form := func(values url.Values) io.Reader { return strings.NewReader(values.Encode()) }
	formHeaders := map[string]string{"Content-Type": "application/x-www-form-urlencoded"}

	rec := server.do(t, http.MethodPost, "/api/folder", form(url.Values{"path": {""}, "name": {"inbox"}}), formHeaders)
	if rec.Code != http.StatusOK || !server.exists("inbox") {
		t.Fatalf("create folder failed: %d", rec.Code)
	}
	rec = server.do(t, http.MethodPost, "/api/folder", form(url.Values{"name": {".trash"}}), formHeaders)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("reserved folder name should be 400, got %d", rec.Code)
	}

	image := base64.StdEncoding.EncodeToString([]byte("PNGDATA"))
	rec = server.do(t, http.MethodPost, "/api/create-file", form(url.Values{"path": {"inbox"}, "filename": {"shot.png"}, "content": {"data:image/png;base64," + image}, "is_base64": {"true"}}), formHeaders)
	if content, _ := os.ReadFile(filepath.Join(server.root, "inbox", "shot.png")); rec.Code != http.StatusOK || string(content) != "PNGDATA" {
		t.Errorf("clipboard image failed: %d %q", rec.Code, content)
	}
	rec = server.do(t, http.MethodPost, "/api/create-file", form(url.Values{"filename": {"x.png"}, "content": {"!!!"}, "is_base64": {"true"}}), formHeaders)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("invalid base64 should be 400, got %d", rec.Code)
	}

	rec = server.do(t, http.MethodDelete, "/api/delete?path=inbox/shot.png", nil, nil)
	if rec.Code != http.StatusOK || server.exists("inbox/shot.png") {
		t.Fatalf("delete failed: %d", rec.Code)
	}
	if !strings.Contains(toastMessage(t, rec), "shot.png") {
		t.Error("delete should confirm with a toast")
	}
	rec = server.do(t, http.MethodPost, "/api/trash/restore?name=shot.png", nil, nil)
	if rec.Code != http.StatusOK || !server.exists("inbox/shot.png") {
		t.Errorf("restore failed: %d", rec.Code)
	}
	rec = server.do(t, http.MethodDelete, "/api/delete?path=", nil, nil)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("deleting the root must fail with 400, got %d", rec.Code)
	}
}

func TestDownloadHeaders(t *testing.T) {
	server := newTestServer(t, nil, 0)
	server.write(t, "page.html", "<script>alert(1)</script>")
	server.write(t, "image.svg", "<svg></svg>")
	server.write(t, "notes.json", "{}")
	server.write(t, "photo.png", "png")
	server.write(t, "doc.pdf", "%PDF-1.4")
	server.write(t, "folder/a.txt", "a")

	for _, name := range []string{"page.html", "image.svg", "notes.json"} {
		rec := server.do(t, http.MethodGet, "/download?path="+name, nil, nil)
		if !strings.HasPrefix(rec.Header().Get("Content-Disposition"), "attachment") || !strings.Contains(rec.Header().Get("Content-Security-Policy"), "sandbox") {
			t.Errorf("%s must be forced to download in a sandbox: %v", name, rec.Header())
		}
	}

	rec := server.do(t, http.MethodGet, "/download?path=photo.png", nil, nil)
	if rec.Header().Get("Content-Disposition") != "" || rec.Header().Get("Content-Type") != "image/png" {
		t.Errorf("images should be served inline: %v", rec.Header())
	}

	rec = server.do(t, http.MethodGet, "/download?path=photo.png&download=1", nil, nil)
	if !strings.HasPrefix(rec.Header().Get("Content-Disposition"), "attachment") {
		t.Error("download=1 must force an attachment")
	}

	rec = server.do(t, http.MethodGet, "/download?path=photo.png", nil, map[string]string{"Range": "bytes=0-1"})
	if rec.Code != http.StatusPartialContent || rec.Body.String() != "pn" {
		t.Errorf("range requests should be supported, got %d %q", rec.Code, rec.Body.String())
	}

	rec = server.do(t, http.MethodGet, "/download?path=folder", nil, nil)
	if rec.Header().Get("Content-Type") != "application/zip" {
		t.Errorf("folders should download as zip, got %q", rec.Header().Get("Content-Type"))
	}

	rec = server.do(t, http.MethodGet, "/download?path=../etc/passwd", nil, nil)
	if rec.Code != http.StatusForbidden {
		t.Errorf("traversal should be 403, got %d", rec.Code)
	}
}

func TestErrorsDoNotLeakPaths(t *testing.T) {
	server := newTestServer(t, nil, 0)
	for _, target := range []string{"/api/preview?path=missing.txt", "/api/file-details?path=missing.txt", "/download?path=missing.txt", "/api/v1/files/missing.txt"} {
		rec := server.do(t, http.MethodGet, target, nil, nil)
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s: expected 404, got %d", target, rec.Code)
		}
		if strings.Contains(rec.Body.String(), server.root) {
			t.Errorf("%s leaks the storage path: %s", target, rec.Body.String())
		}
	}
}

func TestPreviewAndDetails(t *testing.T) {
	server := newTestServer(t, nil, 0)
	server.write(t, "main.go", "package main\n<script>")
	server.write(t, "notes.md", "# Title")

	rec := server.do(t, http.MethodGet, "/api/preview?path=main.go", nil, nil)
	if rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), "<script>") {
		t.Errorf("code preview must escape content: %d", rec.Code)
	}
	rec = server.do(t, http.MethodGet, "/api/preview?path=notes.md", nil, nil)
	if !strings.Contains(rec.Body.String(), "data-markdown=\"# Title\"") {
		t.Error("markdown preview should carry the raw source")
	}
	rec = server.do(t, http.MethodGet, "/api/file-details?path=main.go", nil, nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "main.go") {
		t.Errorf("details failed: %d", rec.Code)
	}
	rec = server.do(t, http.MethodGet, "/api/file-details?path=", nil, nil)
	if rec.Code != http.StatusOK {
		t.Errorf("root details failed: %d", rec.Code)
	}
}

func TestAPIFiles(t *testing.T) {
	server := newTestServer(t, nil, 0)

	rec := server.do(t, http.MethodPut, "/api/v1/files/inbox/deep/report.txt", strings.NewReader("hello"), nil)
	if rec.Code != http.StatusCreated {
		t.Fatalf("PUT failed: %d %s", rec.Code, rec.Body.String())
	}
	var result storage.WriteResult
	decodeJSON(t, rec, &result)
	if result.Path != "inbox/deep/report.txt" || result.Size != 5 || len(result.SHA256) != 64 {
		t.Errorf("unexpected PUT result %+v", result)
	}

	rec = server.do(t, http.MethodPut, "/api/v1/files/inbox/deep/report.txt", strings.NewReader("again"), nil)
	if rec.Code != http.StatusConflict {
		t.Errorf("second PUT should conflict, got %d", rec.Code)
	}
	rec = server.do(t, http.MethodPut, "/api/v1/files/inbox/deep/report.txt?overwrite=true", strings.NewReader("again"), nil)
	if rec.Code != http.StatusOK {
		t.Errorf("overwrite should return 200, got %d", rec.Code)
	}
	rec = server.do(t, http.MethodPut, "/api/v1/files/inbox/deep/report.txt?conflict=rename", strings.NewReader("third"), nil)
	decodeJSON(t, rec, &result)
	if rec.Code != http.StatusCreated || result.Name != "report (1).txt" {
		t.Errorf("rename policy failed: %d %+v", rec.Code, result)
	}
	rec = server.do(t, http.MethodPut, "/api/v1/files/new/x.txt?parents=false", strings.NewReader("x"), nil)
	if rec.Code != http.StatusNotFound {
		t.Errorf("parents=false into a missing folder should 404, got %d", rec.Code)
	}
	rec = server.do(t, http.MethodPut, "/api/v1/files/", strings.NewReader("x"), nil)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("PUT without a path should 400, got %d", rec.Code)
	}

	rec = server.do(t, http.MethodGet, "/api/v1/files/inbox/deep/report.txt", nil, nil)
	if rec.Code != http.StatusOK || rec.Body.String() != "again" || !strings.HasPrefix(rec.Header().Get("Content-Disposition"), "attachment") {
		t.Errorf("GET file failed: %d %q", rec.Code, rec.Body.String())
	}

	rec = server.do(t, http.MethodGet, "/api/v1/files/inbox/deep/", nil, nil)
	var listing struct {
		Path    string          `json:"path"`
		Entries []storage.Entry `json:"entries"`
	}
	decodeJSON(t, rec, &listing)
	if listing.Path != "inbox/deep" || len(listing.Entries) != 2 {
		t.Errorf("unexpected listing %+v", listing)
	}

	rec = server.do(t, http.MethodGet, "/api/v1/files/inbox/deep/report.txt?stat&checksum=true", nil, nil)
	var entry storage.Entry
	decodeJSON(t, rec, &entry)
	if entry.SHA256 == "" || entry.Size != 5 {
		t.Errorf("unexpected stat %+v", entry)
	}

	rec = server.do(t, http.MethodGet, "/api/v1/files/inbox/?archive=zip", nil, nil)
	if rec.Header().Get("Content-Type") != "application/zip" {
		t.Errorf("archive=zip should stream a zip, got %q", rec.Header().Get("Content-Type"))
	}

	body, contentType := multipartBody(t, "file", map[string]string{"a.txt": "a", "b.txt": "b"}, nil)
	rec = server.do(t, http.MethodPost, "/api/v1/files/outbox", body, map[string]string{"Content-Type": contentType})
	var posted struct {
		Files  []storage.WriteResult `json:"files"`
		Errors []map[string]string   `json:"errors"`
	}
	decodeJSON(t, rec, &posted)
	if rec.Code != http.StatusCreated || len(posted.Files) != 2 || !server.exists("outbox/a.txt") {
		t.Errorf("multipart POST failed: %d %+v", rec.Code, posted)
	}

	body, contentType = multipartBody(t, "file", map[string]string{".simplefs": "x"}, nil)
	rec = server.do(t, http.MethodPost, "/api/v1/files/outbox", body, map[string]string{"Content-Type": contentType})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("rejected multipart upload should be 400, got %d", rec.Code)
	}

	rec = server.do(t, http.MethodPost, "/api/v1/folders/one/two", nil, nil)
	if rec.Code != http.StatusCreated || !server.exists("one/two") {
		t.Errorf("mkdir failed: %d", rec.Code)
	}

	rec = server.do(t, http.MethodDelete, "/api/v1/files/outbox/a.txt", nil, nil)
	if rec.Code != http.StatusOK || server.exists("outbox/a.txt") {
		t.Errorf("delete failed: %d", rec.Code)
	}
	rec = server.do(t, http.MethodDelete, "/api/v1/files/outbox/b.txt?permanent=true", nil, nil)
	if rec.Code != http.StatusOK || server.exists("outbox/b.txt") {
		t.Errorf("permanent delete failed: %d", rec.Code)
	}

	for _, target := range []string{"/api/v1/files/.trash/x", "/api/v1/files/..%2Fsecret"} {
		rec = server.do(t, http.MethodGet, target, nil, nil)
		if rec.Code != http.StatusForbidden {
			t.Errorf("%s should be 403, got %d", target, rec.Code)
		}
	}
}

func TestAPIQueries(t *testing.T) {
	server := newTestServer(t, auth.New("tok", "ro"), 0)
	bearer := map[string]string{"Authorization": "Bearer tok"}
	readOnly := map[string]string{"Authorization": "Bearer ro"}
	server.write(t, "inbox/report-2026.pdf", "%PDF")
	server.write(t, "notes.txt", "n")

	rec := server.do(t, http.MethodGet, "/api/v1/whoami", nil, readOnly)
	var whoami map[string]any
	decodeJSON(t, rec, &whoami)
	if whoami["role"] != "read" {
		t.Errorf("unexpected whoami %+v", whoami)
	}

	rec = server.do(t, http.MethodGet, "/api/v1/search?q=report", nil, bearer)
	var search struct {
		Entries []storage.Entry `json:"entries"`
	}
	decodeJSON(t, rec, &search)
	if len(search.Entries) != 1 || search.Entries[0].Path != "inbox/report-2026.pdf" {
		t.Errorf("unexpected search %+v", search)
	}

	rec = server.do(t, http.MethodGet, "/api/v1/recent?limit=1", nil, bearer)
	var recent struct {
		Entries []storage.Entry `json:"entries"`
	}
	decodeJSON(t, rec, &recent)
	if len(recent.Entries) != 1 {
		t.Errorf("unexpected recent %+v", recent)
	}

	rec = server.do(t, http.MethodGet, "/api/v1/stamp?scope=dir&path=inbox", nil, bearer)
	var stamp map[string]string
	decodeJSON(t, rec, &stamp)
	if stamp["stamp"] == "" {
		t.Error("expected a stamp")
	}

	rec = server.do(t, http.MethodPut, "/api/v1/files/x.txt", strings.NewReader("x"), readOnly)
	if rec.Code != http.StatusForbidden {
		t.Errorf("read-only token must not write, got %d", rec.Code)
	}
	rec = server.do(t, http.MethodGet, "/api/v1/files/notes.txt", nil, readOnly)
	if rec.Code != http.StatusOK {
		t.Errorf("read-only token should download, got %d", rec.Code)
	}

	rec = server.do(t, http.MethodGet, "/", nil, readOnly)
	if strings.Contains(rec.Body.String(), "hx-delete=") {
		t.Error("read-only pages should not render delete actions")
	}
}

func TestLoginFlow(t *testing.T) {
	server := newTestServer(t, auth.New("tok", ""), 0)

	rec := server.do(t, http.MethodGet, "/", nil, map[string]string{"Accept": "text/html"})
	if rec.Code != http.StatusSeeOther || !strings.HasPrefix(rec.Header().Get("Location"), "/login") {
		t.Fatalf("expected redirect to login, got %d %q", rec.Code, rec.Header().Get("Location"))
	}

	rec = server.do(t, http.MethodGet, "/login", nil, nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `name="token"`) {
		t.Fatalf("login page failed: %d", rec.Code)
	}

	formHeaders := map[string]string{"Content-Type": "application/x-www-form-urlencoded"}
	rec = server.do(t, http.MethodPost, "/login", strings.NewReader("token=wrong&next=/"), formHeaders)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("wrong token should be 401, got %d", rec.Code)
	}

	rec = server.do(t, http.MethodPost, "/login", strings.NewReader("token=tok&next=//evil.example"), formHeaders)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/" {
		t.Fatalf("login should redirect safely, got %d %q", rec.Code, rec.Header().Get("Location"))
	}
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != auth.SessionCookieName {
		t.Fatalf("expected session cookie, got %+v", cookies)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(cookies[0])
	page := httptest.NewRecorder()
	server.mux.ServeHTTP(page, req)
	if page.Code != http.StatusOK || !strings.Contains(page.Body.String(), `action="/logout"`) {
		t.Errorf("authenticated page failed: %d", page.Code)
	}
}
