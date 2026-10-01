package storage_test

import (
	"archive/zip"
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"simplefs/internal/storage"
)

// createTestZipBuffer builds a zip archive in memory with given files and contents.
func createTestZipBuffer(t *testing.T, files map[string]string) *bytes.Buffer {
	t.Helper()
	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)

	for name, content := range files {
		f, err := zw.Create(name)
		if err != nil {
			t.Fatalf("failed to create zip entry %s: %v", name, err)
		}
		if _, err := f.Write([]byte(content)); err != nil {
			t.Fatalf("failed to write content for %s: %v", name, err)
		}
	}

	if err := zw.Close(); err != nil {
		t.Fatalf("failed to close zip writer: %v", err)
	}
	return buf
}

func TestZip_ValidExtraction(t *testing.T) {
	tempDir := t.TempDir()
	svc := storage.NewService(tempDir)

	files := map[string]string{
		"hello.txt":               "Hello World",
		"subfolder/nested.txt":    "Nested content",
		"subfolder/deep/more.txt": "Deep content",
	}

	zipBuf := createTestZipBuffer(t, files)

	folderName, err := svc.ExtractZipFile("", "my_project.zip", zipBuf)
	if err != nil {
		t.Fatalf("expected successful extraction, got %v", err)
	}

	if folderName != "my_project" {
		t.Errorf("expected folderName 'my_project', got %q", folderName)
	}

	extractedDir := filepath.Join(tempDir, folderName)

	// Verify all files were created
	for relPath, expectedContent := range files {
		fullPath := filepath.Join(extractedDir, filepath.FromSlash(relPath))
		data, err := os.ReadFile(fullPath)
		if err != nil {
			t.Errorf("missing extracted file %s: %v", fullPath, err)
			continue
		}
		if string(data) != expectedContent {
			t.Errorf("content mismatch for %s: expected %q, got %q", relPath, expectedContent, string(data))
		}
	}
}

func TestZip_CollisionHandling(t *testing.T) {
	tempDir := t.TempDir()
	svc := storage.NewService(tempDir)

	files := map[string]string{"file.txt": "data"}

	zipBuf1 := createTestZipBuffer(t, files)
	name1, err := svc.ExtractZipFile("", "docs.zip", zipBuf1)
	if err != nil || name1 != "docs" {
		t.Fatalf("first extract failed: %v", err)
	}

	zipBuf2 := createTestZipBuffer(t, files)
	name2, err := svc.ExtractZipFile("", "docs.zip", zipBuf2)
	if err != nil || name2 != "docs_1" {
		t.Fatalf("second extract expected 'docs_1', got %q, err: %v", name2, err)
	}
}

func TestZip_PathTraversalZipSlip(t *testing.T) {
	traversalCases := []struct {
		name      string
		entryPath string
	}{
		{"dot dot traversal", "../../evil.sh"},
		{"nested dot dot", "folder/../../evil.sh"},
		{"absolute unix path", "/etc/passwd"},
		{"windows backslash traversal", "..\\..\\evil.bat"},
		{"windows drive letter", "C:/evil.exe"},
		{"windows drive with backslash", "C:\\evil.exe"},
	}

	for _, tc := range traversalCases {
		t.Run(tc.name, func(t *testing.T) {
			tempDir := t.TempDir()
			svc := storage.NewService(tempDir)

			buf := new(bytes.Buffer)
			zw := zip.NewWriter(buf)
			w, err := zw.Create(tc.entryPath)
			if err == nil {
				_, _ = w.Write([]byte("malicious content"))
			}
			_ = zw.Close()

			_, err = svc.ExtractZipFile("", "attack.zip", buf)
			if !errors.Is(err, storage.ErrZipPathTraversal) {
				t.Fatalf("expected ErrZipPathTraversal for %q, got %v", tc.entryPath, err)
			}

			// Verify rollback: no folder was left behind
			entries, _ := os.ReadDir(tempDir)
			if len(entries) != 0 {
				t.Fatalf("expected empty directory after rollback, found %d entries", len(entries))
			}
		})
	}
}

func TestZip_SymlinkForbidden(t *testing.T) {
	tempDir := t.TempDir()
	svc := storage.NewService(tempDir)

	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)

	header := &zip.FileHeader{
		Name: "symlink_entry",
	}
	header.SetMode(os.ModeSymlink | 0777)
	w, err := zw.CreateHeader(header)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = w.Write([]byte("/etc/passwd"))
	_ = zw.Close()

	_, err = svc.ExtractZipFile("", "symlink.zip", buf)
	if !errors.Is(err, storage.ErrZipSymlinkForbidden) {
		t.Fatalf("expected ErrZipSymlinkForbidden, got %v", err)
	}
}

func TestZip_SpecialDeviceForbidden(t *testing.T) {
	tempDir := t.TempDir()
	svc := storage.NewService(tempDir)

	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)

	header := &zip.FileHeader{
		Name: "device_entry",
	}
	header.SetMode(os.ModeDevice | 0666)
	_, _ = zw.CreateHeader(header)
	_ = zw.Close()

	_, err := svc.ExtractZipFile("", "device.zip", buf)
	if !errors.Is(err, storage.ErrZipSymlinkForbidden) {
		t.Fatalf("expected ErrZipSymlinkForbidden for device file, got %v", err)
	}
}

func TestZip_RestrictedSegment(t *testing.T) {
	restrictedCases := []string{
		".git/config",
		"nested/.env",
		"app/node_modules/pkg/index.js",
		"simplefs",
	}

	for _, entryName := range restrictedCases {
		t.Run(entryName, func(t *testing.T) {
			tempDir := t.TempDir()
			svc := storage.NewService(tempDir)

			buf := createTestZipBuffer(t, map[string]string{entryName: "secret"})
			_, err := svc.ExtractZipFile("", "restricted.zip", buf)
			if !errors.Is(err, storage.ErrZipRestrictedSegment) {
				t.Fatalf("expected ErrZipRestrictedSegment for %s, got %v", entryName, err)
			}
		})
	}
}

func TestZip_NullByteInFilename(t *testing.T) {
	tempDir := t.TempDir()
	svc := storage.NewService(tempDir)

	buf := createTestZipBuffer(t, map[string]string{"evil\x00file.txt": "bad"})
	_, err := svc.ExtractZipFile("", "nullbyte.zip", buf)
	if !errors.Is(err, storage.ErrZipInvalidName) {
		t.Fatalf("expected ErrZipInvalidName for null byte, got %v", err)
	}
}

func TestZip_SingleFileHeaderSizeLimit(t *testing.T) {
	file := &zip.File{
		FileHeader: zip.FileHeader{
			Name:               "huge.bin",
			UncompressedSize64: storage.MaxZipSingleFileBytes + 100,
		},
	}
	file.SetMode(0644)

	err := storage.ValidateZipEntryHeader(file)
	if !errors.Is(err, storage.ErrZipSingleFileSizeLimit) {
		t.Fatalf("expected ErrZipSingleFileSizeLimit, got %v", err)
	}
}

func TestZip_TotalSizeLimit(t *testing.T) {
	zr := &zip.Reader{
		File: []*zip.File{
			{
				FileHeader: zip.FileHeader{
					Name:               "file1.bin",
					UncompressedSize64: 150 * 1024 * 1024,
				},
			},
			{
				FileHeader: zip.FileHeader{
					Name:               "file2.bin",
					UncompressedSize64: 150 * 1024 * 1024,
				},
			},
			{
				FileHeader: zip.FileHeader{
					Name:               "file3.bin",
					UncompressedSize64: 150 * 1024 * 1024,
				},
			},
			{
				FileHeader: zip.FileHeader{
					Name:               "file4.bin",
					UncompressedSize64: 150 * 1024 * 1024, // 4 * 150 = 600MB > 500MB
				},
			},
		},
	}
	for _, f := range zr.File {
		f.SetMode(0644)
	}

	err := storage.ValidateZipReader(zr)
	if !errors.Is(err, storage.ErrZipTotalSizeLimit) {
		t.Fatalf("expected ErrZipTotalSizeLimit, got %v", err)
	}
}

func TestZip_EmptyArchive(t *testing.T) {
	tempDir := t.TempDir()
	svc := storage.NewService(tempDir)

	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)
	_ = zw.Close()

	_, err := svc.ExtractZipFile("", "empty.zip", buf)
	if !errors.Is(err, storage.ErrZipEmpty) {
		t.Fatalf("expected ErrZipEmpty, got %v", err)
	}
}

func TestZip_ExtractExistingZip(t *testing.T) {
	tempDir := t.TempDir()
	svc := storage.NewService(tempDir)

	zipBuf := createTestZipBuffer(t, map[string]string{
		"config.json": `{"test": true}`,
	})

	zipFilePath := filepath.Join(tempDir, "existing.zip")
	if err := os.WriteFile(zipFilePath, zipBuf.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}

	folderName, err := svc.ExtractExistingZip("", "existing.zip")
	if err != nil {
		t.Fatalf("failed to extract existing zip: %v", err)
	}

	extractedConfig := filepath.Join(tempDir, folderName, "config.json")
	data, err := os.ReadFile(extractedConfig)
	if err != nil {
		t.Fatalf("missing extracted file: %v", err)
	}
	if string(data) != `{"test": true}` {
		t.Errorf("content mismatch: got %q", string(data))
	}
}

func TestZip_DownloadFolderAsZip(t *testing.T) {
	tempDir := t.TempDir()
	svc := storage.NewService(tempDir)

	// Create test folder structure
	folderPath := filepath.Join(tempDir, "sample_folder")
	if err := os.MkdirAll(filepath.Join(folderPath, "sub"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(folderPath, "one.txt"), []byte("content 1"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(folderPath, "sub", "two.txt"), []byte("content 2"), 0644); err != nil {
		t.Fatal(err)
	}

	absPath, archiveName, err := svc.GetFolderDownloadInfo("sample_folder")
	if err != nil {
		t.Fatalf("failed to get folder download info: %v", err)
	}

	if archiveName != "sample_folder.zip" {
		t.Errorf("expected archive name 'sample_folder.zip', got %q", archiveName)
	}

	buf := new(bytes.Buffer)
	if err := svc.StreamFolderZip(absPath, buf); err != nil {
		t.Fatalf("failed to stream folder zip: %v", err)
	}

	// Read and verify the streamed zip
	reader, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("invalid generated zip: %v", err)
	}

	foundFiles := make(map[string]string)
	for _, f := range reader.File {
		if f.Mode().IsDir() {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		content, _ := io.ReadAll(rc)
		rc.Close()
		foundFiles[f.Name] = string(content)
	}

	if foundFiles["one.txt"] != "content 1" {
		t.Errorf("missing or incorrect one.txt: got %q", foundFiles["one.txt"])
	}
	if foundFiles["sub/two.txt"] != "content 2" {
		t.Errorf("missing or incorrect sub/two.txt: got %q", foundFiles["sub/two.txt"])
	}
}
