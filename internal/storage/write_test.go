package storage_test

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"simplefs/internal/storage"
)

func TestWriteFile_ConflictPolicies(t *testing.T) {
	tempDir := t.TempDir()
	svc := storage.NewService(tempDir)

	first, err := svc.WriteFile("inbox", "report.txt", strings.NewReader("v1"), storage.WriteOptions{Conflict: storage.ConflictFail, CreateParents: true})
	if err != nil {
		t.Fatalf("first write failed: %v", err)
	}
	sum := sha256.Sum256([]byte("v1"))
	if first.Path != "inbox/report.txt" || first.Size != 2 || first.SHA256 != hex.EncodeToString(sum[:]) {
		t.Errorf("unexpected write result: %+v", first)
	}

	if _, err := svc.WriteFile("inbox", "report.txt", strings.NewReader("v2"), storage.WriteOptions{Conflict: storage.ConflictFail}); !errors.Is(err, storage.ErrAlreadyExists) {
		t.Errorf("expected ErrAlreadyExists, got %v", err)
	}

	renamed, err := svc.WriteFile("inbox", "report.txt", strings.NewReader("v2"), storage.WriteOptions{Conflict: storage.ConflictRename})
	if err != nil || renamed.Name != "report (1).txt" || !renamed.Renamed {
		t.Errorf("expected rename to 'report (1).txt', got %+v, %v", renamed, err)
	}

	overwritten, err := svc.WriteFile("inbox", "report.txt", strings.NewReader("v3"), storage.WriteOptions{Conflict: storage.ConflictOverwrite})
	if err != nil || !overwritten.Overwritten {
		t.Fatalf("expected overwrite, got %+v, %v", overwritten, err)
	}
	content, _ := os.ReadFile(filepath.Join(tempDir, "inbox", "report.txt"))
	if string(content) != "v3" {
		t.Errorf("expected overwritten content v3, got %q", content)
	}

	trash, err := svc.GetTrashPage("", "", "", "en")
	if err != nil {
		t.Fatal(err)
	}
	if len(trash.Files) != 1 || trash.Files[0].TrashOriginalPath != "inbox/report.txt" {
		t.Errorf("expected previous version in trash, got %+v", trash.Files)
	}

	if err := os.Mkdir(filepath.Join(tempDir, "inbox", "folder"), 0755); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.WriteFile("inbox", "folder", strings.NewReader("x"), storage.WriteOptions{Conflict: storage.ConflictOverwrite}); !errors.Is(err, storage.ErrIsDirectory) {
		t.Errorf("expected ErrIsDirectory when overwriting a folder, got %v", err)
	}
}

type failingReader struct{ sent bool }

func (f *failingReader) Read(p []byte) (int, error) {
	if f.sent {
		return 0, errors.New("connection reset")
	}
	f.sent = true
	return copy(p, "partial data"), nil
}

func TestWriteFile_IsAtomic(t *testing.T) {
	tempDir := t.TempDir()
	svc := storage.NewService(tempDir)

	if _, err := svc.WriteFile("", "broken.bin", &failingReader{}, storage.WriteOptions{}); err == nil {
		t.Fatal("expected interrupted upload to fail")
	}
	if _, err := os.Stat(filepath.Join(tempDir, "broken.bin")); !os.IsNotExist(err) {
		t.Error("interrupted upload left a partial file in place")
	}
	leftovers, _ := os.ReadDir(filepath.Join(tempDir, ".simplefs", "tmp"))
	if len(leftovers) != 0 {
		t.Errorf("expected temporary files to be cleaned up, found %d", len(leftovers))
	}
	page, err := svc.GetDirectoryPage("", "", "", "", "", "en")
	if err != nil || page.HasItems() {
		t.Errorf("expected empty visible listing, got %+v (err %v)", page, err)
	}
}

func TestWriteFile_UsesConfiguredModes(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permissions only")
	}
	tempDir := t.TempDir()
	svc := storage.NewServiceWithOptions(tempDir, storage.Options{FileMode: 0640, DirMode: 0750})
	if _, err := svc.WriteFile("a/b", "file.txt", strings.NewReader("x"), storage.WriteOptions{CreateParents: true}); err != nil {
		t.Fatal(err)
	}
	fileInfo, _ := os.Stat(filepath.Join(tempDir, "a", "b", "file.txt"))
	if fileInfo.Mode().Perm() != 0640 {
		t.Errorf("expected file mode 0640, got %v", fileInfo.Mode().Perm())
	}
}

func TestCreateFolderPath(t *testing.T) {
	tempDir := t.TempDir()
	svc := storage.NewService(tempDir)
	if err := svc.CreateFolderPath("one/two/three"); err != nil {
		t.Fatalf("CreateFolderPath failed: %v", err)
	}
	if info, err := os.Stat(filepath.Join(tempDir, "one", "two", "three")); err != nil || !info.IsDir() {
		t.Errorf("expected nested folders, err %v", err)
	}
	if err := svc.CreateFolderPath("one/.trash"); !errors.Is(err, storage.ErrInvalidName) {
		t.Errorf("expected ErrInvalidName for reserved segment, got %v", err)
	}
	if err := svc.CreateFolderPath("../escape"); err == nil {
		t.Error("expected traversal to fail")
	}
}

func TestListStatSearchRecent(t *testing.T) {
	tempDir := t.TempDir()
	svc := storage.NewService(tempDir)

	write := func(path, content string, age time.Duration) {
		full := filepath.Join(tempDir, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
		modified := time.Now().Add(-age)
		_ = os.Chtimes(full, modified, modified)
	}
	write("outbox/old.txt", "old", 2*time.Hour)
	write("outbox/new.txt", "new", time.Minute)
	write("inbox/deep/report.pdf", "%PDF", time.Hour)

	self, entries, err := svc.List("outbox")
	if err != nil || self.Type != "dir" || len(entries) != 2 {
		t.Fatalf("unexpected listing: %+v %+v %v", self, entries, err)
	}

	entry, err := svc.Stat("outbox/new.txt", true)
	sum := sha256.Sum256([]byte("new"))
	if err != nil || entry.SHA256 != hex.EncodeToString(sum[:]) || entry.Type != "file" {
		t.Errorf("unexpected stat: %+v %v", entry, err)
	}

	results, _, err := svc.Search("", "report", 10)
	if err != nil || len(results) != 1 || results[0].Path != "inbox/deep/report.pdf" {
		t.Errorf("unexpected search results: %+v %v", results, err)
	}

	recent, _, err := svc.Recent(2)
	if err != nil || len(recent) != 2 || recent[0].Path != "outbox/new.txt" || recent[1].Path != "inbox/deep/report.pdf" {
		t.Errorf("unexpected recent order: %+v %v", recent, err)
	}
}

func TestStampChangesWithContent(t *testing.T) {
	tempDir := t.TempDir()
	svc := storage.NewService(tempDir)

	before, err := svc.Stamp("dir", "")
	if err != nil {
		t.Fatal(err)
	}
	same, _ := svc.Stamp("dir", "")
	if before != same {
		t.Error("stamp should be stable without changes")
	}
	if _, err := svc.WriteFile("", "new.txt", strings.NewReader("x"), storage.WriteOptions{}); err != nil {
		t.Fatal(err)
	}
	after, _ := svc.Stamp("dir", "")
	if before == after {
		t.Error("stamp should change after a new file")
	}

	recentBefore, _ := svc.Stamp("recent", "")
	if _, err := svc.WriteFile("deep/er", "x.txt", strings.NewReader("x"), storage.WriteOptions{CreateParents: true}); err != nil {
		t.Fatal(err)
	}
	recentAfter, _ := svc.Stamp("recent", "")
	if recentBefore == recentAfter {
		t.Error("recent stamp should change after a nested write")
	}

	if _, err := svc.Stamp("dir", "missing"); !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestTrash_RestoresDotfiles(t *testing.T) {
	tempDir := t.TempDir()
	svc := storage.NewService(tempDir)
	if err := svc.CreateFile("", ".env", []byte("KEY=1")); err != nil {
		t.Fatal(err)
	}
	if err := svc.DeleteItem(".env"); err != nil {
		t.Fatal(err)
	}
	trash, err := svc.GetTrashPage("", "", "", "en")
	if err != nil || len(trash.Files) != 1 || trash.Files[0].Name != ".env" {
		t.Fatalf("expected .env in trash, got %+v %v", trash.Files, err)
	}
	if err := svc.RestoreFromTrash(".env"); err != nil {
		t.Fatalf("restore failed: %v", err)
	}
	file, err := os.Open(filepath.Join(tempDir, ".env"))
	if err != nil {
		t.Fatalf("expected restored .env: %v", err)
	}
	defer file.Close()
	if data, _ := io.ReadAll(file); string(data) != "KEY=1" {
		t.Errorf("unexpected restored content %q", data)
	}
	if err := svc.RestoreFromTrash(".meta.json"); !errors.Is(err, storage.ErrInvalidName) {
		t.Errorf("expected metadata file to be protected, got %v", err)
	}
}
