package storage_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"simplefs/internal/models"
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
		errTarget   error
	}{
		{"valid relative path", "public", false, nil},
		{"valid nested empty path", "", false, nil},
		{"dot path", ".", false, nil},
		{"slash path", "/", false, nil},
		{"backslash path", "public\\nested", false, nil},
		{"path traversal with dot-dot", "../etc/passwd", true, storage.ErrInvalidPath},
		{"nested path traversal", "public/../../etc", true, storage.ErrInvalidPath},
		{"access to hidden dot-file", ".env", true, storage.ErrProtectedItem},
		{"access to nested hidden folder", "public/.git/config", true, storage.ErrProtectedItem},
		{"access to simplefs binary", "simplefs", true, storage.ErrProtectedItem},
		{"access to node_modules", "node_modules", true, storage.ErrProtectedItem},
		{"access to stitch directory", ".stitch", true, storage.ErrProtectedItem},
		{"access to dc_simplefs directory", ".dc_simplefs", true, storage.ErrProtectedItem},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolved, err := service.ResolvePath(tt.path)
			if tt.expectError {
				if err == nil {
					t.Fatalf("expected error for path %q, got resolved: %q", tt.path, resolved)
				}
				if tt.errTarget != nil && !errors.Is(err, tt.errTarget) {
					t.Errorf("expected error %v, got %v", tt.errTarget, err)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error for path %q: %v", tt.path, err)
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
	if !errors.Is(err, storage.ErrSymlinkForbidden) {
		t.Errorf("expected ErrSymlinkForbidden for external symlink, got %v", err)
	}

	_, err = service.ResolvePath("link_to_outside/non_existent.txt")
	if !errors.Is(err, storage.ErrSymlinkForbidden) {
		t.Errorf("expected ErrSymlinkForbidden for new file inside external symlink, got %v", err)
	}

	_, err = service.ResolvePath("link_to_outside/nested/deep/file.txt")
	if !errors.Is(err, storage.ErrSymlinkForbidden) {
		t.Errorf("expected ErrSymlinkForbidden for deep path inside external symlink, got %v", err)
	}

	err = service.CreateFile("link_to_outside", "evil.txt", []byte("evil payload"))
	if err == nil {
		t.Error("expected CreateFile inside external symlink to fail, but it succeeded")
	}

	err = service.CreateFolder("link_to_outside", "evil_folder")
	if err == nil {
		t.Error("expected CreateFolder inside external symlink to fail, but it succeeded")
	}

	symlinkToFile := filepath.Join(tempDir, "target_symlink.txt")
	if err := os.Symlink(outsideSecret, symlinkToFile); err != nil {
		t.Fatalf("failed to create symlink: %v", err)
	}

	err = service.SaveUploadedFile("", "target_symlink.txt", bytes.NewReader([]byte("overwrite content")))
	if !errors.Is(err, storage.ErrSymlinkOverwrite) {
		t.Errorf("expected ErrSymlinkOverwrite on existing symlink, got %v", err)
	}

	err = service.CreateFile("", "target_symlink.txt", []byte("overwrite content"))
	if !errors.Is(err, storage.ErrSymlinkOverwrite) {
		t.Errorf("expected ErrSymlinkOverwrite in CreateFile on existing symlink, got %v", err)
	}

	err = service.CreateFolder("", "target_symlink.txt")
	if !errors.Is(err, storage.ErrSymlinkOverwrite) {
		t.Errorf("expected ErrSymlinkOverwrite in CreateFolder on existing symlink, got %v", err)
	}

	insideTargetDir := filepath.Join(tempDir, "real_inside")
	if err := os.Mkdir(insideTargetDir, 0700); err != nil {
		t.Fatalf("failed to create inside target dir: %v", err)
	}
	symlinkInside := filepath.Join(tempDir, "link_inside")
	if err := os.Symlink(insideTargetDir, symlinkInside); err != nil {
		t.Fatalf("failed to create internal symlink: %v", err)
	}

	resolvedInside, err := service.ResolvePath("link_inside")
	if err != nil {
		t.Errorf("expected internal symlink resolution to succeed, got %v", err)
	}
	if resolvedInside != insideTargetDir {
		t.Errorf("expected %q, got %q", insideTargetDir, resolvedInside)
	}

	resolvedNonExistentInside, err := service.ResolvePath("link_inside/new_file.txt")
	if err != nil {
		t.Errorf("expected non-existent file inside internal symlink to succeed, got %v", err)
	}
	expectedNestedPath := filepath.Join(insideTargetDir, "new_file.txt")
	if resolvedNonExistentInside != expectedNestedPath {
		t.Errorf("expected %q, got %q", expectedNestedPath, resolvedNonExistentInside)
	}

	secretContent, _ := os.ReadFile(outsideSecret)
	if string(secretContent) != "super-secret-data" {
		t.Errorf("outside file was modified! Content: %q", string(secretContent))
	}
}

func TestStorageService_GetDirectoryPage(t *testing.T) {
	tempDir := t.TempDir()
	service := storage.NewService(tempDir)

	if err := os.Mkdir(filepath.Join(tempDir, "docs"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(tempDir, "images"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(tempDir, "node_modules"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(tempDir, ".git"), 0700); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(tempDir, "docs", "manual.pdf"), []byte("pdf-content"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tempDir, "docs", "notes.txt"), []byte("notes"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tempDir, "alpha.txt"), []byte("alpha-content-long"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tempDir, "beta.go"), []byte("package main"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tempDir, ".env"), []byte("SECRET=123"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tempDir, "simplefs"), []byte("binary"), 0700); err != nil {
		t.Fatal(err)
	}

	t.Run("basic listing and restricted item exclusion", func(t *testing.T) {
		page, err := service.GetDirectoryPage("", "", "", "name", "asc", "en")
		if err != nil {
			t.Fatalf("GetDirectoryPage failed: %v", err)
		}

		if len(page.Folders) != 2 {
			t.Errorf("expected 2 folders, got %d", len(page.Folders))
		}
		if len(page.Files) != 2 {
			t.Errorf("expected 2 files, got %d", len(page.Files))
		}

		for _, folder := range page.Folders {
			if folder.Name == ".git" || folder.Name == "node_modules" {
				t.Errorf("found restricted folder in listing: %s", folder.Name)
			}
			if folder.Name == "docs" && folder.ItemCount != 2 {
				t.Errorf("expected docs item count 2, got %d", folder.ItemCount)
			}
		}

		for _, file := range page.Files {
			if file.Name == ".env" || file.Name == "simplefs" {
				t.Errorf("found restricted file in listing: %s", file.Name)
			}
		}

		if page.ViewMode != models.ViewModeList {
			t.Errorf("expected default ViewMode list, got %q", page.ViewMode)
		}
		if page.Lang != "en" {
			t.Errorf("expected lang en, got %q", page.Lang)
		}
	})

	t.Run("search query filtering", func(t *testing.T) {
		page, err := service.GetDirectoryPage("", "ALP", "", "name", "asc", "en")
		if err != nil {
			t.Fatalf("GetDirectoryPage failed: %v", err)
		}
		if len(page.Files) != 1 || page.Files[0].Name != "alpha.txt" {
			t.Errorf("expected only alpha.txt, got %+v", page.Files)
		}
		if len(page.Folders) != 0 {
			t.Errorf("expected 0 folders matching 'ALP', got %d", len(page.Folders))
		}

		noMatchPage, err := service.GetDirectoryPage("", "nonexistent_query", "", "name", "asc", "en")
		if err != nil {
			t.Fatalf("GetDirectoryPage failed: %v", err)
		}
		if len(noMatchPage.Files) != 0 || len(noMatchPage.Folders) != 0 {
			t.Errorf("expected empty page for unmatched query, got %d files, %d folders", len(noMatchPage.Files), len(noMatchPage.Folders))
		}
	})

	t.Run("sort combinations", func(t *testing.T) {
		sorts := []string{"name", "size", "created", "modified", "invalid_sort"}
		orders := []string{"asc", "desc", "invalid_order"}

		for _, sortBy := range sorts {
			for _, sortOrder := range orders {
				page, err := service.GetDirectoryPage("", "", "grid", sortBy, sortOrder, "es")
				if err != nil {
					t.Fatalf("GetDirectoryPage failed for sort=%s, order=%s: %v", sortBy, sortOrder, err)
				}
				if page.ViewMode != "grid" {
					t.Errorf("expected viewMode grid, got %q", page.ViewMode)
				}
				if page.Lang != "es" {
					t.Errorf("expected lang es, got %q", page.Lang)
				}
				if len(page.Folders) != 2 || len(page.Files) != 2 {
					t.Errorf("unexpected item counts for sort=%s order=%s", sortBy, sortOrder)
				}
			}
		}
	})

	t.Run("non existent directory error", func(t *testing.T) {
		_, err := service.GetDirectoryPage("missing_dir", "", "", "name", "asc", "en")
		if err == nil {
			t.Error("expected error for non-existent directory, got nil")
		}
	})

	t.Run("invalid path error in GetDirectoryPage", func(t *testing.T) {
		_, err := service.GetDirectoryPage("../outside", "", "", "name", "asc", "en")
		if !errors.Is(err, storage.ErrInvalidPath) {
			t.Errorf("expected ErrInvalidPath, got %v", err)
		}
	})
}

func TestStorageService_GetFileDetails(t *testing.T) {
	tempDir := t.TempDir()
	service := storage.NewService(tempDir)

	subDir := filepath.Join(tempDir, "subdir")
	if err := os.Mkdir(subDir, 0700); err != nil {
		t.Fatal(err)
	}

	rootFilePath := filepath.Join(tempDir, "hello.txt")
	if err := os.WriteFile(rootFilePath, []byte("Hello, World!"), 0600); err != nil {
		t.Fatal(err)
	}

	nestedFilePath := filepath.Join(subDir, "photo.png")
	if err := os.WriteFile(nestedFilePath, []byte("image-data"), 0600); err != nil {
		t.Fatal(err)
	}

	t.Run("root file details", func(t *testing.T) {
		details, err := service.GetFileDetails("hello.txt", "en")
		if err != nil {
			t.Fatalf("GetFileDetails failed: %v", err)
		}
		if details.Name != "hello.txt" {
			t.Errorf("expected name hello.txt, got %q", details.Name)
		}
		if details.DirLocation != "/" {
			t.Errorf("expected DirLocation '/', got %q", details.DirLocation)
		}
		if details.FormattedSize != "13 B" {
			t.Errorf("expected formatted size '13 B', got %q", details.FormattedSize)
		}
		if details.IsImage {
			t.Error("expected IsImage to be false for text file")
		}
		if details.Lang != "en" {
			t.Errorf("expected Lang 'en', got %q", details.Lang)
		}
	})

	t.Run("nested image file details with spanish language", func(t *testing.T) {
		details, err := service.GetFileDetails("subdir/photo.png", "es")
		if err != nil {
			t.Fatalf("GetFileDetails failed: %v", err)
		}
		if details.Name != "photo.png" {
			t.Errorf("expected name photo.png, got %q", details.Name)
		}
		if details.DirLocation != "/subdir" {
			t.Errorf("expected DirLocation '/subdir', got %q", details.DirLocation)
		}
		if !details.IsImage {
			t.Error("expected IsImage to be true for png file")
		}
		if details.Lang != "es" {
			t.Errorf("expected Lang 'es', got %q", details.Lang)
		}
	})

	t.Run("non existent file details", func(t *testing.T) {
		_, err := service.GetFileDetails("missing.txt", "en")
		if !errors.Is(err, storage.ErrNotFound) {
			t.Errorf("expected ErrNotFound for missing file, got %v", err)
		}
	})

	t.Run("invalid path details error", func(t *testing.T) {
		_, err := service.GetFileDetails("../escape.txt", "en")
		if !errors.Is(err, storage.ErrInvalidPath) {
			t.Errorf("expected ErrInvalidPath, got %v", err)
		}
	})
}

func TestStorageService_GetFilePreview(t *testing.T) {
	tempDir := t.TempDir()
	service := storage.NewService(tempDir)

	codeContent := "package main\n\nfunc main() {\n\tprintln(\"test\")\n}\n"
	codePath := filepath.Join(tempDir, "main.go")
	if err := os.WriteFile(codePath, []byte(codeContent), 0600); err != nil {
		t.Fatal(err)
	}

	mdContent := "# SimpleFS\nLightweight file manager.\n"
	mdPath := filepath.Join(tempDir, "README.md")
	if err := os.WriteFile(mdPath, []byte(mdContent), 0600); err != nil {
		t.Fatal(err)
	}

	binPath := filepath.Join(tempDir, "data.bin")
	if err := os.WriteFile(binPath, []byte{0x00, 0x01, 0x02, 0xFF}, 0600); err != nil {
		t.Fatal(err)
	}

	largeFilePath := filepath.Join(tempDir, "large.txt")
	largeContent := strings.Repeat("A", 1024*1024+10)
	if err := os.WriteFile(largeFilePath, []byte(largeContent), 0600); err != nil {
		t.Fatal(err)
	}

	dirPath := filepath.Join(tempDir, "somedir")
	if err := os.Mkdir(dirPath, 0700); err != nil {
		t.Fatal(err)
	}

	unreadablePath := filepath.Join(tempDir, "unreadable.go")
	if err := os.WriteFile(unreadablePath, []byte("package unreadable"), 0600); err != nil {
		t.Fatal(err)
	}
	_ = os.Chmod(unreadablePath, 0000)

	t.Run("code preview", func(t *testing.T) {
		preview, err := service.GetFilePreview("main.go", "en")
		if err != nil {
			t.Fatalf("GetFilePreview failed: %v", err)
		}
		if preview.Content != codeContent {
			t.Errorf("unexpected content: %q", preview.Content)
		}
		if preview.LineCount != 6 {
			t.Errorf("expected line count 6, got %d", preview.LineCount)
		}
		if preview.Extension != ".go" {
			t.Errorf("expected extension .go, got %q", preview.Extension)
		}
		if preview.LanguageClass == "" {
			t.Error("expected non-empty language class")
		}
	})

	t.Run("markdown preview", func(t *testing.T) {
		preview, err := service.GetFilePreview("README.md", "en")
		if err != nil {
			t.Fatalf("GetFilePreview failed: %v", err)
		}
		if preview.Content != mdContent {
			t.Errorf("unexpected markdown content: %q", preview.Content)
		}
		if preview.LineCount != 3 {
			t.Errorf("expected line count 3, got %d", preview.LineCount)
		}
	})

	t.Run("binary file preview has empty content", func(t *testing.T) {
		preview, err := service.GetFilePreview("data.bin", "en")
		if err != nil {
			t.Fatalf("GetFilePreview failed: %v", err)
		}
		if preview.Content != "" {
			t.Errorf("expected empty content for binary file, got %q", preview.Content)
		}
		if preview.LineCount != 0 {
			t.Errorf("expected 0 line count for binary file, got %d", preview.LineCount)
		}
	})

	t.Run("oversized file preview localized English", func(t *testing.T) {
		preview, err := service.GetFilePreview("large.txt", "en")
		if err != nil {
			t.Fatalf("GetFilePreview failed: %v", err)
		}
		if preview.Content != "[File too large for inline text preview]" {
			t.Errorf("unexpected oversized notice: %q", preview.Content)
		}
	})

	t.Run("oversized file preview localized Spanish", func(t *testing.T) {
		preview, err := service.GetFilePreview("large.txt", "es")
		if err != nil {
			t.Fatalf("GetFilePreview failed: %v", err)
		}
		if preview.Content != "[Archivo demasiado grande para vista previa en texto]" {
			t.Errorf("unexpected spanish oversized notice: %q", preview.Content)
		}
	})

	t.Run("unreadable file preview localized", func(t *testing.T) {
		// When running as non-root devuser, chmod 0000 causes os.ReadFile to fail
		previewEn, err := service.GetFilePreview("unreadable.go", "en")
		if err == nil && previewEn.Content == "[Error reading file]" {
			previewEs, err := service.GetFilePreview("unreadable.go", "es")
			if err != nil || previewEs.Content != "[Error al leer el archivo]" {
				t.Errorf("unexpected spanish error preview: %+v, err=%v", previewEs, err)
			}
		}
	})

	t.Run("directory preview returns ErrNotFound", func(t *testing.T) {
		_, err := service.GetFilePreview("somedir", "en")
		if !errors.Is(err, storage.ErrNotFound) {
			t.Errorf("expected ErrNotFound for directory preview, got %v", err)
		}
	})

	t.Run("non existent file returns ErrNotFound", func(t *testing.T) {
		_, err := service.GetFilePreview("does_not_exist.txt", "en")
		if !errors.Is(err, storage.ErrNotFound) {
			t.Errorf("expected ErrNotFound for missing file, got %v", err)
		}
	})

	t.Run("invalid path preview error", func(t *testing.T) {
		_, err := service.GetFilePreview("../outside.go", "en")
		if !errors.Is(err, storage.ErrInvalidPath) {
			t.Errorf("expected ErrInvalidPath, got %v", err)
		}
	})
}

func TestStorageService_CRUDOperations(t *testing.T) {
	tempDir := t.TempDir()
	service := storage.NewService(tempDir)

	t.Run("CreateFolder happy path and invalid names", func(t *testing.T) {
		if err := service.CreateFolder("", "documents"); err != nil {
			t.Fatalf("CreateFolder failed: %v", err)
		}
		if err := service.CreateFolder("documents", "reports"); err != nil {
			t.Fatalf("nested CreateFolder failed: %v", err)
		}

		invalidFolderNames := []string{"", "   ", "a/b", "a\\b", "..", ".git", "node_modules", "simplefs"}
		for _, name := range invalidFolderNames {
			if err := service.CreateFolder("", name); !errors.Is(err, storage.ErrInvalidName) {
				t.Errorf("expected ErrInvalidName for folder %q, got %v", name, err)
			}
		}

		if err := service.CreateFolder("missing_parent/sub", "valid"); !errors.Is(err, storage.ErrNotFound) {
			t.Errorf("expected ErrNotFound for non-existent parent directory, got %v", err)
		}

		if err := service.CreateFolder("../escape", "valid"); !errors.Is(err, storage.ErrInvalidPath) {
			t.Errorf("expected ErrInvalidPath for traversal parent, got %v", err)
		}
	})

	t.Run("CreateFile happy path and invalid names", func(t *testing.T) {
		content := []byte("report data")
		if err := service.CreateFile("documents/reports", "annual.csv", content); err != nil {
			t.Fatalf("CreateFile failed: %v", err)
		}

		createdPath := filepath.Join(tempDir, "documents", "reports", "annual.csv")
		readBytes, err := os.ReadFile(createdPath)
		if err != nil || string(readBytes) != "report data" {
			t.Errorf("unexpected file content: %s, err: %v", string(readBytes), err)
		}

		invalidFilenames := []string{"", "   ", "a/b", "a\\b", "..", ".env", "node_modules", "simplefs"}
		for _, name := range invalidFilenames {
			if err := service.CreateFile("", name, []byte("data")); !errors.Is(err, storage.ErrInvalidName) {
				t.Errorf("expected ErrInvalidName for filename %q, got %v", name, err)
			}
		}

		if err := service.CreateFile("missing_parent", "file.txt", []byte("data")); !errors.Is(err, storage.ErrNotFound) {
			t.Errorf("expected ErrNotFound for non-existent parent, got %v", err)
		}

		if err := service.CreateFile("../escape", "file.txt", []byte("data")); !errors.Is(err, storage.ErrInvalidPath) {
			t.Errorf("expected ErrInvalidPath for traversal parent, got %v", err)
		}
	})

	t.Run("SaveUploadedFile happy path and invalid names", func(t *testing.T) {
		payload := "uploaded content payload"
		if err := service.SaveUploadedFile("documents", "upload.txt", strings.NewReader(payload)); err != nil {
			t.Fatalf("SaveUploadedFile failed: %v", err)
		}

		uploadedPath := filepath.Join(tempDir, "documents", "upload.txt")
		readBytes, err := os.ReadFile(uploadedPath)
		if err != nil || string(readBytes) != payload {
			t.Errorf("unexpected uploaded content: %s, err: %v", string(readBytes), err)
		}

		invalidUploadNames := []string{"", "   ", "../../evil.sh", ".bashrc", "simplefs", "node_modules"}
		for _, name := range invalidUploadNames {
			if err := service.SaveUploadedFile("", name, strings.NewReader("payload")); !errors.Is(err, storage.ErrInvalidName) {
				t.Errorf("expected ErrInvalidName for upload %q, got %v", name, err)
			}
		}

		if err := service.SaveUploadedFile("missing_dir", "valid.txt", strings.NewReader("payload")); !errors.Is(err, storage.ErrNotFound) {
			t.Errorf("expected ErrNotFound for non-existent upload target dir, got %v", err)
		}

		if err := service.SaveUploadedFile("../escape", "valid.txt", strings.NewReader("payload")); !errors.Is(err, storage.ErrInvalidPath) {
			t.Errorf("expected ErrInvalidPath for traversal upload dir, got %v", err)
		}
	})

	t.Run("DeleteItem operations and protection", func(t *testing.T) {
		toDeleteFile := filepath.Join(tempDir, "delete_me.txt")
		if err := os.WriteFile(toDeleteFile, []byte("delete"), 0600); err != nil {
			t.Fatal(err)
		}

		if err := service.DeleteItem("delete_me.txt"); err != nil {
			t.Fatalf("DeleteItem on file failed: %v", err)
		}
		if _, err := os.Stat(toDeleteFile); !os.IsNotExist(err) {
			t.Error("file still exists after DeleteItem")
		}

		toDeleteDir := filepath.Join(tempDir, "delete_dir")
		if err := os.Mkdir(toDeleteDir, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(toDeleteDir, "nested.txt"), []byte("sub"), 0600); err != nil {
			t.Fatal(err)
		}

		if err := service.DeleteItem("delete_dir"); err != nil {
			t.Fatalf("DeleteItem on directory failed: %v", err)
		}
		if _, err := os.Stat(toDeleteDir); !os.IsNotExist(err) {
			t.Error("directory still exists after DeleteItem")
		}

		if err := service.DeleteItem(""); !errors.Is(err, storage.ErrCannotDeleteRoot) {
			t.Errorf("expected ErrCannotDeleteRoot for empty path, got %v", err)
		}
		if err := service.DeleteItem("."); !errors.Is(err, storage.ErrCannotDeleteRoot) {
			t.Errorf("expected ErrCannotDeleteRoot for '.', got %v", err)
		}
		if err := service.DeleteItem("/"); !errors.Is(err, storage.ErrCannotDeleteRoot) {
			t.Errorf("expected ErrCannotDeleteRoot for '/', got %v", err)
		}

		protectedDeletions := []string{".git", "node_modules", "simplefs", ".stitch", ".dc_simplefs"}
		for _, name := range protectedDeletions {
			if err := service.DeleteItem(name); !errors.Is(err, storage.ErrProtectedItem) {
				t.Errorf("expected ErrProtectedItem for %q, got %v", name, err)
			}
		}

		if err := service.DeleteItem("../outside"); !errors.Is(err, storage.ErrInvalidPath) {
			t.Errorf("expected ErrInvalidPath for DeleteItem with traversal, got %v", err)
		}
	})
}

func TestStorageService_GetDownloadFile(t *testing.T) {
	tempDir := t.TempDir()
	service := storage.NewService(tempDir)

	htmlPath := filepath.Join(tempDir, "page.html")
	if err := os.WriteFile(htmlPath, []byte("<html></html>"), 0600); err != nil {
		t.Fatal(err)
	}

	svgPath := filepath.Join(tempDir, "icon.svg")
	if err := os.WriteFile(svgPath, []byte("<svg></svg>"), 0600); err != nil {
		t.Fatal(err)
	}

	pdfPath := filepath.Join(tempDir, "document.pdf")
	if err := os.WriteFile(pdfPath, []byte("%PDF-1.4"), 0600); err != nil {
		t.Fatal(err)
	}

	dirPath := filepath.Join(tempDir, "folder")
	if err := os.Mkdir(dirPath, 0700); err != nil {
		t.Fatal(err)
	}

	t.Run("dangerous extensions force attachment", func(t *testing.T) {
		absPath, filename, mimeType, isForced, err := service.GetDownloadFile("page.html")
		if err != nil {
			t.Fatalf("GetDownloadFile failed: %v", err)
		}
		if absPath != htmlPath || filename != "page.html" || !isForced || mimeType == "" {
			t.Errorf("unexpected download values: abs=%s, name=%s, forced=%v, mime=%s", absPath, filename, isForced, mimeType)
		}

		_, _, _, isSvgForced, err := service.GetDownloadFile("icon.svg")
		if err != nil || !isSvgForced {
			t.Errorf("expected svg to force attachment, err: %v, forced: %v", err, isSvgForced)
		}
	})

	t.Run("safe extensions do not force attachment", func(t *testing.T) {
		_, filename, _, isForced, err := service.GetDownloadFile("document.pdf")
		if err != nil {
			t.Fatalf("GetDownloadFile failed: %v", err)
		}
		if filename != "document.pdf" || isForced {
			t.Errorf("expected document.pdf forced=false, got %v", isForced)
		}
	})

	t.Run("directory download returns ErrNotFound", func(t *testing.T) {
		_, _, _, _, err := service.GetDownloadFile("folder")
		if !errors.Is(err, storage.ErrNotFound) {
			t.Errorf("expected ErrNotFound for directory download, got %v", err)
		}
	})

	t.Run("missing file download returns ErrNotFound", func(t *testing.T) {
		_, _, _, _, err := service.GetDownloadFile("missing.pdf")
		if !errors.Is(err, storage.ErrNotFound) {
			t.Errorf("expected ErrNotFound for missing file download, got %v", err)
		}
	})

	t.Run("invalid path download returns ErrInvalidPath", func(t *testing.T) {
		_, _, _, _, err := service.GetDownloadFile("../outside.pdf")
		if !errors.Is(err, storage.ErrInvalidPath) {
			t.Errorf("expected ErrInvalidPath for traversal download, got %v", err)
		}
	})
}

func TestStorageService_BuildBreadcrumbs(t *testing.T) {
	tests := []struct {
		input    string
		expected []models.Breadcrumb
	}{
		{"", nil},
		{".", nil},
		{"/", nil},
		{"   ", nil},
		{"docs", []models.Breadcrumb{
			{Name: "docs", Path: "docs"},
		}},
		{"docs/api/v1", []models.Breadcrumb{
			{Name: "docs", Path: "docs"},
			{Name: "api", Path: "docs/api"},
			{Name: "v1", Path: "docs/api/v1"},
		}},
		{"docs\\api\\v1", []models.Breadcrumb{
			{Name: "docs", Path: "docs"},
			{Name: "api", Path: "docs/api"},
			{Name: "v1", Path: "docs/api/v1"},
		}},
		{"/docs/api/", []models.Breadcrumb{
			{Name: "docs", Path: "docs"},
			{Name: "api", Path: "docs/api"},
		}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			actual := storage.BuildBreadcrumbs(tt.input)
			if len(actual) != len(tt.expected) {
				t.Fatalf("for input %q: expected %d crumbs, got %d (%+v)", tt.input, len(tt.expected), len(actual), actual)
			}
			for i := range actual {
				if actual[i].Name != tt.expected[i].Name || actual[i].Path != tt.expected[i].Path {
					t.Errorf("crumb %d mismatch: got %+v, want %+v", i, actual[i], tt.expected[i])
				}
			}
		})
	}
}

func TestStorageService_FormatBytes(t *testing.T) {
	tests := []struct {
		bytesCount int64
		expected   string
	}{
		{-100, "0 B"},
		{0, "0 B"},
		{1, "1 B"},
		{500, "500 B"},
		{1023, "1023 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
		{1073741824, "1.0 GB"},
		{1099511627776, "1.0 TB"},
		{1125899906842624, "1.0 PB"},
		{1152921504606846976, "1.0 EB"},
	}

	for _, tt := range tests {
		actual := storage.FormatBytes(tt.bytesCount)
		if actual != tt.expected {
			t.Errorf("FormatBytes(%d): got %q, want %q", tt.bytesCount, actual, tt.expected)
		}
	}
}

func TestStorageService_SortingAndComparators(t *testing.T) {
	tempDir := t.TempDir()
	service := storage.NewService(tempDir)

	now := time.Now()
	folders := []string{"folder_b", "folder_a", "folder_c"}
	for _, f := range folders {
		if err := os.Mkdir(filepath.Join(tempDir, f), 0700); err != nil {
			t.Fatal(err)
		}
	}

	if err := os.WriteFile(filepath.Join(tempDir, "folder_a", "f1.txt"), []byte("1"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tempDir, "folder_a", "f2.txt"), []byte("2"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tempDir, "folder_b", "f1.txt"), []byte("1"), 0600); err != nil {
		t.Fatal(err)
	}

	files := []struct {
		name string
		size int
		time time.Time
	}{
		{"file_b.txt", 300, now.Add(-2 * time.Hour)},
		{"file_a.txt", 100, now.Add(-1 * time.Hour)},
		{"file_c.txt", 200, now},
	}
	for _, f := range files {
		p := filepath.Join(tempDir, f.name)
		if err := os.WriteFile(p, make([]byte, f.size), 0600); err != nil {
			t.Fatal(err)
		}
		_ = os.Chtimes(p, f.time, f.time)
	}

	t.Run("folder sort by size desc", func(t *testing.T) {
		page, err := service.GetDirectoryPage("", "", "list", "size", "desc", "en")
		if err != nil {
			t.Fatalf("GetDirectoryPage failed: %v", err)
		}
		if page.Folders[0].Name != "folder_a" || page.Folders[0].ItemCount != 2 {
			t.Errorf("expected first folder folder_a (count 2), got %s (count %d)", page.Folders[0].Name, page.Folders[0].ItemCount)
		}
		if page.Folders[1].Name != "folder_b" || page.Folders[1].ItemCount != 1 {
			t.Errorf("expected second folder folder_b (count 1), got %s (count %d)", page.Folders[1].Name, page.Folders[1].ItemCount)
		}
		if page.Folders[2].Name != "folder_c" || page.Folders[2].ItemCount != 0 {
			t.Errorf("expected third folder folder_c (count 0), got %s (count %d)", page.Folders[2].Name, page.Folders[2].ItemCount)
		}
	})

	t.Run("folder sort by name asc and desc", func(t *testing.T) {
		pageAsc, err := service.GetDirectoryPage("", "", "list", "name", "asc", "en")
		if err != nil {
			t.Fatalf("GetDirectoryPage failed: %v", err)
		}
		if pageAsc.Folders[0].Name != "folder_a" || pageAsc.Folders[2].Name != "folder_c" {
			t.Errorf("unexpected folder name asc sort: %+v", pageAsc.Folders)
		}

		pageDesc, err := service.GetDirectoryPage("", "", "list", "name", "desc", "en")
		if err != nil {
			t.Fatalf("GetDirectoryPage failed: %v", err)
		}
		if pageDesc.Folders[0].Name != "folder_c" || pageDesc.Folders[2].Name != "folder_a" {
			t.Errorf("unexpected folder name desc sort: %+v", pageDesc.Folders)
		}
	})

	t.Run("file sort by size asc", func(t *testing.T) {
		page, err := service.GetDirectoryPage("", "", "list", "size", "asc", "en")
		if err != nil {
			t.Fatalf("GetDirectoryPage failed: %v", err)
		}
		if page.Files[0].Name != "file_a.txt" || page.Files[1].Name != "file_c.txt" || page.Files[2].Name != "file_b.txt" {
			t.Errorf("unexpected file sort by size asc: %+v", page.Files)
		}
	})

	t.Run("file sort by modified asc and desc", func(t *testing.T) {
		pageAsc, err := service.GetDirectoryPage("", "", "list", "modified", "asc", "en")
		if err != nil {
			t.Fatalf("GetDirectoryPage failed: %v", err)
		}
		if pageAsc.Files[0].Name != "file_b.txt" || pageAsc.Files[2].Name != "file_c.txt" {
			t.Errorf("unexpected modified asc sort: %+v", pageAsc.Files)
		}

		pageDesc, err := service.GetDirectoryPage("", "", "list", "created", "desc", "en")
		if err != nil {
			t.Fatalf("GetDirectoryPage failed: %v", err)
		}
		if pageDesc.Files[0].Name != "file_c.txt" || pageDesc.Files[2].Name != "file_b.txt" {
			t.Errorf("unexpected created desc sort: %+v", pageDesc.Files)
		}
	})
}

func TestStorageService_Concurrency(t *testing.T) {
	tempDir := t.TempDir()
	service := storage.NewService(tempDir)

	var wg sync.WaitGroup
	workers := 10
	iterations := 20

	for i := 0; i < workers; i++ {
		wg.Add(1)
		workerID := i
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				folderName := "worker_folder"
				_ = service.CreateFolder("", folderName)

				filename := "concurrent_test.txt"
				_ = service.CreateFile(folderName, filename, []byte("data"))

				_, _ = service.GetDirectoryPage(folderName, "", "list", "name", "asc", "en")
				_, _ = service.GetFileDetails(filepath.Join(folderName, filename), "en")
				_, _ = service.GetFilePreview(filepath.Join(folderName, filename), "en")
				_, _, _, _, _ = service.GetDownloadFile(filepath.Join(folderName, filename))

				if workerID%2 == 0 {
					_ = service.SaveUploadedFile(folderName, "stream.txt", strings.NewReader("streamed"))
				}
			}
		}()
	}

	wg.Wait()
}
