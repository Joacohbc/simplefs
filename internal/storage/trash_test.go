package storage_test

import (
	"os"
	"path/filepath"
	"testing"

	"simplefs/internal/storage"
)

func TestStorage_TrashOperations(t *testing.T) {
	tempDir := t.TempDir()
	svc := storage.NewService(tempDir)

	// Create test files and folders
	if err := svc.CreateFolder("", "testfolder"); err != nil {
		t.Fatalf("failed to create folder: %v", err)
	}
	if err := svc.CreateFile("", "testfile.txt", []byte("hello world")); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}
	if err := svc.CreateFile("testfolder", "nested.txt", []byte("nested")); err != nil {
		t.Fatalf("failed to create nested file: %v", err)
	}

	t.Run("move file and folder to trash", func(t *testing.T) {
		if err := svc.DeleteItem("testfile.txt"); err != nil {
			t.Fatalf("failed to move testfile.txt to trash: %v", err)
		}
		if err := svc.DeleteItem("testfolder"); err != nil {
			t.Fatalf("failed to move testfolder to trash: %v", err)
		}

		// Verify files are no longer in root
		if _, err := os.Stat(filepath.Join(tempDir, "testfile.txt")); !os.IsNotExist(err) {
			t.Errorf("testfile.txt still exists in root")
		}
		if _, err := os.Stat(filepath.Join(tempDir, "testfolder")); !os.IsNotExist(err) {
			t.Errorf("testfolder still exists in root")
		}

		// Verify trash page contains items
		page, err := svc.GetTrashPage("list", "name", "asc", "es")
		if err != nil {
			t.Fatalf("failed to get trash page: %v", err)
		}
		if !page.IsTrash {
			t.Errorf("expected IsTrash=true, got false")
		}
		if len(page.Files) != 1 {
			t.Errorf("expected 1 file in trash, got %d", len(page.Files))
		}
		if len(page.Folders) != 1 {
			t.Errorf("expected 1 folder in trash, got %d", len(page.Folders))
		}
		if page.Files[0].TrashOriginalPath != "testfile.txt" {
			t.Errorf("expected TrashOriginalPath='testfile.txt', got '%s'", page.Files[0].TrashOriginalPath)
		}
	})

	t.Run("restore item from trash", func(t *testing.T) {
		page, err := svc.GetTrashPage("list", "name", "asc", "es")
		if err != nil {
			t.Fatalf("failed to get trash page: %v", err)
		}

		trashFileName := page.Files[0].Name
		if err := svc.RestoreFromTrash(trashFileName); err != nil {
			t.Fatalf("failed to restore file: %v", err)
		}

		// Verify file restored to root
		content, err := os.ReadFile(filepath.Join(tempDir, "testfile.txt"))
		if err != nil {
			t.Fatalf("restored file not found in root: %v", err)
		}
		if string(content) != "hello world" {
			t.Errorf("unexpected content: %s", string(content))
		}

		// Verify trash now only has 0 files
		pageAfter, err := svc.GetTrashPage("list", "name", "asc", "es")
		if err != nil {
			t.Fatalf("failed to get trash page: %v", err)
		}
		if len(pageAfter.Files) != 0 {
			t.Errorf("expected 0 files in trash, got %d", len(pageAfter.Files))
		}
	})

	t.Run("permanently delete from trash", func(t *testing.T) {
		// Move testfile.txt back to trash
		if err := svc.DeleteItem("testfile.txt"); err != nil {
			t.Fatalf("failed to move to trash: %v", err)
		}

		page, err := svc.GetTrashPage("list", "name", "asc", "es")
		if err != nil {
			t.Fatalf("failed to get trash page: %v", err)
		}

		var targetName string
		for _, f := range page.Files {
			if f.TrashOriginalPath == "testfile.txt" {
				targetName = f.Name
				break
			}
		}
		if targetName == "" {
			t.Fatalf("testfile.txt not found in trash")
		}

		if err := svc.DeletePermanentFromTrash(targetName); err != nil {
			t.Fatalf("failed to delete permanently: %v", err)
		}

		pageAfter, _ := svc.GetTrashPage("list", "name", "asc", "es")
		for _, f := range pageAfter.Files {
			if f.Name == targetName {
				t.Errorf("item still in trash after DeletePermanentFromTrash")
			}
		}
	})

	t.Run("empty trash", func(t *testing.T) {
		if err := svc.EmptyTrash(); err != nil {
			t.Fatalf("failed to empty trash: %v", err)
		}

		page, err := svc.GetTrashPage("list", "name", "asc", "es")
		if err != nil {
			t.Fatalf("failed to get trash page: %v", err)
		}
		if len(page.Files) != 0 || len(page.Folders) != 0 {
			t.Errorf("expected empty trash, got %d files, %d folders", len(page.Files), len(page.Folders))
		}
	})
}
