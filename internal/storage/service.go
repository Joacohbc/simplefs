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

func (s *Service) ResolvePath(relativePath string) (string, error) {
	absoluteBase, err := filepath.Abs(s.baseDirectory)
	if err != nil {
		return "", fmt.Errorf("storage directory error: %w", err)
	}

	if evaluatedBase, err := filepath.EvalSymlinks(absoluteBase); err == nil {
		absoluteBase = evaluatedBase
	}

	cleanRelative := filepath.Clean(filepath.FromSlash(relativePath))
	if cleanRelative == "." || cleanRelative == "/" || cleanRelative == "" {
		cleanRelative = ""
	}

	if strings.HasPrefix(cleanRelative, "..") {
		return "", errors.New("invalid path access")
	}

	for _, segment := range strings.Split(cleanRelative, string(filepath.Separator)) {
		if isRestrictedSegment(segment) {
			return "", errors.New("access denied: protected item")
		}
	}

	targetPath := filepath.Join(absoluteBase, cleanRelative)

	relativeFromBase, err := filepath.Rel(absoluteBase, targetPath)
	if err != nil || strings.HasPrefix(relativeFromBase, "..") || relativeFromBase == ".." {
		return "", errors.New("access denied")
	}

	checkPath := targetPath
	if _, err := os.Lstat(targetPath); os.IsNotExist(err) {
		checkPath = filepath.Dir(targetPath)
	}

	if evaluated, err := filepath.EvalSymlinks(checkPath); err == nil {
		evaluatedRel, err := filepath.Rel(absoluteBase, evaluated)
		if err != nil || strings.HasPrefix(evaluatedRel, "..") || evaluatedRel == ".." {
			return "", errors.New("symlink target outside storage root")
		}
		if checkPath == targetPath {
			return evaluated, nil
		}
		return filepath.Join(evaluated, filepath.Base(targetPath)), nil
	}

	return targetPath, nil
}

func (s *Service) GetDirectoryPage(relativePath, searchQuery, viewMode, sortBy, sortOrder, lang string) (models.PageData, error) {
	absolutePath, err := s.ResolvePath(relativePath)
	if err != nil {
		return models.PageData{}, err
	}

	entries, err := os.ReadDir(absolutePath)
	if err != nil {
		return models.PageData{}, err
	}

	var folders []models.FileInfo
	var files []models.FileInfo
	searchLower := strings.ToLower(searchQuery)
	normalizedLang := i18n.NormalizeLang(lang)

	for _, entry := range entries {
		name := entry.Name()
		if isExcludedItem(name) {
			continue
		}

		if searchLower != "" && !strings.Contains(strings.ToLower(name), searchLower) {
			continue
		}

		entryInfo, err := entry.Info()
		if err != nil {
			continue
		}

		entryRelativePath := filepath.Join(relativePath, name)
		extension := filepath.Ext(name)
		typeDef := filetype.Resolve(extension, entry.IsDir())

		childCount := 0
		if entry.IsDir() {
			childCount = countDirectoryChildren(filepath.Join(absolutePath, name))
		}

		fileObject := models.FileInfo{
			Name:             name,
			RelPath:          filepath.ToSlash(entryRelativePath),
			IsDir:            entry.IsDir(),
			Size:             entryInfo.Size(),
			FormattedSize:    FormatBytes(entryInfo.Size()),
			ModTime:          entryInfo.ModTime(),
			FormattedMod:     i18n.FormatDate(entryInfo.ModTime(), normalizedLang),
			FormattedCreated: i18n.FormatDate(entryInfo.ModTime(), normalizedLang),
			ItemCount:        childCount,
			TypeLabel:        typeDef.Label,
			MaterialIcon:     typeDef.Icon,
			IconColorClass:   typeDef.ColorClass,
			IsImage:          typeDef.Category == filetype.CategoryImage,
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
		viewMode = "list"
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
		return models.FileDetailsData{}, err
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
		return models.PreviewData{}, errors.New("file not found")
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
	cleanFilename := filepath.Base(filepath.Clean(originalFilename))
	if cleanFilename == "" || cleanFilename == "." || cleanFilename == ".." || isRestrictedSegment(cleanFilename) {
		return errors.New("invalid filename")
	}

	targetDirectory, err := s.ResolvePath(targetDirectoryRelativePath)
	if err != nil {
		return err
	}

	destinationPath := filepath.Join(targetDirectory, cleanFilename)
	if fi, err := os.Lstat(destinationPath); err == nil {
		if fi.Mode()&os.ModeSymlink != 0 {
			return errors.New("cannot overwrite symlink")
		}
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
	trimmedName := strings.TrimSpace(folderName)
	if trimmedName == "" || strings.Contains(trimmedName, "/") || strings.Contains(trimmedName, "\\") || strings.Contains(trimmedName, "..") {
		return errors.New("invalid folder name")
	}

	cleanFolderName := filepath.Base(filepath.Clean(trimmedName))
	if cleanFolderName == "" || cleanFolderName == "." || cleanFolderName == ".." || isRestrictedSegment(cleanFolderName) {
		return errors.New("invalid folder name")
	}

	targetDirectory, err := s.ResolvePath(parentRelativePath)
	if err != nil {
		return err
	}

	newFolderPath := filepath.Join(targetDirectory, cleanFolderName)
	if fi, err := os.Lstat(newFolderPath); err == nil {
		if fi.Mode()&os.ModeSymlink != 0 {
			return errors.New("cannot create folder on symlink")
		}
	}

	return os.MkdirAll(newFolderPath, 0700)
}

func (s *Service) CreateFile(parentRelativePath, filename string, content []byte) error {
	trimmedFilename := strings.TrimSpace(filename)
	if trimmedFilename == "" || strings.Contains(trimmedFilename, "/") || strings.Contains(trimmedFilename, "\\") || strings.Contains(trimmedFilename, "..") {
		return errors.New("invalid filename")
	}

	cleanFilename := filepath.Base(filepath.Clean(trimmedFilename))
	if cleanFilename == "" || cleanFilename == "." || cleanFilename == ".." || isRestrictedSegment(cleanFilename) {
		return errors.New("invalid filename")
	}

	targetFilePath, err := s.ResolvePath(filepath.Join(parentRelativePath, cleanFilename))
	if err != nil {
		return err
	}

	if fi, err := os.Lstat(targetFilePath); err == nil {
		if fi.Mode()&os.ModeSymlink != 0 {
			return errors.New("cannot overwrite symlink")
		}
	}

	return os.WriteFile(targetFilePath, content, 0600)
}

func (s *Service) DeleteItem(relativePath string) error {
	cleanRelative := filepath.Clean(filepath.FromSlash(strings.TrimSpace(relativePath)))
	if cleanRelative == "" || cleanRelative == "." || cleanRelative == "/" {
		return errors.New("cannot delete root directory")
	}

	for _, segment := range strings.Split(cleanRelative, string(filepath.Separator)) {
		if isRestrictedSegment(segment) {
			return errors.New("cannot delete protected item")
		}
	}

	absolutePath, err := s.ResolvePath(relativePath)
	if err != nil {
		return err
	}

	absoluteBase, _ := filepath.Abs(s.baseDirectory)
	if evaluatedBase, err := filepath.EvalSymlinks(absoluteBase); err == nil {
		absoluteBase = evaluatedBase
	}

	if absolutePath == absoluteBase {
		return errors.New("cannot delete root directory")
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
		return "", "", "", false, errors.New("file not found")
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
	if relativePath == "" || relativePath == "." {
		return nil
	}

	pathParts := strings.Split(filepath.ToSlash(relativePath), "/")
	var breadcrumbs []models.Breadcrumb
	var currentPath string

	for _, part := range pathParts {
		if part == "" {
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
	const unit = 1024
	if bytesCount < unit {
		return fmt.Sprintf("%d B", bytesCount)
	}
	divider, exponent := int64(unit), 0
	for n := bytesCount / unit; n >= unit; n /= unit {
		divider *= unit
		exponent++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytesCount)/float64(divider), "KMGTPE"[exponent])
}

func isExcludedItem(name string) bool {
	return isRestrictedSegment(name)
}

func countDirectoryChildren(directoryPath string) int {
	subEntries, err := os.ReadDir(directoryPath)
	if err != nil {
		return 0
	}
	count := 0
	for _, entry := range subEntries {
		name := entry.Name()
		if !strings.HasPrefix(name, ".") && name != "node_modules" {
			count++
		}
	}
	return count
}

func normalizeSortParams(sortBy, sortOrder string) (string, string) {
	validSorts := map[string]bool{"name": true, "created": true, "modified": true, "size": true}
	normalizedSort := strings.ToLower(sortBy)
	if !validSorts[normalizedSort] {
		normalizedSort = "name"
	}

	normalizedOrder := strings.ToLower(sortOrder)
	if normalizedOrder != "desc" {
		normalizedOrder = "asc"
	}

	return normalizedSort, normalizedOrder
}

func sortFolders(folders []models.FileInfo, sortBy, sortOrder string) {
	isDesc := sortOrder == "desc"
	sort.Slice(folders, func(i, j int) bool {
		var less bool
		switch sortBy {
		case "size":
			less = folders[i].ItemCount < folders[j].ItemCount
		case "created", "modified":
			less = folders[i].ModTime.Before(folders[j].ModTime)
		case "name":
			fallthrough
		default:
			less = strings.ToLower(folders[i].Name) < strings.ToLower(folders[j].Name)
		}
		if isDesc {
			return !less
		}
		return less
	})
}

func sortFiles(files []models.FileInfo, sortBy, sortOrder string) {
	isDesc := sortOrder == "desc"
	sort.Slice(files, func(i, j int) bool {
		var less bool
		switch sortBy {
		case "size":
			less = files[i].Size < files[j].Size
		case "created", "modified":
			less = files[i].ModTime.Before(files[j].ModTime)
		case "name":
			fallthrough
		default:
			less = strings.ToLower(files[i].Name) < strings.ToLower(files[j].Name)
		}
		if isDesc {
			return !less
		}
		return less
	})
}
