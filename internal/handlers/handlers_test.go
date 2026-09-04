package handlers_test

import (
	"bytes"
	"encoding/base64"
	"html/template"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
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

func TestHandler_RegisterRoutes(t *testing.T) {
	handler, _ := setupTestServer(t)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux, web.Assets)

	req := httptest.NewRequest(http.MethodGet, "/static/css/styles.css", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK && rec.Code != http.StatusNotFound {
		t.Errorf("unexpected status code for static file: %d", rec.Code)
	}
}

func TestHandler_Index(t *testing.T) {
	handler, tempDir := setupTestServer(t)

	if err := os.WriteFile(filepath.Join(tempDir, "sample.txt"), []byte("sample content"), 0600); err != nil {
		t.Fatal(err)
	}

	t.Run("renders index page successfully", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/?lang=en&view=grid&sort=name&order=asc", nil)
		rec := httptest.NewRecorder()

		handler.Index(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "sample.txt") {
			t.Errorf("body does not contain file name: %s", rec.Body.String())
		}

		cookies := rec.Result().Cookies()
		foundLangCookie := false
		for _, cookie := range cookies {
			if cookie.Name == "lang" && cookie.Value == "en" {
				foundLangCookie = true
				break
			}
		}
		if !foundLangCookie {
			t.Error("expected lang cookie to be set")
		}
	})

	t.Run("returns 404 for invalid directory path", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/?path=../outside", nil)
		rec := httptest.NewRecorder()

		handler.Index(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("expected 404 for invalid path, got %d", rec.Code)
		}
	})
}

func TestHandler_Files(t *testing.T) {
	handler, tempDir := setupTestServer(t)

	if err := os.WriteFile(filepath.Join(tempDir, "alpha.txt"), []byte("alpha"), 0600); err != nil {
		t.Fatal(err)
	}

	t.Run("renders file list fragment", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/files?query=alpha", nil)
		rec := httptest.NewRecorder()

		handler.Files(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "alpha.txt") {
			t.Errorf("expected fragment to contain alpha.txt")
		}
	})

	t.Run("returns 400 for path traversal", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/files?path=../outside", nil)
		rec := httptest.NewRecorder()

		handler.Files(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request, got %d", rec.Code)
		}
	})
}

func TestHandler_FileDetails(t *testing.T) {
	handler, tempDir := setupTestServer(t)

	if err := os.WriteFile(filepath.Join(tempDir, "info.txt"), []byte("details content"), 0600); err != nil {
		t.Fatal(err)
	}

	t.Run("renders file details modal", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/file-details?path=info.txt", nil)
		rec := httptest.NewRecorder()

		handler.FileDetails(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "info.txt") {
			t.Errorf("expected details body to contain file name")
		}
	})

	t.Run("returns 404 for missing file details", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/file-details?path=missing.txt", nil)
		rec := httptest.NewRecorder()

		handler.FileDetails(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("expected status 404, got %d", rec.Code)
		}
	})
}

func TestHandler_Upload(t *testing.T) {
	handler, tempDir := setupTestServer(t)

	t.Run("uploads multiple files", func(t *testing.T) {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)

		_ = writer.WriteField("path", "")
		part1, _ := writer.CreateFormFile("files", "uploaded_a.txt")
		_, _ = part1.Write([]byte("content a"))
		part2, _ := writer.CreateFormFile("files", "uploaded_b.txt")
		_, _ = part2.Write([]byte("content b"))
		_ = writer.Close()

		req := httptest.NewRequest(http.MethodPost, "/api/upload", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		rec := httptest.NewRecorder()

		handler.Upload(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rec.Code)
		}

		if _, err := os.Stat(filepath.Join(tempDir, "uploaded_a.txt")); err != nil {
			t.Errorf("uploaded file a does not exist: %v", err)
		}
		if _, err := os.Stat(filepath.Join(tempDir, "uploaded_b.txt")); err != nil {
			t.Errorf("uploaded file b does not exist: %v", err)
		}
	})

	t.Run("returns 400 for malformed multipart request", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/upload", strings.NewReader("invalid-form"))
		req.Header.Set("Content-Type", "multipart/form-data; boundary=invalid")
		rec := httptest.NewRecorder()

		handler.Upload(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for malformed form, got %d", rec.Code)
		}
	})
}

func TestHandler_Folder(t *testing.T) {
	handler, tempDir := setupTestServer(t)

	t.Run("creates new folder successfully", func(t *testing.T) {
		form := url.Values{}
		form.Set("path", "")
		form.Set("name", "new_folder")

		req := httptest.NewRequest(http.MethodPost, "/api/folder", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()

		handler.Folder(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rec.Code)
		}

		if info, err := os.Stat(filepath.Join(tempDir, "new_folder")); err != nil || !info.IsDir() {
			t.Errorf("expected folder to exist, err: %v", err)
		}
	})

	t.Run("returns 400 for invalid folder name", func(t *testing.T) {
		form := url.Values{}
		form.Set("path", "")
		form.Set("name", ".hidden")

		req := httptest.NewRequest(http.MethodPost, "/api/folder", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()

		handler.Folder(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", rec.Code)
		}
	})
}

func TestHandler_CreateFile(t *testing.T) {
	handler, tempDir := setupTestServer(t)

	t.Run("creates plain text file", func(t *testing.T) {
		form := url.Values{}
		form.Set("path", "")
		form.Set("filename", "notes.txt")
		form.Set("content", "plain text payload")

		req := httptest.NewRequest(http.MethodPost, "/api/create-file", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()

		handler.CreateFile(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rec.Code)
		}

		content, err := os.ReadFile(filepath.Join(tempDir, "notes.txt"))
		if err != nil || string(content) != "plain text payload" {
			t.Errorf("unexpected created file content: %s, err: %v", string(content), err)
		}
	})

	t.Run("creates base64 encoded file with data URI prefix", func(t *testing.T) {
		rawBinary := []byte("binary image simulation")
		encoded := base64.StdEncoding.EncodeToString(rawBinary)
		dataURI := "data:image/png;base64," + encoded

		form := url.Values{}
		form.Set("path", "")
		form.Set("filename", "image.png")
		form.Set("content", dataURI)
		form.Set("is_base64", "true")

		req := httptest.NewRequest(http.MethodPost, "/api/create-file", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()

		handler.CreateFile(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rec.Code)
		}

		content, err := os.ReadFile(filepath.Join(tempDir, "image.png"))
		if err != nil || !bytes.Equal(content, rawBinary) {
			t.Errorf("unexpected decoded binary content: %s, err: %v", string(content), err)
		}
	})

	t.Run("returns 400 for invalid base64 content", func(t *testing.T) {
		form := url.Values{}
		form.Set("path", "")
		form.Set("filename", "bad.png")
		form.Set("content", "not_valid_base64!!!")
		form.Set("is_base64", "true")

		req := httptest.NewRequest(http.MethodPost, "/api/create-file", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()

		handler.CreateFile(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for invalid base64, got %d", rec.Code)
		}
	})

	t.Run("returns 400 for invalid filename", func(t *testing.T) {
		form := url.Values{}
		form.Set("path", "")
		form.Set("filename", ".env")
		form.Set("content", "secret")

		req := httptest.NewRequest(http.MethodPost, "/api/create-file", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()

		handler.CreateFile(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for protected item name, got %d", rec.Code)
		}
	})
}

func TestHandler_Delete(t *testing.T) {
	handler, tempDir := setupTestServer(t)

	targetFile := filepath.Join(tempDir, "trash.txt")
	if err := os.WriteFile(targetFile, []byte("to delete"), 0600); err != nil {
		t.Fatal(err)
	}

	t.Run("deletes file successfully", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/api/delete?path=trash.txt", nil)
		rec := httptest.NewRecorder()

		handler.Delete(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rec.Code)
		}
		if _, err := os.Stat(targetFile); !os.IsNotExist(err) {
			t.Error("file still exists after deletion")
		}
	})

	t.Run("returns 403 when attempting to delete root directory", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/api/delete?path=", nil)
		rec := httptest.NewRecorder()

		handler.Delete(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Errorf("expected status 403 Forbidden, got %d", rec.Code)
		}
	})
}

func TestHandler_Preview(t *testing.T) {
	handler, tempDir := setupTestServer(t)

	mainFile := filepath.Join(tempDir, "main.go")
	if err := os.WriteFile(mainFile, []byte("package main\n\nfunc main() {}\n"), 0600); err != nil {
		t.Fatal(err)
	}

	t.Run("renders preview modal for existing file", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/preview?path=main.go", nil)
		rec := httptest.NewRecorder()

		handler.Preview(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "main.go") {
			t.Errorf("expected preview body to contain filename")
		}
	})

	t.Run("returns 404 for missing preview target", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/preview?path=notfound.go", nil)
		rec := httptest.NewRecorder()

		handler.Preview(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("expected 404 for missing file preview, got %d", rec.Code)
		}
	})
}

func TestHandler_DownloadSecurityHeaders(t *testing.T) {
	handler, tempDir := setupTestServer(t)

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
