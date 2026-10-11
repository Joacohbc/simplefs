package storage

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"

	"simplefs/internal/filetype"
	"simplefs/internal/i18n"
	"simplefs/internal/models"
)

const (
	maxInlinePreviewBytes = 1000 * 1024
	maxNameLength         = 255

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

type Options struct {
	FileMode     os.FileMode
	DirMode      os.FileMode
	HideDotfiles bool
}

func DefaultOptions() Options {
	return Options{FileMode: 0644, DirMode: 0755}
}

type Service struct {
	baseDirectory string
	options       Options
}

func NewService(baseDirectory string) *Service {
	return NewServiceWithOptions(baseDirectory, DefaultOptions())
}

func NewServiceWithOptions(baseDirectory string, options Options) *Service {
	defaults := DefaultOptions()
	if options.FileMode == 0 {
		options.FileMode = defaults.FileMode
	}
	if options.DirMode == 0 {
		options.DirMode = defaults.DirMode
	}
	return &Service{baseDirectory: baseDirectory, options: options}
}

// IsReservedName reports names SimpleFS keeps for itself: the trash, its internal
// state directory and in-flight upload files.
func IsReservedName(name string) bool {
	trimmed := strings.TrimSpace(name)
	return trimmed == trashDirName || trimmed == internalDirName || strings.HasPrefix(trimmed, internalTmpPrefix)
}

func isRestrictedSegment(name string) bool {
	return IsReservedName(name)
}

func (s *Service) isHidden(name string) bool {
	if IsReservedName(name) {
		return true
	}
	return s.options.HideDotfiles && strings.HasPrefix(name, ".")
}

func (s *Service) validateItemName(name string) (string, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" || trimmed == "." || trimmed == ".." || len(trimmed) > maxNameLength {
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

func (s *Service) absoluteBase() (string, error) {
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

func (s *Service) ResolvePath(relativePath string) (string, error) {
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

func (s *Service) internalTempDir() string {
	return filepath.Join(s.baseDirectory, internalDirName, "tmp")
}

// CleanupTemp removes partial uploads left behind by an interrupted process.
func (s *Service) CleanupTemp() error {
	tempDir := s.internalTempDir()
	if err := os.RemoveAll(tempDir); err != nil {
		return err
	}
	return os.MkdirAll(tempDir, 0700)
}

func (s *Service) createTempFile() (*os.File, error) {
	tempDir := s.internalTempDir()
	if err := os.MkdirAll(tempDir, 0700); err != nil {
		return nil, err
	}
	return os.CreateTemp(tempDir, internalTmpPrefix+"*.part")
}

func folderStyle(name string) (string, filetype.Accent) {
	tokens := strings.FieldsFunc(strings.ToLower(name), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	styles := []struct {
		icon   string
		accent filetype.Accent
		words  []string
	}{
		{"move_to_inbox", filetype.AccentPrimary, []string{"inbox", "incoming", "entrada", "recibidos"}},
		{"outbox", filetype.AccentPrimary, []string{"outbox", "outgoing", "salida", "enviados"}},
		{"download", filetype.AccentPrimary, []string{"downloads", "download", "descargas"}},
		{"folder_code", filetype.AccentCode, []string{"src", "source", "code", "cmd", "pkg", "internal", "lib", "app", "apps"}},
		{"perm_media", filetype.AccentMedia, []string{"assets", "asset", "media", "img", "imgs", "image", "images", "photo", "photos", "pictures", "static", "video", "videos"}},
		{"topic", filetype.AccentSheet, []string{"docs", "doc", "documents", "documentos", "guide", "guides", "spec", "specs", "manual", "notes", "wiki"}},
		{"terminal", filetype.AccentArchive, []string{"build", "dist", "bin", "out", "output", "script", "scripts", "target", "release", "releases"}},
	}
	for _, style := range styles {
		for _, token := range tokens {
			for _, word := range style.words {
				if token == word {
					return style.icon, style.accent
				}
			}
		}
	}
	return "folder", filetype.AccentPrimary
}

func (s *Service) toFileInfo(name string, info fs.FileInfo, relativeDir, absoluteDir, lang string) models.FileInfo {
	entryRelativePath := filepath.ToSlash(filepath.Join(relativeDir, name))
	extension := filepath.Ext(name)
	typeDef := filetype.Resolve(name, info.IsDir())
	icon := typeDef.Icon
	accent := filetype.AccentFor(typeDef)

	childCount := 0
	if info.IsDir() {
		childCount = s.countDirectoryChildren(filepath.Join(absoluteDir, name))
		icon, accent = folderStyle(name)
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

var errStopWalk = errors.New("stop walk")

// walk visits every visible entry below rootAbs without following symlinks.
// visit returns false to stop early. The bool result reports truncation.
func (s *Service) walk(rootAbs, rootRel string, visit func(walkHit) bool) (bool, error) {
	visited := 0
	truncated := false
	err := filepath.WalkDir(rootAbs, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			if path == rootAbs {
				return walkErr
			}
			return nil
		}
		if path == rootAbs {
			return nil
		}
		if s.isHidden(entry.Name()) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		visited++
		if visited > MaxWalkEntries {
			truncated = true
			return errStopWalk
		}
		info, err := entry.Info()
		if err != nil {
			return nil
		}
		absoluteDir := filepath.Dir(path)
		relFromRoot, err := filepath.Rel(rootAbs, absoluteDir)
		if err != nil {
			return nil
		}
		relativeDir := filepath.Join(rootRel, relFromRoot)
		if !visit(walkHit{name: entry.Name(), info: info, relativeDir: relativeDir, absoluteDir: absoluteDir}) {
			truncated = true
			return errStopWalk
		}
		return nil
	})
	if errors.Is(err, errStopWalk) {
		err = nil
	}
	return truncated, err
}

func (s *Service) GetDirectoryPage(relativePath, searchQuery, viewMode, sortBy, sortOrder, lang string) (models.PageData, error) {
	absolutePath, err := s.ResolvePath(relativePath)
	if err != nil {
		return models.PageData{}, err
	}

	stat, err := os.Stat(absolutePath)
	if err != nil || !stat.IsDir() {
		return models.PageData{}, ErrNotFound
	}

	normalizedLang := i18n.NormalizeLang(lang)
	cleanRelative := cleanRelativePath(relativePath)
	query := strings.TrimSpace(searchQuery)

	var folders, files []models.FileInfo
	truncated := false

	if query == "" {
		entries, err := os.ReadDir(absolutePath)
		if err != nil {
			return models.PageData{}, ErrNotFound
		}
		for _, entry := range entries {
			if s.isHidden(entry.Name()) {
				continue
			}
			info, err := entry.Info()
			if err != nil {
				continue
			}
			item := s.toFileInfo(entry.Name(), info, cleanRelative, absolutePath, normalizedLang)
			item.ParentPath = ""
			if info.IsDir() {
				folders = append(folders, item)
			} else {
				files = append(files, item)
			}
		}
	} else {
		queryLower := strings.ToLower(query)
		matches := 0
		truncated, err = s.walk(absolutePath, cleanRelative, func(hit walkHit) bool {
			if !strings.Contains(strings.ToLower(hit.name), queryLower) {
				return true
			}
			item := s.toFileInfo(hit.name, hit.info, hit.relativeDir, hit.absoluteDir, normalizedLang)
			if hit.info.IsDir() {
				folders = append(folders, item)
			} else {
				files = append(files, item)
			}
			matches++
			return matches < MaxSearchResults
		})
		if err != nil {
			return models.PageData{}, ErrNotFound
		}
	}

	normalizedSortBy, normalizedSortOrder := NormalizeSortParams(sortBy, sortOrder)
	sortItems(folders, normalizedSortBy, normalizedSortOrder)
	sortItems(files, normalizedSortBy, normalizedSortOrder)

	return models.PageData{
		Path:        filepath.ToSlash(cleanRelative),
		Query:       query,
		SortBy:      normalizedSortBy,
		SortOrder:   normalizedSortOrder,
		Breadcrumbs: BuildBreadcrumbs(relativePath),
		Folders:     folders,
		Files:       files,
		ViewMode:    NormalizeViewMode(viewMode),
		Lang:        normalizedLang,
		Truncated:   truncated,
	}, nil
}

// GetRecentPage lists the most recently modified files of the whole storage tree.
func (s *Service) GetRecentPage(viewMode, lang string, limit int) (models.PageData, error) {
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

func (s *Service) recentFiles(limit int, lang string) ([]models.FileInfo, bool, error) {
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

// Stamp returns a cheap fingerprint of a view so clients can poll for changes.
func (s *Service) Stamp(scope, relativePath string) (string, error) {
	hasher := fnv.New64a()
	switch scope {
	case "trash":
		writeDirStamp(hasher, s.getTrashDir(), func(name string) bool { return name == trashMetaFileName })
	case "recent":
		absoluteBase, err := s.absoluteBase()
		if err != nil {
			return "", err
		}
		count := 0
		var latest time.Time
		var total int64
		if _, err := s.walk(absoluteBase, "", func(hit walkHit) bool {
			count++
			total += hit.info.Size()
			if hit.info.ModTime().After(latest) {
				latest = hit.info.ModTime()
			}
			return true
		}); err != nil {
			return "", err
		}
		fmt.Fprintf(hasher, "%d|%d|%d", count, total, latest.UnixNano())
	default:
		absolutePath, err := s.ResolvePath(relativePath)
		if err != nil {
			return "", err
		}
		if stat, err := os.Stat(absolutePath); err != nil || !stat.IsDir() {
			return "", ErrNotFound
		}
		writeDirStamp(hasher, absolutePath, s.isHidden)
	}
	return fmt.Sprintf("%016x", hasher.Sum64()), nil
}

func writeDirStamp(w io.Writer, directory string, skip func(string) bool) {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if skip(entry.Name()) {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		fmt.Fprintf(w, "%s|%d|%d\n", entry.Name(), info.Size(), info.ModTime().UnixNano())
	}
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

// WriteFile streams r into a temporary file and atomically moves it into place,
// so readers never observe a partially written file.
func (s *Service) WriteFile(directoryRelativePath, filename string, reader io.Reader, options WriteOptions) (WriteResult, error) {
	cleanFilename, err := s.validateItemName(filename)
	if err != nil {
		return WriteResult{}, err
	}

	targetDirectory, err := s.ResolvePath(directoryRelativePath)
	if err != nil {
		return WriteResult{}, err
	}

	if options.CreateParents {
		if err := os.MkdirAll(targetDirectory, s.options.DirMode); err != nil {
			return WriteResult{}, err
		}
	}

	directoryInfo, err := os.Stat(targetDirectory)
	if err != nil {
		return WriteResult{}, ErrNotFound
	}
	if !directoryInfo.IsDir() {
		return WriteResult{}, ErrNotADirectory
	}

	tempFile, err := s.createTempFile()
	if err != nil {
		return WriteResult{}, fmt.Errorf("cannot create temporary file: %w", err)
	}
	tempPath := tempFile.Name()
	placed := false
	defer func() {
		if !placed {
			_ = os.Remove(tempPath)
		}
	}()

	hasher := sha256.New()
	written, copyErr := io.Copy(io.MultiWriter(tempFile, hasher), reader)
	closeErr := tempFile.Close()
	if copyErr != nil {
		return WriteResult{}, copyErr
	}
	if closeErr != nil {
		return WriteResult{}, closeErr
	}
	if err := os.Chmod(tempPath, s.options.FileMode); err != nil {
		return WriteResult{}, err
	}

	cleanDirectory := cleanRelativePath(directoryRelativePath)
	finalName, overwritten, err := s.placeFile(tempPath, targetDirectory, cleanDirectory, cleanFilename, options.Conflict)
	if err != nil {
		return WriteResult{}, err
	}
	placed = true

	return WriteResult{
		Name:        finalName,
		Path:        filepath.ToSlash(filepath.Join(cleanDirectory, finalName)),
		Size:        written,
		SHA256:      hex.EncodeToString(hasher.Sum(nil)),
		Overwritten: overwritten,
		Renamed:     finalName != cleanFilename,
	}, nil
}

func (s *Service) placeFile(tempPath, targetDirectory, relativeDirectory, name string, policy ConflictPolicy) (string, bool, error) {
	destination := filepath.Join(targetDirectory, name)

	if policy == ConflictOverwrite {
		existing, err := os.Lstat(destination)
		if err == nil {
			if existing.Mode()&os.ModeSymlink != 0 {
				return "", false, ErrSymlinkOverwrite
			}
			if existing.IsDir() {
				return "", false, ErrIsDirectory
			}
			if err := s.MoveToTrash(filepath.Join(relativeDirectory, name)); err != nil {
				return "", false, err
			}
		}
		if err := renameOrCopy(tempPath, destination, s.options.FileMode); err != nil {
			return "", false, err
		}
		return name, existing != nil, nil
	}

	candidate := name
	for attempt := 1; attempt <= 10000; attempt++ {
		err := linkNoClobber(tempPath, filepath.Join(targetDirectory, candidate), s.options.FileMode)
		if err == nil {
			return candidate, false, nil
		}
		if !errors.Is(err, fs.ErrExist) {
			return "", false, err
		}
		if policy == ConflictFail {
			return "", false, ErrAlreadyExists
		}
		candidate = numberedName(name, attempt)
	}
	return "", false, ErrAlreadyExists
}

// linkNoClobber moves src to dst failing with fs.ErrExist when dst exists.
// A hard link gives an atomic no-overwrite rename; filesystems without hard links
// fall back to an existence check followed by a rename.
func linkNoClobber(src, dst string, mode os.FileMode) error {
	if err := os.Link(src, dst); err == nil {
		return os.Remove(src)
	} else if errors.Is(err, fs.ErrExist) {
		return fs.ErrExist
	}
	if _, err := os.Lstat(dst); err == nil {
		return fs.ErrExist
	}
	return renameOrCopy(src, dst, mode)
}

func renameOrCopy(src, dst string, mode os.FileMode) error {
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	if err := copyFile(src, dst, mode); err != nil {
		return err
	}
	return os.Remove(src)
}

func copyFile(src, dst string, mode os.FileMode) error {
	source, err := os.Open(src)
	if err != nil {
		return err
	}
	defer source.Close()

	destination, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(destination, source); err != nil {
		destination.Close()
		return err
	}
	return destination.Close()
}

func numberedName(name string, number int) string {
	extension := filepath.Ext(name)
	base := strings.TrimSuffix(name, extension)
	if base == "" {
		base, extension = name, ""
	}
	return fmt.Sprintf("%s (%d)%s", base, number, extension)
}

func uniqueName(directory, name string) string {
	candidate := name
	for number := 1; number <= 10000; number++ {
		if _, err := os.Lstat(filepath.Join(directory, candidate)); errors.Is(err, fs.ErrNotExist) {
			return candidate
		}
		candidate = numberedName(name, number)
	}
	return candidate + "-" + randomSuffix()
}

func randomSuffix() string {
	buffer := make([]byte, 4)
	_, _ = rand.Read(buffer)
	return hex.EncodeToString(buffer)
}

func (s *Service) SaveUploadedFile(targetDirectoryRelativePath, originalFilename string, fileReader io.Reader) error {
	_, err := s.WriteFile(targetDirectoryRelativePath, originalFilename, fileReader, WriteOptions{Conflict: ConflictRename})
	return err
}

func (s *Service) CreateFile(parentRelativePath, filename string, content []byte) error {
	_, err := s.WriteFile(parentRelativePath, filename, bytes.NewReader(content), WriteOptions{Conflict: ConflictRename})
	return err
}

func (s *Service) CreateFolder(parentRelativePath, folderName string) error {
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
func (s *Service) CreateFolderPath(relativePath string) error {
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

func (s *Service) validateDeletable(relativePath string) (string, error) {
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

func (s *Service) DeleteItem(relativePath string) error {
	if _, err := s.validateDeletable(relativePath); err != nil {
		return err
	}
	return s.MoveToTrash(relativePath)
}

func (s *Service) DeleteItemPermanently(relativePath string) error {
	absolutePath, err := s.validateDeletable(relativePath)
	if err != nil {
		return err
	}
	if _, err := os.Lstat(absolutePath); err != nil {
		return ErrNotFound
	}
	return os.RemoveAll(absolutePath)
}

// isInlineSafe allowlists the types a browser may render directly from /download.
// Everything else is served as an attachment inside a sandbox CSP.
func isInlineSafe(definition filetype.Definition, extension string) bool {
	switch definition.Category {
	case filetype.CategoryImage:
		return extension != ".svg" && extension != ".svgz"
	case filetype.CategoryVideo, filetype.CategoryAudio, filetype.CategoryPDF:
		return true
	}
	return definition.MimeType == "text/plain"
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

	filename := filepath.Base(absolutePath)
	extension := strings.ToLower(filepath.Ext(filename))
	typeDef := filetype.Resolve(filename, false)
	return absolutePath, filename, typeDef.MimeType, !isInlineSafe(typeDef, extension), nil
}

type Entry struct {
	Name     string    `json:"name"`
	Path     string    `json:"path"`
	Type     string    `json:"type"`
	Size     int64     `json:"size"`
	Modified time.Time `json:"modified"`
	Mime     string    `json:"mime,omitempty"`
	SHA256   string    `json:"sha256,omitempty"`
}

func toEntry(name, relativePath string, info fs.FileInfo) Entry {
	entry := Entry{
		Name:     name,
		Path:     filepath.ToSlash(relativePath),
		Type:     "file",
		Size:     info.Size(),
		Modified: info.ModTime().UTC(),
	}
	if info.IsDir() {
		entry.Type = "dir"
		entry.Size = 0
		return entry
	}
	entry.Mime = filetype.Resolve(name, false).MimeType
	return entry
}

// List returns the visible entries of a directory, or the entry itself for a file.
func (s *Service) List(relativePath string) (Entry, []Entry, error) {
	absolutePath, err := s.ResolvePath(relativePath)
	if err != nil {
		return Entry{}, nil, err
	}
	info, err := os.Stat(absolutePath)
	if err != nil {
		return Entry{}, nil, ErrNotFound
	}
	cleanRelative := cleanRelativePath(relativePath)
	self := toEntry(info.Name(), cleanRelative, info)
	if cleanRelative == "" {
		self.Name = "/"
	}
	if !info.IsDir() {
		return self, nil, nil
	}

	dirEntries, err := os.ReadDir(absolutePath)
	if err != nil {
		return Entry{}, nil, ErrNotFound
	}
	entries := make([]Entry, 0, len(dirEntries))
	for _, dirEntry := range dirEntries {
		if s.isHidden(dirEntry.Name()) {
			continue
		}
		childInfo, err := dirEntry.Info()
		if err != nil {
			continue
		}
		entries = append(entries, toEntry(dirEntry.Name(), filepath.Join(cleanRelative, dirEntry.Name()), childInfo))
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Type != entries[j].Type {
			return entries[i].Type == "dir"
		}
		return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
	})
	return self, entries, nil
}

// Stat returns metadata of a single entry, optionally with its SHA-256 checksum.
func (s *Service) Stat(relativePath string, withChecksum bool) (Entry, error) {
	absolutePath, err := s.ResolvePath(relativePath)
	if err != nil {
		return Entry{}, err
	}
	info, err := os.Stat(absolutePath)
	if err != nil {
		return Entry{}, ErrNotFound
	}
	entry := toEntry(info.Name(), cleanRelativePath(relativePath), info)
	if withChecksum && !info.IsDir() {
		file, err := os.Open(absolutePath)
		if err != nil {
			return Entry{}, err
		}
		defer file.Close()
		hasher := sha256.New()
		if _, err := io.Copy(hasher, file); err != nil {
			return Entry{}, err
		}
		entry.SHA256 = hex.EncodeToString(hasher.Sum(nil))
	}
	return entry, nil
}

// Search finds entries whose name contains query below relativePath.
func (s *Service) Search(relativePath, query string, limit int) ([]Entry, bool, error) {
	absolutePath, err := s.ResolvePath(relativePath)
	if err != nil {
		return nil, false, err
	}
	if limit <= 0 || limit > MaxSearchResults {
		limit = MaxSearchResults
	}
	queryLower := strings.ToLower(strings.TrimSpace(query))
	var results []Entry
	truncated, err := s.walk(absolutePath, cleanRelativePath(relativePath), func(hit walkHit) bool {
		if queryLower != "" && !strings.Contains(strings.ToLower(hit.name), queryLower) {
			return true
		}
		results = append(results, toEntry(hit.name, filepath.Join(hit.relativeDir, hit.name), hit.info))
		return len(results) < limit
	})
	if err != nil {
		return nil, false, ErrNotFound
	}
	return results, truncated, nil
}

// Recent returns the most recently modified files of the whole storage tree.
func (s *Service) Recent(limit int) ([]Entry, bool, error) {
	files, truncated, err := s.recentFiles(limit, i18n.LangEN)
	if err != nil {
		return nil, false, err
	}
	entries := make([]Entry, 0, len(files))
	for _, file := range files {
		entries = append(entries, Entry{
			Name:     file.Name,
			Path:     file.RelPath,
			Type:     "file",
			Size:     file.Size,
			Modified: file.ModTime.UTC(),
			Mime:     filetype.Resolve(file.Name, false).MimeType,
		})
	}
	return entries, truncated, nil
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

func (s *Service) countDirectoryChildren(directoryPath string) int {
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
