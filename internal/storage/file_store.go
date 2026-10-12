package storage

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

const (
	maxInlinePreviewBytes   = 1000 * 1024
	maxNameLengthBytes      = 255
	maxNumberedNameAttempts = 10000
	randomSuffixBytes       = 4
	internalDirMode         = 0700

	MaxSearchResults   = 500
	MaxWalkEntries     = 100000
	DefaultRecentLimit = 50
	MaxRecentLimit     = 500

	trashDirName      = ".trash"
	internalDirName   = ".simplefs"
	internalTmpPrefix = ".simplefs-"
)

var (
	ErrNotFound         = errors.New("file or directory not found")
	ErrInvalidPath      = errors.New("invalid path access")
	ErrAccessDenied     = errors.New("access denied")
	ErrProtectedItem    = errors.New("access denied: protected item")
	ErrSymlinkForbidden = errors.New("symlink target outside storage root")
	ErrSymlinkOverwrite = errors.New("cannot overwrite symlink")
	ErrInvalidName      = errors.New("invalid filename or folder name")
	ErrCannotDeleteRoot = errors.New("cannot delete root directory")
	ErrAlreadyExists    = errors.New("a file or folder with that name already exists")
	ErrIsDirectory      = errors.New("target is a directory")
)

type Options struct {
	FileMode     os.FileMode
	DirMode      os.FileMode
	HideDotfiles bool
}

func DefaultOptions() Options {
	return Options{FileMode: 0644, DirMode: 0755}
}

type FileStore struct {
	baseDirectory string
	options       Options
}

func NewFileStore(baseDirectory string) *FileStore {
	return NewFileStoreWithOptions(baseDirectory, DefaultOptions())
}

func NewFileStoreWithOptions(baseDirectory string, options Options) *FileStore {
	defaults := DefaultOptions()
	if options.FileMode == 0 {
		options.FileMode = defaults.FileMode
	}
	if options.DirMode == 0 {
		options.DirMode = defaults.DirMode
	}
	return &FileStore{baseDirectory: baseDirectory, options: options}
}

// IsReservedName reports names SimpleFS keeps for itself: the trash, its internal
// state directory and in-flight upload files.
func IsReservedName(name string) bool {
	trimmed := strings.TrimSpace(name)
	return trimmed == trashDirName || trimmed == internalDirName || strings.HasPrefix(trimmed, internalTmpPrefix)
}

func (s *FileStore) isHidden(name string) bool {
	if IsReservedName(name) {
		return true
	}
	return s.options.HideDotfiles && strings.HasPrefix(name, ".")
}

func (s *FileStore) validateItemName(name string) (string, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" || trimmed == "." || trimmed == ".." || len(trimmed) > maxNameLengthBytes {
		return "", ErrInvalidName
	}
	if strings.ContainsAny(trimmed, "/\\\x00") {
		return "", ErrInvalidName
	}
	for _, r := range trimmed {
		if unicode.IsControl(r) {
			return "", ErrInvalidName
		}
	}
	if s.isHidden(trimmed) {
		return "", ErrInvalidName
	}
	return trimmed, nil
}

func ensureNotSymlink(path string) error {
	fileInfo, err := os.Lstat(path)
	if err != nil {
		return nil
	}
	if fileInfo.Mode()&os.ModeSymlink != 0 {
		return ErrSymlinkOverwrite
	}
	return nil
}

func (s *FileStore) absoluteBase() (string, error) {
	absoluteBase, err := filepath.Abs(s.baseDirectory)
	if err != nil {
		return "", fmt.Errorf("storage directory error: %w", err)
	}
	if evaluatedBase, err := filepath.EvalSymlinks(absoluteBase); err == nil {
		absoluteBase = evaluatedBase
	}
	return absoluteBase, nil
}

func cleanRelativePath(relativePath string) string {
	normalized := strings.ReplaceAll(strings.TrimSpace(relativePath), "\\", "/")
	cleanRelative := filepath.Clean(filepath.FromSlash(normalized))
	cleanRelative = strings.TrimPrefix(cleanRelative, string(filepath.Separator))
	if cleanRelative == "." {
		return ""
	}
	return cleanRelative
}

func (s *FileStore) ResolvePath(relativePath string) (string, error) {
	absoluteBase, err := s.absoluteBase()
	if err != nil {
		return "", err
	}

	cleanRelative := cleanRelativePath(relativePath)
	if cleanRelative == ".." || strings.HasPrefix(cleanRelative, ".."+string(filepath.Separator)) {
		return "", ErrInvalidPath
	}

	if cleanRelative != "" {
		for _, segment := range strings.Split(cleanRelative, string(filepath.Separator)) {
			if s.isHidden(segment) {
				return "", ErrProtectedItem
			}
		}
	}

	targetPath := filepath.Join(absoluteBase, cleanRelative)
	relativeFromBase, err := filepath.Rel(absoluteBase, targetPath)
	if err != nil || relativeFromBase == ".." || strings.HasPrefix(relativeFromBase, ".."+string(filepath.Separator)) {
		return "", ErrAccessDenied
	}

	return s.evaluateSymlinkTarget(targetPath, absoluteBase)
}

func (s *FileStore) evaluateSymlinkTarget(targetPath, absoluteBase string) (string, error) {
	current := targetPath
	for {
		evaluated, err := filepath.EvalSymlinks(current)
		if err != nil {
			parent := filepath.Dir(current)
			if parent == current || len(parent) < len(absoluteBase) {
				break
			}
			current = parent
			continue
		}

		evaluatedRel, err := filepath.Rel(absoluteBase, evaluated)
		if err != nil || evaluatedRel == ".." || strings.HasPrefix(evaluatedRel, ".."+string(filepath.Separator)) {
			return "", ErrSymlinkForbidden
		}
		if current == targetPath {
			return evaluated, nil
		}
		suffix, relErr := filepath.Rel(current, targetPath)
		if relErr != nil {
			return "", ErrInvalidPath
		}
		return filepath.Join(evaluated, suffix), nil
	}
	return targetPath, nil
}

func (s *FileStore) internalTempDir() string {
	return filepath.Join(s.baseDirectory, internalDirName, "tmp")
}

// CleanupTemp removes partial uploads left behind by an interrupted process.
func (s *FileStore) CleanupTemp() error {
	tempDir := s.internalTempDir()
	if err := os.RemoveAll(tempDir); err != nil {
		return err
	}
	return os.MkdirAll(tempDir, internalDirMode)
}

func (s *FileStore) createTempFile() (*os.File, error) {
	tempDir := s.internalTempDir()
	if err := os.MkdirAll(tempDir, internalDirMode); err != nil {
		return nil, err
	}
	return os.CreateTemp(tempDir, internalTmpPrefix+"*.part")
}

func (s *FileStore) countDirectoryChildren(directoryPath string) int {
	subEntries, err := os.ReadDir(directoryPath)
	if err != nil {
		return 0
	}
	childCount := 0
	for _, entry := range subEntries {
		if s.isHidden(entry.Name()) {
			continue
		}
		childCount++
	}
	return childCount
}
