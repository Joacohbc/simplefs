package storage

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"simplefs/internal/filetype"
	"simplefs/internal/i18n"
	"simplefs/internal/models"
)

const maxInlinePreviewBytes = 1000 * 1024

var (
	ErrNotFound         = errors.New("file or directory not found")
	ErrInvalidPath      = errors.New("invalid path access")
	ErrAccessDenied     = errors.New("access denied")
	ErrProtectedItem    = errors.New("access denied: protected item")
	ErrSymlinkForbidden = errors.New("symlink target outside storage root")
	ErrSymlinkOverwrite = errors.New("cannot overwrite symlink")
	ErrInvalidName      = errors.New("invalid filename or folder name")
	ErrCannotDeleteRoot = errors.New("cannot delete root directory")
)

type ServiceInterface interface {
	ResolvePath(relativePath string) (string, error)
	GetDirectoryPage(relativePath, searchQuery, viewMode, sortBy, sortOrder, lang string) (models.PageData, error)
	GetFileDetails(relativePath, lang string) (models.FileDetailsData, error)
	GetFilePreview(relativePath, lang string) (models.PreviewData, error)
	SaveUploadedFile(targetDirectoryRelativePath, originalFilename string, fileReader io.Reader) error
	CreateFolder(parentRelativePath, folderName string) error
	CreateFile(parentRelativePath, filename string, content []byte) error
	DeleteItem(relativePath string) error
	GetDownloadFile(relativePath string) (filePath, filename, mimeType string, forceAttachment bool, err error)
}

type Service struct {
	baseDirectory string
}

func NewService(baseDirectory string) *Service {
	return &Service{
		baseDirectory: baseDirectory,
	}
}

func isRestrictedSegment(name string) bool {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return false
	}
	return strings.HasPrefix(trimmed, ".") ||
		trimmed == "simplefs" ||
		trimmed == "node_modules" ||
		trimmed == ".stitch" ||
		trimmed == ".dc_simplefs"
}

func validateItemName(name string) (string, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return "", ErrInvalidName
	}
	if strings.Contains(trimmed, "/") || strings.Contains(trimmed, "\\") || strings.Contains(trimmed, "..") {
		return "", ErrInvalidName
	}
	clean := filepath.Base(filepath.Clean(trimmed))
	if clean == "" || clean == "." || clean == ".." || isRestrictedSegment(clean) {
		return "", ErrInvalidName
	}
	return clean, nil
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

func (s *Service) ResolvePath(relativePath string) (string, error) {
	absoluteBase, err := filepath.Abs(s.baseDirectory)
	if err != nil {
		return "", fmt.Errorf("storage directory error: %w", err)
	}

	if evaluatedBase, err := filepath.EvalSymlinks(absoluteBase); err == nil {
		absoluteBase = evaluatedBase
	}

	normalized := strings.ReplaceAll(strings.TrimSpace(relativePath), "\\", "/")
	cleanRelative := filepath.Clean(filepath.FromSlash(normalized))
	if cleanRelative == "." || cleanRelative == "/" || cleanRelative == "" {
		cleanRelative = ""
	}

	if strings.HasPrefix(cleanRelative, "..") {
		return "", ErrInvalidPath
	}

	for _, segment := range strings.Split(cleanRelative, string(filepath.Separator)) {
		if isRestrictedSegment(segment) {
			return "", ErrProtectedItem
		}
	}

	targetPath := filepath.Join(absoluteBase, cleanRelative)
	relativeFromBase, err := filepath.Rel(absoluteBase, targetPath)
	if err != nil || strings.HasPrefix(relativeFromBase, "..") || relativeFromBase == ".." {
		return "", ErrAccessDenied
	}

	return s.evaluateSymlinkTarget(targetPath, absoluteBase)
}

func (s *Service) evaluateSymlinkTarget(targetPath, absoluteBase string) (string, error) {
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
		if err != nil || strings.HasPrefix(evaluatedRel, "..") || evaluatedRel == ".." {
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

func (s *Service) mapDirEntryToFileInfo(entry os.DirEntry, relativePath, absolutePath, lang string) (models.FileInfo, bool) {
	name := entry.Name()
	if isRestrictedSegment(name) {
		return models.FileInfo{}, false
	}

	entryInfo, err := entry.Info()
	if err != nil {
		return models.FileInfo{}, false
	}

	entryRelativePath := filepath.Join(relativePath, name)
	extension := filepath.Ext(name)
	typeDef := filetype.Resolve(extension, entry.IsDir())

	childCount := 0
	if entry.IsDir() {
		childCount = countDirectoryChildren(filepath.Join(absolutePath, name))
	}

	return models.FileInfo{
		Name:             name,
		RelPath:          filepath.ToSlash(entryRelativePath),
		IsDir:            entry.IsDir(),
		Size:             entryInfo.Size(),
		FormattedSize:    FormatBytes(entryInfo.Size()),
		ModTime:          entryInfo.ModTime(),
		FormattedMod:     i18n.FormatDate(entryInfo.ModTime(), lang),
		FormattedCreated: i18n.FormatDate(entryInfo.ModTime(), lang),
		ItemCount:        childCount,
		TypeLabel:        typeDef.Label,
		MaterialIcon:     typeDef.Icon,
		IconColorClass:   typeDef.ColorClass,
		IsImage:          typeDef.Category == filetype.CategoryImage,
	}, true
}

func (s *Service) GetDirectoryPage(relativePath, searchQuery, viewMode, sortBy, sortOrder, lang string) (models.PageData, error) {
	absolutePath, err := s.ResolvePath(relativePath)
	if err != nil {
		return models.PageData{}, err
	}

	entries, err := os.ReadDir(absolutePath)
	if err != nil {
		return models.PageData{}, ErrNotFound
	}

	var folders []models.FileInfo
	var files []models.FileInfo
	searchLower := strings.ToLower(searchQuery)
	normalizedLang := i18n.NormalizeLang(lang)

	for _, entry := range entries {
		name := entry.Name()
		if searchLower != "" && !strings.Contains(strings.ToLower(name), searchLower) {
			continue
		}

		fileObject, ok := s.mapDirEntryToFileInfo(entry, relativePath, absolutePath, normalizedLang)
		if !ok {
			continue
		}

		if entry.IsDir() {
			folders = append(folders, fileObject)
		} else {
			files = append(files, fileObject)
		}
	}

	normalizedSortBy, normalizedSortOrder := normalizeSortParams(sortBy, sortOrder)
	sortFolders(folders, normalizedSortBy, normalizedSortOrder)
	sortFiles(files, normalizedSortBy, normalizedSortOrder)

	if viewMode == "" {
		viewMode = models.ViewModeList
	}

	return models.PageData{
		Path:        filepath.ToSlash(relativePath),
		Query:       searchQuery,
		SortBy:      normalizedSortBy,
		SortOrder:   normalizedSortOrder,
		Breadcrumbs: BuildBreadcrumbs(relativePath),
		Folders:     folders,
		Files:       files,
		ViewMode:    viewMode,
		Lang:        normalizedLang,
	}, nil
}

func (s *Service) GetFileDetails(relativePath, lang string) (models.FileDetailsData, error) {
	absolutePath, err := s.ResolvePath(relativePath)
	if err != nil {
		return models.FileDetailsData{}, err
	}

	fileInfo, err := os.Stat(absolutePath)
	if err != nil {
		return models.FileDetailsData{}, ErrNotFound
	}

	extension := filepath.Ext(absolutePath)
	typeDef := filetype.Resolve(extension, fileInfo.IsDir())
	normalizedLang := i18n.NormalizeLang(lang)

	parentDirectory := filepath.ToSlash(filepath.Dir(relativePath))
	if parentDirectory == "." || parentDirectory == "" {
		parentDirectory = "/"
	} else {
		parentDirectory = "/" + parentDirectory
	}

	return models.FileDetailsData{
		Name:          filepath.Base(absolutePath),
		RelPath:       filepath.ToSlash(relativePath),
		DirLocation:   parentDirectory,
		TypeLabel:     typeDef.Label,
		MaterialIcon:  typeDef.Icon,
		FormattedSize: FormatBytes(fileInfo.Size()),
		CreatedDate:   i18n.FormatDate(fileInfo.ModTime(), normalizedLang),
		ModifiedDate:  i18n.FormatDateTime(fileInfo.ModTime(), normalizedLang),
		IsImage:       typeDef.Category == filetype.CategoryImage,
		Lang:          normalizedLang,
	}, nil
}

func (s *Service) GetFilePreview(relativePath, lang string) (models.PreviewData, error) {
	absolutePath, err := s.ResolvePath(relativePath)
	if err != nil {
		return models.PreviewData{}, err
	}

	fileInfo, err := os.Stat(absolutePath)
	if err != nil || fileInfo.IsDir() {
		return models.PreviewData{}, ErrNotFound
	}

	extension := filepath.Ext(absolutePath)
	typeDef := filetype.Resolve(extension, false)
	normalizedLang := i18n.NormalizeLang(lang)

	preview := models.PreviewData{
		Name:          filepath.Base(absolutePath),
		RelPath:       filepath.ToSlash(relativePath),
		Type:          string(typeDef.Category),
		Extension:     strings.ToLower(extension),
		MimeType:      typeDef.MimeType,
		FormattedSize: FormatBytes(fileInfo.Size()),
		ModTime:       i18n.FormatDateTime(fileInfo.ModTime(), normalizedLang),
		LanguageClass: typeDef.LanguageClass,
		MaterialIcon:  typeDef.Icon,
		Lang:          normalizedLang,
	}

	isTextual := typeDef.Category == filetype.CategoryCode || typeDef.Category == filetype.CategoryMarkdown
	if !isTextual {
		return preview, nil
	}

	if fileInfo.Size() > maxInlinePreviewBytes {
		if normalizedLang == i18n.LangEN {
			preview.Content = "[File too large for inline text preview]"
		} else {
			preview.Content = "[Archivo demasiado grande para vista previa en texto]"
		}
		return preview, nil
	}

	fileContent, err := os.ReadFile(absolutePath)
	if err != nil {
		if normalizedLang == i18n.LangEN {
			preview.Content = "[Error reading file]"
		} else {
			preview.Content = "[Error al leer el archivo]"
		}
		return preview, nil
	}

	preview.Content = string(fileContent)
	preview.LineCount = strings.Count(preview.Content, "\n") + 1
	return preview, nil
}

func (s *Service) SaveUploadedFile(targetDirectoryRelativePath, originalFilename string, fileReader io.Reader) error {
	cleanFilename, err := validateItemName(originalFilename)
	if err != nil {
		return err
	}

	targetDirectory, err := s.ResolvePath(targetDirectoryRelativePath)
	if err != nil {
		return err
	}

	if _, err := os.Stat(targetDirectory); err != nil {
		return ErrNotFound
	}

	destinationPath := filepath.Join(targetDirectory, cleanFilename)
	if err := ensureNotSymlink(destinationPath); err != nil {
		return err
	}

	destinationFile, err := os.OpenFile(destinationPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer destinationFile.Close()

	_, err = io.Copy(destinationFile, fileReader)
	return err
}

func (s *Service) CreateFolder(parentRelativePath, folderName string) error {
	cleanFolderName, err := validateItemName(folderName)
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

	return os.MkdirAll(newFolderPath, 0700)
}

func (s *Service) CreateFile(parentRelativePath, filename string, content []byte) error {
	cleanFilename, err := validateItemName(filename)
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

	targetFilePath := filepath.Join(targetDirectory, cleanFilename)
	if err := ensureNotSymlink(targetFilePath); err != nil {
		return err
	}

	return os.WriteFile(targetFilePath, content, 0600)
}

func (s *Service) DeleteItem(relativePath string) error {
	normalized := strings.ReplaceAll(strings.TrimSpace(relativePath), "\\", "/")
	cleanRelative := filepath.Clean(filepath.FromSlash(normalized))
	if cleanRelative == "" || cleanRelative == "." || cleanRelative == "/" {
		return ErrCannotDeleteRoot
	}

	if strings.HasPrefix(cleanRelative, "..") {
		return ErrInvalidPath
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

	absoluteBase, err := filepath.Abs(s.baseDirectory)
	if err != nil {
		return err
	}
	if evaluatedBase, err := filepath.EvalSymlinks(absoluteBase); err == nil {
		absoluteBase = evaluatedBase
	}

	if absolutePath == absoluteBase {
		return ErrCannotDeleteRoot
	}

	return os.RemoveAll(absolutePath)
}

func (s *Service) GetDownloadFile(relativePath string) (string, string, string, bool, error) {
	absolutePath, err := s.ResolvePath(relativePath)
	if err != nil {
		return "", "", "", false, err
	}

	fileInfo, err := os.Stat(absolutePath)
	if err != nil || fileInfo.IsDir() {
		return "", "", "", false, ErrNotFound
	}

	extension := strings.ToLower(filepath.Ext(absolutePath))
	typeDef := filetype.Resolve(extension, false)
	filename := filepath.Base(absolutePath)

	dangerousExts := map[string]bool{
		".html": true, ".htm": true, ".xhtml": true, ".xht": true,
		".svg": true, ".svgz": true, ".xml": true, ".xsl": true,
		".xslt": true, ".mht": true, ".mhtml": true, ".shtml": true,
	}
	forceAttachment := dangerousExts[extension]
	return absolutePath, filename, typeDef.MimeType, forceAttachment, nil
}

func BuildBreadcrumbs(relativePath string) []models.Breadcrumb {
	normalized := strings.ReplaceAll(strings.TrimSpace(relativePath), "\\", "/")
	cleanRelative := filepath.Clean(filepath.FromSlash(normalized))
	if cleanRelative == "" || cleanRelative == "." || cleanRelative == "/" {
		return nil
	}

	pathParts := strings.Split(filepath.ToSlash(cleanRelative), "/")
	var breadcrumbs []models.Breadcrumb
	var currentPath string

	for _, part := range pathParts {
		if part == "" || part == "." {
			continue
		}
		if currentPath == "" {
			currentPath = part
		} else {
			currentPath = currentPath + "/" + part
		}
		breadcrumbs = append(breadcrumbs, models.Breadcrumb{
			Name: part,
			Path: currentPath,
		})
	}

	return breadcrumbs
}

func FormatBytes(bytesCount int64) string {
	if bytesCount < 0 {
		return "0 B"
	}
	const unit = 1024
	if bytesCount < unit {
		return fmt.Sprintf("%d B", bytesCount)
	}
	divider := int64(unit)
	exponent := 0
	units := "KMGTPE"
	for n := bytesCount / unit; n >= unit && exponent < len(units)-1; n /= unit {
		divider *= unit
		exponent++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytesCount)/float64(divider), units[exponent])
}

func countDirectoryChildren(directoryPath string) int {
	subEntries, err := os.ReadDir(directoryPath)
	if err != nil {
		return 0
	}
	childCount := 0
	for _, entry := range subEntries {
		if isRestrictedSegment(entry.Name()) {
			continue
		}
		childCount++
	}
	return childCount
}

func normalizeSortParams(sortBy, sortOrder string) (string, string) {
	validSorts := map[string]bool{
		models.SortByName:     true,
		models.SortByCreated:  true,
		models.SortByModified: true,
		models.SortBySize:     true,
	}
	normalizedSort := strings.ToLower(strings.TrimSpace(sortBy))
	if !validSorts[normalizedSort] {
		normalizedSort = models.SortByName
	}

	normalizedOrder := strings.ToLower(strings.TrimSpace(sortOrder))
	if normalizedOrder != models.SortOrderDesc {
		normalizedOrder = models.SortOrderAsc
	}

	return normalizedSort, normalizedOrder
}

func compareFileItems(a, b models.FileInfo, sortBy, sortOrder string) bool {
	isDesc := sortOrder == models.SortOrderDesc
	var isLess bool
	switch sortBy {
	case models.SortBySize:
		if a.IsDir && b.IsDir {
			isLess = a.ItemCount < b.ItemCount
		} else {
			isLess = a.Size < b.Size
		}
	case models.SortByCreated, models.SortByModified:
		isLess = a.ModTime.Before(b.ModTime)
	case models.SortByName:
		fallthrough
	default:
		isLess = strings.ToLower(a.Name) < strings.ToLower(b.Name)
	}

	if isDesc {
		return !isLess
	}
	return isLess
}

func sortFolders(folders []models.FileInfo, sortBy, sortOrder string) {
	sort.Slice(folders, func(i, j int) bool {
		return compareFileItems(folders[i], folders[j], sortBy, sortOrder)
	})
}

func sortFiles(files []models.FileInfo, sortBy, sortOrder string) {
	sort.Slice(files, func(i, j int) bool {
		return compareFileItems(files[i], files[j], sortBy, sortOrder)
	})
}
