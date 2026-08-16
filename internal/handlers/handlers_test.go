package handlers_test

import (
	"html/template"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"simplefs/internal/handlers"
	"simplefs/internal/i18n"
	"simplefs/internal/storage"
	"simplefs/web"
)

func setupTestServer(t *testing.T) (*handlers.Handler, string) {
	t.Helper()
	tempDir := t.TempDir()
	storageService := storage.NewService(tempDir)

	tmpl, err := template.New("").Funcs(template.FuncMap{
		"T": i18n.T,
	}).ParseFS(web.Assets, "templates/*.html")
	if err != nil {
		t.Fatalf("failed to parse templates: %v", err)
	}

	handler := handlers.NewHandler(storageService, tmpl)
	return handler, tempDir
}

func TestHandler_DownloadSecurityHeaders(t *testing.T) {
	handler, tempDir := setupTestServer(t)

	// Create a test dangerous file
	htmlFile := filepath.Join(tempDir, "test.HTML")
	if err := os.WriteFile(htmlFile, []byte("<h1>hello</h1>"), 0600); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/download?path=test.HTML", nil)
	rec := httptest.NewRecorder()

	handler.Download(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	contentDisp := rec.Header().Get("Content-Disposition")
	if !strings.Contains(contentDisp, "attachment") {
		t.Errorf("expected Content-Disposition to contain 'attachment', got %q", contentDisp)
	}

	csp := rec.Header().Get("Content-Security-Policy")
	if !strings.Contains(csp, "sandbox") {
		t.Errorf("expected Content-Security-Policy to contain 'sandbox', got %q", csp)
	}
}

func TestHandler_CreateFile_SizeLimit(t *testing.T) {
	handler, _ := setupTestServer(t)

	// Create payload larger than 10MB
	largePayload := "filename=test.txt&content=" + strings.Repeat("A", 11*1024*1024)

	req := httptest.NewRequest(http.MethodPost, "/api/create-file", strings.NewReader(largePayload))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	handler.CreateFile(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request for oversized body, got %d", rec.Code)
	}
}

func TestHandler_ErrorMasking(t *testing.T) {
	handler, _ := setupTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/files?path=../invalid/path", nil)
	rec := httptest.NewRecorder()

	handler.Files(rec, req)

	body := rec.Body.String()
	if strings.Contains(body, "/home/") || strings.Contains(body, "/var/") || strings.Contains(body, "\\") {
		t.Errorf("error response leaks internal server path: %q", body)
	}
}
