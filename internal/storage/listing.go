package storage

import (
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"simplefs/internal/filetype"
	"simplefs/internal/i18n"
	"simplefs/internal/models"
	"sort"
	"strings"
	"time"
)

func (s *FileStore) toFileInfo(name string, info fs.FileInfo, relativeDir, absoluteDir, lang string) models.FileInfo {
	entryRelativePath := filepath.ToSlash(filepath.Join(relativeDir, name))
	extension := filepath.Ext(name)
	typeDef := filetype.Resolve(name, info.IsDir())
	icon := typeDef.Icon
	accent := filetype.AccentFor(typeDef)

	childCount := 0
	if info.IsDir() {
		childCount = s.countDirectoryChildren(filepath.Join(absoluteDir, name))
		style := styleForFolder(name)
		icon, accent = style.icon, style.accent
	}
	classes := filetype.ClassesFor(accent)

	parentPath := filepath.ToSlash(relativeDir)
	if parentPath == "." {
		parentPath = ""
	}

	return models.FileInfo{
		Name:           name,
		RelPath:        entryRelativePath,
		IsDir:          info.IsDir(),
		Size:           info.Size(),
		FormattedSize:  FormatBytes(info.Size()),
		ModTime:        info.ModTime(),
		FormattedMod:   i18n.FormatDate(info.ModTime(), lang),
		ItemCount:      childCount,
		TypeLabel:      typeDef.Label,
		MaterialIcon:   icon,
		IconColorClass: classes.Text,
		TileClass:      classes.Tile,
		DotClass:       classes.Dot,
		IsImage:        typeDef.Category == filetype.CategoryImage,
		IsZip:          !info.IsDir() && strings.EqualFold(extension, ".zip"),
		Category:       string(typeDef.Category),
		Extension:      strings.TrimPrefix(strings.ToUpper(extension), "."),
		ParentPath:     parentPath,
	}
}

type walkHit struct {
	name        string
	info        fs.FileInfo
	relativeDir string
	absoluteDir string
}

type walkVisitor func(walkHit) (keepWalking bool)

var errStopWalk = errors.New("stop walk")

func skipHiddenEntry(entry fs.DirEntry) error {
	if entry.IsDir() {
		return filepath.SkipDir
	}
	return nil
}

func newWalkHit(rootAbs, rootRel, path string, entry fs.DirEntry) (walkHit, error) {
	info, err := entry.Info()
	if err != nil {
		return walkHit{}, err
	}
	absoluteDir := filepath.Dir(path)
	relativeFromRoot, err := filepath.Rel(rootAbs, absoluteDir)
	if err != nil {
		return walkHit{}, err
	}
	return walkHit{
		name:        entry.Name(),
		info:        info,
		relativeDir: filepath.Join(rootRel, relativeFromRoot),
		absoluteDir: absoluteDir,
	}, nil
}

// walk visits every visible entry below rootAbs without following symlinks,
// stopping when visit returns false or after MaxWalkEntries entries.
func (s *FileStore) walk(rootAbs, rootRel string, visit walkVisitor) (wasTruncated bool, err error) {
	visitedEntries := 0
	err = filepath.WalkDir(rootAbs, func(path string, entry fs.DirEntry, walkErr error) error {
		isRoot := path == rootAbs
		if walkErr != nil && isRoot {
			return walkErr
		}
		if walkErr != nil || isRoot {
			return nil
		}
		if s.isHidden(entry.Name()) {
			return skipHiddenEntry(entry)
		}
		visitedEntries++
		if visitedEntries > MaxWalkEntries {
			wasTruncated = true
			return errStopWalk
		}
		hit, err := newWalkHit(rootAbs, rootRel, path, entry)
		if err != nil {
			return nil
		}
		if !visit(hit) {
			wasTruncated = true
			return errStopWalk
		}
		return nil
	})
	if errors.Is(err, errStopWalk) {
		err = nil
	}
	return wasTruncated, err
}

type directoryItems struct {
	folders      []models.FileInfo
	files        []models.FileInfo
	wasTruncated bool
}

func (items *directoryItems) add(item models.FileInfo) {
	if item.IsDir {
		items.folders = append(items.folders, item)
		return
	}
	items.files = append(items.files, item)
}

func (s *FileStore) listDirectory(absolutePath, relativePath, lang string) (directoryItems, error) {
	var items directoryItems
	entries, err := os.ReadDir(absolutePath)
	if err != nil {
		return items, ErrNotFound
	}
	for _, entry := range entries {
		if s.isHidden(entry.Name()) {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		item := s.toFileInfo(entry.Name(), info, relativePath, absolutePath, lang)
		item.ParentPath = ""
		items.add(item)
	}
	return items, nil
}

func (s *FileStore) searchDirectory(absolutePath, relativePath, query, lang string) (directoryItems, error) {
	var items directoryItems
	queryLower := strings.ToLower(query)
	matchCount := 0
	wasTruncated, err := s.walk(absolutePath, relativePath, func(hit walkHit) bool {
		if !strings.Contains(strings.ToLower(hit.name), queryLower) {
			return true
		}
		items.add(s.toFileInfo(hit.name, hit.info, hit.relativeDir, hit.absoluteDir, lang))
		matchCount++
		return matchCount < MaxSearchResults
	})
	if err != nil {
		return items, ErrNotFound
	}
	items.wasTruncated = wasTruncated
	return items, nil
}

func (s *FileStore) GetDirectoryPage(relativePath, searchQuery, viewMode, sortBy, sortOrder, lang string) (models.PageData, error) {
	absolutePath, err := s.ResolvePath(relativePath)
	if err != nil {
		return models.PageData{}, err
	}
	if stat, err := os.Stat(absolutePath); err != nil || !stat.IsDir() {
		return models.PageData{}, ErrNotFound
	}

	normalizedLang := i18n.NormalizeLang(lang)
	cleanRelative := cleanRelativePath(relativePath)
	query := strings.TrimSpace(searchQuery)

	var items directoryItems
	if query == "" {
		items, err = s.listDirectory(absolutePath, cleanRelative, normalizedLang)
	} else {
		items, err = s.searchDirectory(absolutePath, cleanRelative, query, normalizedLang)
	}
	if err != nil {
		return models.PageData{}, err
	}

	normalizedSortBy, normalizedSortOrder := NormalizeSortParams(sortBy, sortOrder)
	sortItems(items.folders, normalizedSortBy, normalizedSortOrder)
	sortItems(items.files, normalizedSortBy, normalizedSortOrder)

	return models.PageData{
		Path:        filepath.ToSlash(cleanRelative),
		Query:       query,
		SortBy:      normalizedSortBy,
		SortOrder:   normalizedSortOrder,
		Breadcrumbs: BuildBreadcrumbs(relativePath),
		Folders:     items.folders,
		Files:       items.files,
		ViewMode:    NormalizeViewMode(viewMode),
		Lang:        normalizedLang,
		Truncated:   items.wasTruncated,
	}, nil
}

// GetRecentPage lists the most recently modified files of the whole storage tree.
func (s *FileStore) GetRecentPage(viewMode, lang string, limit int) (models.PageData, error) {
	normalizedLang := i18n.NormalizeLang(lang)
	files, truncated, err := s.recentFiles(limit, normalizedLang)
	if err != nil {
		return models.PageData{}, err
	}
	return models.PageData{
		SortBy:    models.SortByModified,
		SortOrder: models.SortOrderDesc,
		Files:     files,
		ViewMode:  NormalizeViewMode(viewMode),
		Lang:      normalizedLang,
		IsRecent:  true,
		Truncated: truncated,
	}, nil
}

func clampRecentLimit(limit int) int {
	if limit <= 0 {
		return DefaultRecentLimit
	}
	if limit > MaxRecentLimit {
		return MaxRecentLimit
	}
	return limit
}

func (s *FileStore) recentFiles(limit int, lang string) ([]models.FileInfo, bool, error) {
	absoluteBase, err := s.absoluteBase()
	if err != nil {
		return nil, false, err
	}
	var hits []walkHit
	truncated, err := s.walk(absoluteBase, "", func(hit walkHit) bool {
		if !hit.info.IsDir() {
			hits = append(hits, hit)
		}
		return true
	})
	if err != nil {
		return nil, false, err
	}
	sort.Slice(hits, func(i, j int) bool {
		return hits[i].info.ModTime().After(hits[j].info.ModTime())
	})
	limit = clampRecentLimit(limit)
	if len(hits) > limit {
		hits = hits[:limit]
	}
	files := make([]models.FileInfo, 0, len(hits))
	for _, hit := range hits {
		files = append(files, s.toFileInfo(hit.name, hit.info, hit.relativeDir, hit.absoluteDir, lang))
	}
	return files, truncated, nil
}

type StampScope string

const (
	StampScopeDirectory StampScope = "dir"
	StampScopeRecent    StampScope = "recent"
	StampScopeTrash     StampScope = "trash"
)

// Stamp returns a cheap fingerprint of a view so clients can poll for changes.
func (s *FileStore) Stamp(scope StampScope, relativePath string) (string, error) {
	fingerprint := fnv.New64a()
	var err error
	switch scope {
	case StampScopeTrash:
		writeDirectoryFingerprint(fingerprint, s.getTrashDir(), isTrashMetadata)
	case StampScopeRecent:
		err = s.writeTreeFingerprint(fingerprint)
	default:
		err = s.writeVisibleDirectoryFingerprint(fingerprint, relativePath)
	}
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%016x", fingerprint.Sum64()), nil
}

func isTrashMetadata(name string) bool {
	return name == trashMetaFileName
}

func (s *FileStore) writeTreeFingerprint(fingerprint io.Writer) error {
	absoluteBase, err := s.absoluteBase()
	if err != nil {
		return err
	}
	entryCount := 0
	var totalBytes int64
	var latestModification time.Time
	_, err = s.walk(absoluteBase, "", func(hit walkHit) bool {
		entryCount++
		totalBytes += hit.info.Size()
		if hit.info.ModTime().After(latestModification) {
			latestModification = hit.info.ModTime()
		}
		return true
	})
	fmt.Fprintf(fingerprint, "%d|%d|%d", entryCount, totalBytes, latestModification.UnixNano())
	return err
}

func (s *FileStore) writeVisibleDirectoryFingerprint(fingerprint io.Writer, relativePath string) error {
	absolutePath, err := s.ResolvePath(relativePath)
	if err != nil {
		return err
	}
	if stat, err := os.Stat(absolutePath); err != nil || !stat.IsDir() {
		return ErrNotFound
	}
	writeDirectoryFingerprint(fingerprint, absolutePath, s.isHidden)
	return nil
}

func writeDirectoryFingerprint(fingerprint io.Writer, directory string, shouldSkip func(name string) bool) {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if shouldSkip(entry.Name()) {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		fmt.Fprintf(fingerprint, "%s|%d|%d\n", entry.Name(), info.Size(), info.ModTime().UnixNano())
	}
}

func (s *FileStore) GetFileDetails(relativePath, lang string) (models.FileDetailsData, error) {
	absolutePath, err := s.ResolvePath(relativePath)
	if err != nil {
		return models.FileDetailsData{}, err
	}

	fileInfo, err := os.Stat(absolutePath)
	if err != nil {
		return models.FileDetailsData{}, ErrNotFound
	}

	normalizedLang := i18n.NormalizeLang(lang)
	cleanRelative := cleanRelativePath(relativePath)
	item := s.toFileInfo(fileInfo.Name(), fileInfo, filepath.Dir(cleanRelative), filepath.Dir(absolutePath), normalizedLang)
	if cleanRelative == "" {
		item.MaterialIcon = "home"
	}

	parentDirectory := filepath.ToSlash(filepath.Dir(cleanRelative))
	if parentDirectory == "." || parentDirectory == "" {
		parentDirectory = "/"
	} else {
		parentDirectory = "/" + parentDirectory
	}

	name := filepath.Base(absolutePath)
	if cleanRelative == "" {
		name = "/"
	}

	return models.FileDetailsData{
		Name:          name,
		RelPath:       filepath.ToSlash(cleanRelative),
		DirLocation:   parentDirectory,
		TypeLabel:     item.TypeLabel,
		MaterialIcon:  item.MaterialIcon,
		TileClass:     item.TileClass,
		FormattedSize: FormatBytes(fileInfo.Size()),
		ModifiedDate:  i18n.FormatDateTime(fileInfo.ModTime(), normalizedLang),
		IsImage:       item.IsImage,
		IsDir:         fileInfo.IsDir(),
		Lang:          normalizedLang,
	}, nil
}

func (s *FileStore) GetFilePreview(relativePath, lang string) (models.PreviewData, error) {
	absolutePath, err := s.ResolvePath(relativePath)
	if err != nil {
		return models.PreviewData{}, err
	}

	fileInfo, err := os.Stat(absolutePath)
	if err != nil || fileInfo.IsDir() {
		return models.PreviewData{}, ErrNotFound
	}

	extension := filepath.Ext(absolutePath)
	typeDef := filetype.Resolve(filepath.Base(absolutePath), false)
	normalizedLang := i18n.NormalizeLang(lang)

	preview := models.PreviewData{
		Name:          filepath.Base(absolutePath),
		RelPath:       filepath.ToSlash(cleanRelativePath(relativePath)),
		Type:          string(typeDef.Category),
		Extension:     strings.ToLower(extension),
		MimeType:      typeDef.MimeType,
		FormattedSize: FormatBytes(fileInfo.Size()),
		ModTime:       i18n.FormatDateTime(fileInfo.ModTime(), normalizedLang),
		LanguageClass: typeDef.LanguageClass,
		MaterialIcon:  typeDef.Icon,
		IconClass:     filetype.ClassesFor(filetype.AccentFor(typeDef)).Text,
		Lang:          normalizedLang,
	}

	isTextual := typeDef.Category == filetype.CategoryCode || typeDef.Category == filetype.CategoryMarkdown
	if !isTextual {
		return preview, nil
	}

	if fileInfo.Size() > maxInlinePreviewBytes {
		preview.Content = i18n.T(normalizedLang, "preview.too_large")
		return preview, nil
	}

	fileContent, err := os.ReadFile(absolutePath)
	if err != nil {
		preview.Content = i18n.T(normalizedLang, "preview.read_error")
		return preview, nil
	}

	preview.Content = string(fileContent)
	preview.LineCount = strings.Count(preview.Content, "\n") + 1
	return preview, nil
}

func BuildBreadcrumbs(relativePath string) []models.Breadcrumb {
	cleanRelative := cleanRelativePath(relativePath)
	if cleanRelative == "" {
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

func NormalizeViewMode(viewMode string) string {
	if viewMode == models.ViewModeGrid {
		return models.ViewModeGrid
	}
	return models.ViewModeList
}

func NormalizeSortParams(sortBy, sortOrder string) (string, string) {
	normalizedSort := strings.ToLower(strings.TrimSpace(sortBy))
	switch normalizedSort {
	case models.SortByName, models.SortByModified, models.SortBySize:
	case "created":
		normalizedSort = models.SortByModified
	default:
		normalizedSort = models.SortByName
	}

	normalizedOrder := strings.ToLower(strings.TrimSpace(sortOrder))
	if normalizedOrder != models.SortOrderDesc {
		normalizedOrder = models.SortOrderAsc
	}

	return normalizedSort, normalizedOrder
}

func compareFileItems(a, b models.FileInfo, sortBy, sortOrder string) bool {
	if sortOrder == models.SortOrderDesc {
		a, b = b, a
	}
	switch sortBy {
	case models.SortBySize:
		if a.IsDir && b.IsDir {
			return a.ItemCount < b.ItemCount
		}
		return a.Size < b.Size
	case models.SortByModified:
		return a.ModTime.Before(b.ModTime)
	}
	return strings.ToLower(a.Name) < strings.ToLower(b.Name)
}

func sortItems(items []models.FileInfo, sortBy, sortOrder string) {
	sort.SliceStable(items, func(i, j int) bool {
		return compareFileItems(items[i], items[j], sortBy, sortOrder)
	})
}
