package storage_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"simplefs/internal/storage"
)

func TestStorageService_PathTraversalAndRestrictedSegments(t *testing.T) {
	tempDir := t.TempDir()
	service := storage.NewService(tempDir)

	publicDir := filepath.Join(tempDir, "public")
	if err := os.Mkdir(publicDir, 0700); err != nil {
		t.Fatalf("failed to create public directory: %v", err)
	}

	tests := []struct {
		name        string
		path        string
		expectError bool
	}{
		{"valid relative path", "public", false},
		{"valid nested empty path", "", false},
		{"dot path", ".", false},
		{"path traversal with dot-dot", "../etc/passwd", true},
		{"nested path traversal", "public/../../etc", true},
		{"access to hidden dot-file", ".env", true},
		{"access to nested hidden folder", "public/.git/config", true},
		{"access to simplefs binary", "simplefs", true},
		{"access to node_modules", "node_modules", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolved, err := service.ResolvePath(tt.path)
			if tt.expectError {
				if err == nil {
					t.Errorf("expected error for path %q, got resolved: %q", tt.path, resolved)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error for path %q: %v", tt.path, err)
				}
				if !strings.HasPrefix(resolved, tempDir) {
					t.Errorf("resolved path %q escapes root %q", resolved, tempDir)
				}
			}
		})
	}
}

func TestStorageService_SymlinkSecurity(t *testing.T) {
	tempDir := t.TempDir()
	outsideDir := t.TempDir()

	service := storage.NewService(tempDir)

	outsideSecret := filepath.Join(outsideDir, "secret.txt")
	if err := os.WriteFile(outsideSecret, []byte("super-secret-data"), 0600); err != nil {
		t.Fatalf("failed to write outside secret: %v", err)
	}

	symlinkToOutside := filepath.Join(tempDir, "link_to_outside")
	if err := os.Symlink(outsideDir, symlinkToOutside); err != nil {
		t.Skipf("symlinks not supported on this platform/environment: %v", err)
	}

	_, err := service.ResolvePath("link_to_outside")
	if err == nil {
		t.Error("expected ResolvePath on external symlink to fail, but it succeeded")
	}

	_, err = service.ResolvePath("link_to_outside/non_existent.txt")
	if err == nil {
		t.Error("expected ResolvePath for new file inside external symlink to fail (SEC-01), but it succeeded")
	}

	err = service.CreateFile("link_to_outside", "evil.txt", []byte("evil payload"))
	if err == nil {
		t.Error("expected CreateFile inside external symlink to fail, but it succeeded")
	}

	symlinkToFile := filepath.Join(tempDir, "target_symlink.txt")
	if err := os.Symlink(outsideSecret, symlinkToFile); err != nil {
		t.Fatalf("failed to create symlink: %v", err)
	}

	err = service.SaveUploadedFile("", "target_symlink.txt", bytes.NewReader([]byte("overwrite content")))
	if err == nil {
		t.Error("expected SaveUploadedFile on existing symlink to fail, but it succeeded")
	}

	secretContent, _ := os.ReadFile(outsideSecret)
	if string(secretContent) != "super-secret-data" {
		t.Errorf("outside file was modified! Content: %q", string(secretContent))
	}
}

func TestStorageService_GetDownloadFile_DangerousExtensions(t *testing.T) {
	tempDir := t.TempDir()
	service := storage.NewService(tempDir)

	files := map[string]bool{
		"exploit.HTML":   true,
		"vector.SVG":     true,
		"document.xhtml": true,
		"script.xml":     true,
		"image.png":      false,
		"photo.JPG":      false,
		"data.json":      false,
	}

	for filename, shouldForceAttachment := range files {
		filePath := filepath.Join(tempDir, filename)
		if err := os.WriteFile(filePath, []byte("content"), 0600); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}

		_, _, _, isForced, err := service.GetDownloadFile(filename)
		if err != nil {
			t.Fatalf("GetDownloadFile(%q) failed: %v", filename, err)
		}

		if isForced != shouldForceAttachment {
			t.Errorf("file %q: got forceAttachment=%v, want %v", filename, isForced, shouldForceAttachment)
		}
	}
}

func TestStorageService_DeleteItem_ProtectedPaths(t *testing.T) {
	tempDir := t.TempDir()
	service := storage.NewService(tempDir)

	tests := []struct {
		name        string
		path        string
		expectError bool
	}{
		{"root path", "", true},
		{"dot path", ".", true},
		{"slash path", "/", true},
		{"protected dot-segment", ".git", true},
		{"protected segment name", "node_modules", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.DeleteItem(tt.path)
			if tt.expectError && err == nil {
				t.Errorf("expected DeleteItem(%q) to fail, but got nil", tt.path)
			}
		})
	}
}
