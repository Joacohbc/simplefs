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

func (s *Service) getTrashDir() string {
	return filepath.Join(s.baseDirectory, trashDirName)
}

func (s *Service) getTrashMetaFile() string {
	return filepath.Join(s.getTrashDir(), trashMetaFileName)
}

func validTrashName(trashName string) (string, error) {
	cleanName := filepath.Base(filepath.Clean(strings.TrimSpace(trashName)))
	if cleanName == "" || cleanName == "." || cleanName == ".." || cleanName == trashMetaFileName || cleanName == string(filepath.Separator) {
		return "", ErrInvalidName
	}
	return cleanName, nil
}

func (s *Service) ensureTrashDir() error {
	trashDir := s.getTrashDir()
	return os.MkdirAll(trashDir, 0700)
}

func (s *Service) loadTrashMeta() TrashMetaStore {
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

func (s *Service) saveTrashMeta(store TrashMetaStore) error {
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
func (s *Service) MoveToTrash(relativePath string) error {
	trashMutex.Lock()
	defer trashMutex.Unlock()

	cleanRelative := cleanRelativePath(relativePath)
	absolutePath, err := s.validateDeletable(cleanRelative)
	if err != nil {
		return err
	}

	fileInfo, err := os.Lstat(absolutePath)
	if err != nil {
		return ErrNotFound
	}

	if err := s.ensureTrashDir(); err != nil {
		return fmt.Errorf("failed to prepare trash directory: %w", err)
	}

	store := s.loadTrashMeta()

	baseName := filepath.Base(absolutePath)
	targetTrashName := baseName
	trashDir := s.getTrashDir()
	destinationPath := filepath.Join(trashDir, targetTrashName)

	// If name collision in trash, append timestamp
	if _, err := os.Lstat(destinationPath); err == nil {
		ext := filepath.Ext(baseName)
		nameWithoutExt := strings.TrimSuffix(baseName, ext)
		targetTrashName = fmt.Sprintf("%s_%s%s", nameWithoutExt, time.Now().Format("20060102_150405"), ext)
		targetTrashName = uniqueName(trashDir, targetTrashName)
		destinationPath = filepath.Join(trashDir, targetTrashName)
	}

	// Move item into trash
	if err := os.Rename(absolutePath, destinationPath); err != nil {
		// Fallback for cross-device moves
		if copyErr := moveCrossDevice(absolutePath, destinationPath, fileInfo.IsDir(), s.options); copyErr != nil {
			return fmt.Errorf("cannot move to trash: %w", copyErr)
		}
	}

	store.Items[targetTrashName] = TrashMetaEntry{
		OriginalPath: filepath.ToSlash(cleanRelative),
		DeletedAt:    time.Now(),
		IsDir:        fileInfo.IsDir(),
	}

	return s.saveTrashMeta(store)
}

// GetTrashPage retrieves all items in the trash directory and returns PageData with IsTrash = true.
func (s *Service) GetTrashPage(viewMode, sortBy, sortOrder, lang string) (models.PageData, error) {
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
func (s *Service) RestoreFromTrash(trashName string) error {
	trashMutex.Lock()
	defer trashMutex.Unlock()

	cleanName, err := validTrashName(trashName)
	if err != nil {
		return err
	}

	trashDir := s.getTrashDir()
	sourcePath := filepath.Join(trashDir, cleanName)

	fileInfo, err := os.Lstat(sourcePath)
	if err != nil {
		return ErrNotFound
	}

	store := s.loadTrashMeta()
	meta, hasMeta := store.Items[cleanName]

	targetRelPath := cleanName
	if hasMeta && meta.OriginalPath != "" {
		targetRelPath = meta.OriginalPath
	}

	// Resolve through ResolvePath so a parent swapped for a symlink cannot redirect the restore outside the root.
	destPath, err := s.ResolvePath(targetRelPath)
	if err != nil {
		destPath, err = s.ResolvePath(cleanName)
		if err != nil {
			return err
		}
	}

	destParent := filepath.Dir(destPath)
	if err := os.MkdirAll(destParent, s.options.DirMode); err != nil {
		return err
	}
	destPath = filepath.Join(destParent, uniqueName(destParent, filepath.Base(destPath)))

	if err := os.Rename(sourcePath, destPath); err != nil {
		if copyErr := moveCrossDevice(sourcePath, destPath, fileInfo.IsDir(), s.options); copyErr != nil {
			return copyErr
		}
	}

	delete(store.Items, cleanName)
	return s.saveTrashMeta(store)
}

// DeletePermanentFromTrash permanently deletes an item from trash.
func (s *Service) DeletePermanentFromTrash(trashName string) error {
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
func (s *Service) EmptyTrash() error {
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

	store := TrashMetaStore{Items: make(map[string]TrashMetaEntry)}
	return s.saveTrashMeta(store)
}

func moveCrossDevice(src, dst string, isDir bool, options Options) error {
	if isDir {
		if err := os.MkdirAll(dst, options.DirMode); err != nil {
			return err
		}
		entries, err := os.ReadDir(src)
		if err != nil {
			return err
		}
		for _, e := range entries {
			if err := moveCrossDevice(filepath.Join(src, e.Name()), filepath.Join(dst, e.Name()), e.IsDir(), options); err != nil {
				return err
			}
		}
		return os.RemoveAll(src)
	}

	if err := copyFile(src, dst, options.FileMode); err != nil {
		return err
	}
	return os.Remove(src)
}
