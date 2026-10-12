package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"simplefs/internal/i18n"
	"simplefs/internal/models"
)

type TrashMetaEntry struct {
	OriginalPath string    `json:"original_path"`
	DeletedAt    time.Time `json:"deleted_at"`
	IsDir        bool      `json:"is_dir"`
}

type TrashMetaStore struct {
	Items map[string]TrashMetaEntry `json:"items"`
}

var trashMutex sync.Mutex

const trashMetaFileName = ".meta.json"

func (s *FileStore) getTrashDir() string {
	return filepath.Join(s.baseDirectory, trashDirName)
}

func (s *FileStore) getTrashMetaFile() string {
	return filepath.Join(s.getTrashDir(), trashMetaFileName)
}

func validTrashName(trashName string) (string, error) {
	cleanName := filepath.Base(filepath.Clean(strings.TrimSpace(trashName)))
	if cleanName == "" || cleanName == "." || cleanName == ".." || cleanName == trashMetaFileName || cleanName == string(filepath.Separator) {
		return "", ErrInvalidName
	}
	return cleanName, nil
}

func (s *FileStore) ensureTrashDir() error {
	trashDir := s.getTrashDir()
	return os.MkdirAll(trashDir, 0700)
}

func (s *FileStore) loadTrashMeta() TrashMetaStore {
	store := TrashMetaStore{Items: make(map[string]TrashMetaEntry)}
	metaFile := s.getTrashMetaFile()
	data, err := os.ReadFile(metaFile)
	if err != nil {
		return store
	}
	_ = json.Unmarshal(data, &store)
	if store.Items == nil {
		store.Items = make(map[string]TrashMetaEntry)
	}
	return store
}

func (s *FileStore) saveTrashMeta(store TrashMetaStore) error {
	if err := s.ensureTrashDir(); err != nil {
		return err
	}
	metaFile := s.getTrashMetaFile()
	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(metaFile, data, 0600)
}

// MoveToTrash moves a file or directory into .trash and records its original path.
func (s *FileStore) MoveToTrash(relativePath string) error {
	trashMutex.Lock()
	defer trashMutex.Unlock()

	cleanRelative := cleanRelativePath(relativePath)
	absolutePath, err := s.validateDeletable(cleanRelative)
	if err != nil {
		return err
	}
	entryInfo, err := os.Lstat(absolutePath)
	if err != nil {
		return ErrNotFound
	}
	if err := s.ensureTrashDir(); err != nil {
		return fmt.Errorf("failed to prepare trash directory: %w", err)
	}

	trashDir := s.getTrashDir()
	trashName := uniqueEntryName(trashDir, filepath.Base(absolutePath), entryInfo.IsDir())
	if err := moveEntry(absolutePath, filepath.Join(trashDir, trashName), entryInfo.IsDir(), s.options); err != nil {
		return fmt.Errorf("cannot move to trash: %w", err)
	}

	trashIndex := s.loadTrashMeta()
	trashIndex.Items[trashName] = TrashMetaEntry{
		OriginalPath: filepath.ToSlash(cleanRelative),
		DeletedAt:    time.Now(),
		IsDir:        entryInfo.IsDir(),
	}
	return s.saveTrashMeta(trashIndex)
}

// GetTrashPage retrieves all items in the trash directory and returns PageData with IsTrash = true.
func (s *FileStore) GetTrashPage(viewMode, sortBy, sortOrder, lang string) (models.PageData, error) {
	trashMutex.Lock()
	defer trashMutex.Unlock()

	if err := s.ensureTrashDir(); err != nil {
		return models.PageData{}, err
	}

	trashDir := s.getTrashDir()
	entries, err := os.ReadDir(trashDir)
	if err != nil {
		return models.PageData{}, err
	}

	store := s.loadTrashMeta()
	normalizedLang := i18n.NormalizeLang(lang)

	var folders []models.FileInfo
	var files []models.FileInfo

	for _, entry := range entries {
		name := entry.Name()
		if name == trashMetaFileName {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		meta, hasMeta := store.Items[name]
		originalPath := ""
		if hasMeta {
			originalPath = meta.OriginalPath
		}

		itemObj := s.toFileInfo(name, info, "", trashDir, normalizedLang)
		itemObj.RelPath = name
		itemObj.ParentPath = ""
		itemObj.TrashOriginalPath = originalPath

		if entry.IsDir() {
			folders = append(folders, itemObj)
		} else {
			files = append(files, itemObj)
		}
	}

	normalizedSortBy, normalizedSortOrder := NormalizeSortParams(sortBy, sortOrder)
	sortItems(folders, normalizedSortBy, normalizedSortOrder)
	sortItems(files, normalizedSortBy, normalizedSortOrder)

	return models.PageData{
		Path:        "",
		Query:       "",
		SortBy:      normalizedSortBy,
		SortOrder:   normalizedSortOrder,
		Breadcrumbs: nil,
		Folders:     folders,
		Files:       files,
		ViewMode:    NormalizeViewMode(viewMode),
		Lang:        normalizedLang,
		IsTrash:     true,
	}, nil
}

// RestoreFromTrash restores a trashed item back to its original location or root.
func (s *FileStore) RestoreFromTrash(trashName string) error {
	trashMutex.Lock()
	defer trashMutex.Unlock()

	cleanName, err := validTrashName(trashName)
	if err != nil {
		return err
	}
	sourcePath := filepath.Join(s.getTrashDir(), cleanName)
	entryInfo, err := os.Lstat(sourcePath)
	if err != nil {
		return ErrNotFound
	}

	trashIndex := s.loadTrashMeta()
	destinationPath, err := s.restoreDestination(cleanName, trashIndex.Items[cleanName])
	if err != nil {
		return err
	}
	destinationParent := filepath.Dir(destinationPath)
	if err := os.MkdirAll(destinationParent, s.options.DirMode); err != nil {
		return err
	}
	restoredName := uniqueEntryName(destinationParent, filepath.Base(destinationPath), entryInfo.IsDir())
	if err := moveEntry(sourcePath, filepath.Join(destinationParent, restoredName), entryInfo.IsDir(), s.options); err != nil {
		return err
	}

	delete(trashIndex.Items, cleanName)
	return s.saveTrashMeta(trashIndex)
}

// restoreDestination resolves the original location through ResolvePath so a parent
// swapped for a symlink cannot redirect the restore outside the root.
func (s *FileStore) restoreDestination(trashName string, meta TrashMetaEntry) (string, error) {
	if meta.OriginalPath != "" {
		if destination, err := s.ResolvePath(meta.OriginalPath); err == nil {
			return destination, nil
		}
	}
	return s.ResolvePath(trashName)
}

// DeletePermanentFromTrash permanently deletes an item from trash.
func (s *FileStore) DeletePermanentFromTrash(trashName string) error {
	trashMutex.Lock()
	defer trashMutex.Unlock()

	cleanName, err := validTrashName(trashName)
	if err != nil {
		return err
	}

	trashDir := s.getTrashDir()
	targetPath := filepath.Join(trashDir, cleanName)

	if _, err := os.Stat(targetPath); err != nil {
		return ErrNotFound
	}

	if err := os.RemoveAll(targetPath); err != nil {
		return err
	}

	store := s.loadTrashMeta()
	delete(store.Items, cleanName)
	return s.saveTrashMeta(store)
}

// EmptyTrash removes all items currently in the trash.
func (s *FileStore) EmptyTrash() error {
	trashMutex.Lock()
	defer trashMutex.Unlock()

	trashDir := s.getTrashDir()
	entries, err := os.ReadDir(trashDir)
	if err != nil {
		return nil
	}

	for _, entry := range entries {
		if entry.Name() == trashMetaFileName {
			continue
		}
		_ = os.RemoveAll(filepath.Join(trashDir, entry.Name()))
	}

	return s.saveTrashMeta(TrashMetaStore{Items: make(map[string]TrashMetaEntry)})
}

func moveEntry(source, destination string, isDir bool, options Options) error {
	if err := os.Rename(source, destination); err == nil {
		return nil
	}
	if isDir {
		return moveDirectoryAcrossDevices(source, destination, options)
	}
	if err := copyFile(source, destination, options.FileMode); err != nil {
		return err
	}
	return os.Remove(source)
}

func moveDirectoryAcrossDevices(source, destination string, options Options) error {
	if err := os.MkdirAll(destination, options.DirMode); err != nil {
		return err
	}
	children, err := os.ReadDir(source)
	if err != nil {
		return err
	}
	for _, child := range children {
		childSource := filepath.Join(source, child.Name())
		childDestination := filepath.Join(destination, child.Name())
		if err := moveEntry(childSource, childDestination, child.IsDir(), options); err != nil {
			return err
		}
	}
	return os.RemoveAll(source)
}
