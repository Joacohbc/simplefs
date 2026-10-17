package storage

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

type ConflictPolicy string

const (
	ConflictRename    ConflictPolicy = "rename"
	ConflictOverwrite ConflictPolicy = "overwrite"
	ConflictFail      ConflictPolicy = "fail"
)

func ParseConflictPolicy(value string, fallback ConflictPolicy) ConflictPolicy {
	switch ConflictPolicy(strings.ToLower(strings.TrimSpace(value))) {
	case ConflictRename:
		return ConflictRename
	case ConflictOverwrite:
		return ConflictOverwrite
	case ConflictFail:
		return ConflictFail
	}
	return fallback
}

type WriteOptions struct {
	Conflict      ConflictPolicy
	CreateParents bool
}

type WriteResult struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	Size        int64  `json:"size"`
	SHA256      string `json:"sha256"`
	Overwritten bool   `json:"overwritten"`
	Renamed     bool   `json:"renamed"`
}

// WriteFile streams reader into a temporary file and atomically moves it into place,
// so readers never observe a partially written file.
func (s *FileStore) WriteFile(directoryRelativePath, filename string, reader io.Reader, options WriteOptions) (WriteResult, error) {
	cleanFilename, err := s.validateItemName(filename)
	if err != nil {
		return WriteResult{}, err
	}
	targetDirectory, err := s.prepareTargetDirectory(directoryRelativePath, options.CreateParents)
	if err != nil {
		return WriteResult{}, err
	}

	spooled, err := s.spoolToTempFile(reader)
	if err != nil {
		return WriteResult{}, err
	}
	cleanDirectory := cleanRelativePath(directoryRelativePath)
	placed, err := s.placeFile(spooled.path, targetDirectory, cleanDirectory, cleanFilename, options.Conflict)
	if err != nil {
		_ = os.Remove(spooled.path)
		return WriteResult{}, err
	}

	return WriteResult{
		Name:        placed.name,
		Path:        filepath.ToSlash(filepath.Join(cleanDirectory, placed.name)),
		Size:        spooled.sizeBytes,
		SHA256:      spooled.sha256,
		Overwritten: placed.wasOverwritten,
		Renamed:     placed.name != cleanFilename,
	}, nil
}

func (s *FileStore) prepareTargetDirectory(directoryRelativePath string, createParents bool) (string, error) {
	targetDirectory, err := s.ResolvePath(directoryRelativePath)
	if err != nil {
		return "", err
	}
	if createParents {
		if err := os.MkdirAll(targetDirectory, s.options.DirMode); err != nil {
			return "", err
		}
	}
	directoryInfo, err := os.Stat(targetDirectory)
	if err != nil {
		return "", ErrNotFound
	}
	if !directoryInfo.IsDir() {
		return "", ErrNotADirectory
	}
	return targetDirectory, nil
}

type spooledFile struct {
	path      string
	sizeBytes int64
	sha256    string
}

func (s *FileStore) spoolToTempFile(reader io.Reader) (spooledFile, error) {
	tempFile, err := s.createTempFile()
	if err != nil {
		return spooledFile{}, fmt.Errorf("cannot create temporary file: %w", err)
	}
	hasher := sha256.New()
	sizeBytes, copyErr := io.Copy(io.MultiWriter(tempFile, hasher), reader)
	closeErr := tempFile.Close()
	spoolErr := errors.Join(copyErr, closeErr)
	if spoolErr == nil {
		spoolErr = os.Chmod(tempFile.Name(), s.options.FileMode)
	}
	if spoolErr != nil {
		_ = os.Remove(tempFile.Name())
		return spooledFile{}, spoolErr
	}
	return spooledFile{path: tempFile.Name(), sizeBytes: sizeBytes, sha256: hex.EncodeToString(hasher.Sum(nil))}, nil
}

type placement struct {
	name           string
	wasOverwritten bool
}

func (s *FileStore) placeFile(tempPath, targetDirectory, relativeDirectory, name string, policy ConflictPolicy) (placement, error) {
	if policy == ConflictOverwrite {
		return s.placeOverwriting(tempPath, targetDirectory, relativeDirectory, name)
	}
	return s.placeWithoutClobbering(tempPath, targetDirectory, name, policy)
}

// placeOverwriting keeps the replaced file recoverable by moving it to the trash first.
func (s *FileStore) placeOverwriting(tempPath, targetDirectory, relativeDirectory, name string) (placement, error) {
	destination := filepath.Join(targetDirectory, name)
	existing, err := os.Lstat(destination)
	destinationExists := err == nil
	if destinationExists && existing.Mode()&os.ModeSymlink != 0 {
		return placement{}, ErrSymlinkOverwrite
	}
	if destinationExists && existing.IsDir() {
		return placement{}, ErrIsDirectory
	}
	if destinationExists {
		if err := s.MoveToTrash(filepath.Join(relativeDirectory, name)); err != nil {
			return placement{}, err
		}
	}
	if err := renameOrCopy(tempPath, destination, s.options.FileMode); err != nil {
		return placement{}, err
	}
	return placement{name: name, wasOverwritten: destinationExists}, nil
}

func (s *FileStore) placeWithoutClobbering(tempPath, targetDirectory, name string, policy ConflictPolicy) (placement, error) {
	candidate := name
	for attempt := 1; attempt <= maxNumberedNameAttempts; attempt++ {
		err := linkNoClobber(tempPath, filepath.Join(targetDirectory, candidate), s.options.FileMode)
		if err == nil {
			return placement{name: candidate}, nil
		}
		if !errors.Is(err, fs.ErrExist) {
			return placement{}, err
		}
		if policy == ConflictFail {
			return placement{}, ErrAlreadyExists
		}
		candidate = numberedName(name, attempt)
	}
	return placement{}, ErrAlreadyExists
}

// linkNoClobber moves source to destination failing with fs.ErrExist when destination exists.
// A hard link gives an atomic no-overwrite rename; filesystems without hard links
// fall back to an existence check followed by a rename.
func linkNoClobber(source, destination string, mode os.FileMode) error {
	linkErr := os.Link(source, destination)
	if linkErr == nil {
		return os.Remove(source)
	}
	if errors.Is(linkErr, fs.ErrExist) {
		return fs.ErrExist
	}
	if _, err := os.Lstat(destination); err == nil {
		return fs.ErrExist
	}
	return renameOrCopy(source, destination, mode)
}

func renameOrCopy(source, destination string, mode os.FileMode) error {
	if err := os.Rename(source, destination); err == nil {
		return nil
	}
	if err := copyFile(source, destination, mode); err != nil {
		return err
	}
	return os.Remove(source)
}

func copyFile(source, destination string, mode os.FileMode) error {
	sourceFile, err := os.Open(source)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destinationFile, err := os.OpenFile(destination, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(destinationFile, sourceFile); err != nil {
		destinationFile.Close()
		return err
	}
	return destinationFile.Close()
}

func numberedName(name string, number int) string {
	extension := filepath.Ext(name)
	baseName := strings.TrimSuffix(name, extension)
	if baseName == "" {
		baseName, extension = name, ""
	}
	return fmt.Sprintf("%s (%d)%s", baseName, number, extension)
}

func numberedFolderName(name string, number int) string {
	return fmt.Sprintf("%s (%d)", name, number)
}

func firstAvailableName(directory, name string, numbered func(name string, number int) string) string {
	candidate := name
	for number := 1; number <= maxNumberedNameAttempts; number++ {
		if _, err := os.Lstat(filepath.Join(directory, candidate)); errors.Is(err, fs.ErrNotExist) {
			return candidate
		}
		candidate = numbered(name, number)
	}
	return candidate + "-" + randomSuffix()
}

func uniqueFileName(directory, name string) string {
	return firstAvailableName(directory, name, numberedName)
}

func uniqueFolderName(directory, name string) string {
	return firstAvailableName(directory, name, numberedFolderName)
}

func uniqueEntryName(directory, name string, isDir bool) string {
	if isDir {
		return uniqueFolderName(directory, name)
	}
	return uniqueFileName(directory, name)
}

func randomSuffix() string {
	randomBytes := make([]byte, randomSuffixBytes)
	_, _ = rand.Read(randomBytes)
	return hex.EncodeToString(randomBytes)
}

func (s *FileStore) CreateFolder(parentRelativePath, folderName string) error {
	cleanFolderName, err := s.validateItemName(folderName)
	if err != nil {
		return err
	}

	targetDirectory, err := s.ResolvePath(parentRelativePath)
	if err != nil {
		return err
	}

	if _, err := os.Stat(targetDirectory); err != nil {
		return ErrNotFound
	}

	newFolderPath := filepath.Join(targetDirectory, cleanFolderName)
	if err := ensureNotSymlink(newFolderPath); err != nil {
		return err
	}

	return os.MkdirAll(newFolderPath, s.options.DirMode)
}

// CreateFolderPath creates every missing folder of relativePath (mkdir -p).
func (s *FileStore) CreateFolderPath(relativePath string) error {
	cleanRelative := cleanRelativePath(relativePath)
	if cleanRelative == "" {
		return nil
	}
	for _, segment := range strings.Split(cleanRelative, string(filepath.Separator)) {
		if _, err := s.validateItemName(segment); err != nil {
			return err
		}
	}
	absolutePath, err := s.ResolvePath(cleanRelative)
	if err != nil {
		return err
	}
	if info, err := os.Stat(absolutePath); err == nil && !info.IsDir() {
		return ErrAlreadyExists
	}
	return os.MkdirAll(absolutePath, s.options.DirMode)
}

func (s *FileStore) validateDeletable(relativePath string) (string, error) {
	cleanRelative := cleanRelativePath(relativePath)
	if cleanRelative == "" {
		return "", ErrCannotDeleteRoot
	}
	if cleanRelative == ".." || strings.HasPrefix(cleanRelative, ".."+string(filepath.Separator)) {
		return "", ErrInvalidPath
	}
	for _, segment := range strings.Split(cleanRelative, string(filepath.Separator)) {
		if s.isHidden(segment) {
			return "", ErrProtectedItem
		}
	}

	absolutePath, err := s.ResolvePath(cleanRelative)
	if err != nil {
		return "", err
	}
	absoluteBase, err := s.absoluteBase()
	if err != nil {
		return "", err
	}
	if absolutePath == absoluteBase {
		return "", ErrCannotDeleteRoot
	}
	return absolutePath, nil
}

func (s *FileStore) DeleteItem(relativePath string) error {
	if _, err := s.validateDeletable(relativePath); err != nil {
		return err
	}
	return s.MoveToTrash(relativePath)
}

func (s *FileStore) DeleteItemPermanently(relativePath string) error {
	absolutePath, err := s.validateDeletable(relativePath)
	if err != nil {
		return err
	}
	if _, err := os.Lstat(absolutePath); err != nil {
		return ErrNotFound
	}
	return os.RemoveAll(absolutePath)
}
