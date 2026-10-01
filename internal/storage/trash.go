package storage

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"simplefs/internal/filetype"
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

func (s *Service) getTrashDir() string {
	return filepath.Join(s.baseDirectory, ".trash")
}

func (s *Service) getTrashMetaFile() string {
	return filepath.Join(s.getTrashDir(), ".meta.json")
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

	normalized := strings.ReplaceAll(strings.TrimSpace(relativePath), "\\", "/")
	cleanRelative := filepath.Clean(filepath.FromSlash(normalized))
	if cleanRelative == "" || cleanRelative == "." || cleanRelative == "/" {
		return ErrCannotDeleteRoot
	}

	for _, segment := range strings.Split(cleanRelative, string(filepath.Separator)) {
		if isRestrictedSegment(segment) {
			return ErrProtectedItem
		}
	}

	absolutePath, err := s.ResolvePath(relativePath)
	if err != nil {
		return err
	}

	fileInfo, err := os.Stat(absolutePath)
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
	if _, err := os.Stat(destinationPath); err == nil {
		ext := filepath.Ext(baseName)
		nameWithoutExt := strings.TrimSuffix(baseName, ext)
		targetTrashName = fmt.Sprintf("%s_%s%s", nameWithoutExt, time.Now().Format("20060102_150405"), ext)
		destinationPath = filepath.Join(trashDir, targetTrashName)
	}

	// Move item into trash
	if err := os.Rename(absolutePath, destinationPath); err != nil {
		// Fallback for cross-device moves
		if copyErr := moveCrossDevice(absolutePath, destinationPath, fileInfo.IsDir()); copyErr != nil {
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
		if strings.HasPrefix(name, ".") {
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

		extension := filepath.Ext(name)
		typeDef := filetype.Resolve(extension, entry.IsDir())

		childCount := 0
		folderBgClass := ""
		folderDotClass := ""
		if entry.IsDir() {
			childCount = countDirectoryChildren(filepath.Join(trashDir, name))
			folderBgClass = "bg-primary/10 dark:bg-primary-container-dark/30 text-primary dark:text-primary-dark"
			folderDotClass = "bg-primary dark:bg-primary-dark"
		}

		itemObj := models.FileInfo{
			Name:              name,
			RelPath:           name,
			TrashOriginalPath: originalPath,
			IsDir:             entry.IsDir(),
			Size:              info.Size(),
			FormattedSize:     FormatBytes(info.Size()),
			ModTime:           info.ModTime(),
			FormattedMod:      i18n.FormatDate(info.ModTime(), normalizedLang),
			FormattedCreated:  i18n.FormatDate(info.ModTime(), normalizedLang),
			ItemCount:         childCount,
			TypeLabel:         typeDef.Label,
			MaterialIcon:      typeDef.Icon,
			IconColorClass:    typeDef.ColorClass,
			FolderBgClass:     folderBgClass,
			FolderDotClass:    folderDotClass,
			IsImage:           typeDef.Category == filetype.CategoryImage,
			IsZip:             !entry.IsDir() && strings.EqualFold(extension, ".zip"),
			Category:          string(typeDef.Category),
			Extension:         strings.TrimPrefix(strings.ToUpper(extension), "."),
		}

		if entry.IsDir() {
			folders = append(folders, itemObj)
		} else {
			files = append(files, itemObj)
		}
	}

	normalizedSortBy, normalizedSortOrder := NormalizeSortParams(sortBy, sortOrder)
	sortFolders(folders, normalizedSortBy, normalizedSortOrder)
	sortFiles(files, normalizedSortBy, normalizedSortOrder)

	if viewMode == "" {
		viewMode = models.ViewModeList
	}

	return models.PageData{
		Path:        "",
		Query:       "",
		SortBy:      normalizedSortBy,
		SortOrder:   normalizedSortOrder,
		Breadcrumbs: nil,
		Folders:     folders,
		Files:       files,
		ViewMode:    viewMode,
		Lang:        normalizedLang,
		IsTrash:     true,
	}, nil
}

// RestoreFromTrash restores a trashed item back to its original location or root.
func (s *Service) RestoreFromTrash(trashName string) error {
	trashMutex.Lock()
	defer trashMutex.Unlock()

	cleanName := filepath.Base(filepath.Clean(trashName))
	if cleanName == "" || cleanName == "." || cleanName == ".." || strings.HasPrefix(cleanName, ".") {
		return ErrInvalidName
	}

	trashDir := s.getTrashDir()
	sourcePath := filepath.Join(trashDir, cleanName)

	fileInfo, err := os.Stat(sourcePath)
	if err != nil {
		return ErrNotFound
	}

	store := s.loadTrashMeta()
	meta, hasMeta := store.Items[cleanName]

	targetRelPath := cleanName
	if hasMeta && meta.OriginalPath != "" {
		targetRelPath = meta.OriginalPath
	}

	cleanTarget := filepath.Clean(filepath.FromSlash(targetRelPath))
	if strings.HasPrefix(cleanTarget, "..") || filepath.IsAbs(cleanTarget) {
		cleanTarget = cleanName
	}

	absBase, err := filepath.Abs(s.baseDirectory)
	if err != nil {
		return err
	}

	destPath := filepath.Join(absBase, cleanTarget)
	destParent := filepath.Dir(destPath)

	if err := os.MkdirAll(destParent, 0700); err != nil {
		return err
	}

	// If destination already exists, append "_restored" to prevent overwriting
	if _, err := os.Stat(destPath); err == nil {
		ext := filepath.Ext(destPath)
		noExt := strings.TrimSuffix(destPath, ext)
		destPath = fmt.Sprintf("%s_restored%s", noExt, ext)
	}

	if err := os.Rename(sourcePath, destPath); err != nil {
		if copyErr := moveCrossDevice(sourcePath, destPath, fileInfo.IsDir()); copyErr != nil {
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

	cleanName := filepath.Base(filepath.Clean(trashName))
	if cleanName == "" || cleanName == "." || cleanName == ".." || strings.HasPrefix(cleanName, ".") {
		return ErrInvalidName
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
		if entry.Name() == ".meta.json" {
			continue
		}
		_ = os.RemoveAll(filepath.Join(trashDir, entry.Name()))
	}

	store := TrashMetaStore{Items: make(map[string]TrashMetaEntry)}
	return s.saveTrashMeta(store)
}

func moveCrossDevice(src, dst string, isDir bool) error {
	if isDir {
		if err := os.MkdirAll(dst, 0700); err != nil {
			return err
		}
		entries, err := os.ReadDir(src)
		if err != nil {
			return err
		}
		for _, e := range entries {
			if err := moveCrossDevice(filepath.Join(src, e.Name()), filepath.Join(dst, e.Name()), e.IsDir()); err != nil {
				return err
			}
		}
		return os.RemoveAll(src)
	}

	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return err
	}

	return os.Remove(src)
}
