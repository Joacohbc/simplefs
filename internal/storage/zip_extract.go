package storage

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

const (
	MaxZipFileCount              = 10000
	MaxZipTotalUncompressedBytes = 500 * 1024 * 1024
	MaxZipSingleFileBytes        = 200 * 1024 * 1024
	MaxZipCompressionRatio       = 100
	MaxZipPathDepth              = 30

	compressionRatioCheckThresholdBytes = 1024 * 1024
	fallbackExtractedFolderName         = "extracted_archive"
)

var (
	ErrZipEmpty               = errors.New("zip archive is empty")
	ErrZipTooManyFiles        = errors.New("zip archive exceeds maximum file count limit (10,000 files)")
	ErrZipTotalSizeLimit      = errors.New("zip archive uncompressed size exceeds maximum limit (500 MB)")
	ErrZipSingleFileSizeLimit = errors.New("zip entry uncompressed size exceeds limit (200 MB)")
	ErrZipCompressionRatio    = errors.New("zip entry exceeds compression ratio threshold (potential zip bomb)")
	ErrZipPathTraversal       = errors.New("illegal file path in zip entry (Zip Slip attempt detected)")
	ErrZipSymlinkForbidden    = errors.New("zip entry contains symlink or special device (forbidden for security)")
	ErrZipRestrictedSegment   = errors.New("zip entry contains protected or restricted folder name")
	ErrZipInvalidName         = errors.New("zip entry has invalid or dangerous characters in filename")
	ErrNotAZipArchive         = errors.New("uploaded file is not a valid zip archive")
	ErrNotADirectory          = errors.New("target path is not a directory")
)

const specialFileModes = os.ModeDevice | os.ModeNamedPipe | os.ModeSocket | os.ModeCharDevice | os.ModeIrregular

func hasControlCharacters(name string) bool {
	for _, r := range name {
		if r == 0 || (unicode.IsControl(r) && r != '\t') {
			return true
		}
	}
	return false
}

func isAbsoluteOrDriveRooted(normalizedName string) bool {
	hasDriveLetter := len(normalizedName) >= 2 && normalizedName[1] == ':'
	return strings.HasPrefix(normalizedName, "/") || hasDriveLetter
}

func hasParentSegment(normalizedName string) bool {
	for _, segment := range strings.Split(normalizedName, "/") {
		if segment == ".." {
			return true
		}
	}
	return false
}

func hasReservedSegment(segments []string) bool {
	for _, segment := range segments {
		if IsReservedName(segment) {
			return true
		}
	}
	return false
}

func exceedsCompressionRatio(uncompressedBytes, compressedBytes uint64) bool {
	isLargeEnoughToCheck := compressedBytes > 0 && uncompressedBytes > compressionRatioCheckThresholdBytes
	return isLargeEnoughToCheck && uncompressedBytes/compressedBytes > MaxZipCompressionRatio
}

func normalizeZipEntryName(name string) (normalized string, cleanRelative string) {
	normalized = strings.ReplaceAll(name, "\\", "/")
	return normalized, filepath.Clean(filepath.FromSlash(normalized))
}

// ValidateZipEntryHeader inspects a single zip file header for security threats before extraction.
func ValidateZipEntryHeader(entry *zip.File) error {
	if hasControlCharacters(entry.Name) {
		return ErrZipInvalidName
	}
	normalized, cleanRelative := normalizeZipEntryName(entry.Name)
	if isAbsoluteOrDriveRooted(normalized) || hasParentSegment(normalized) {
		return ErrZipPathTraversal
	}
	if cleanRelative == "." || cleanRelative == "" {
		return nil
	}
	if strings.HasPrefix(cleanRelative, "..") || filepath.IsAbs(cleanRelative) {
		return ErrZipPathTraversal
	}

	segments := strings.Split(cleanRelative, string(filepath.Separator))
	if len(segments) > MaxZipPathDepth {
		return ErrZipPathTraversal
	}
	if hasReservedSegment(segments) {
		return ErrZipRestrictedSegment
	}

	mode := entry.Mode()
	if mode&os.ModeSymlink != 0 || mode&specialFileModes != 0 {
		return ErrZipSymlinkForbidden
	}
	if entry.UncompressedSize64 > MaxZipSingleFileBytes {
		return ErrZipSingleFileSizeLimit
	}
	if exceedsCompressionRatio(entry.UncompressedSize64, entry.CompressedSize64) {
		return ErrZipCompressionRatio
	}
	return nil
}

// ValidateZipReader validates all entries in a zip archive prior to extracting anything to disk.
func ValidateZipReader(archive *zip.Reader) error {
	if len(archive.File) == 0 {
		return ErrZipEmpty
	}
	if len(archive.File) > MaxZipFileCount {
		return ErrZipTooManyFiles
	}

	var totalDeclaredBytes uint64
	for _, entry := range archive.File {
		if err := ValidateZipEntryHeader(entry); err != nil {
			return err
		}
		totalDeclaredBytes += entry.UncompressedSize64
		if totalDeclaredBytes > MaxZipTotalUncompressedBytes {
			return ErrZipTotalSizeLimit
		}
	}
	return nil
}

// ExtractZipReader safely extracts all entries of archive into targetFolderPath.
// Declared sizes are not trusted: limits are enforced again while decompressing,
// and everything extracted is removed if any entry fails.
func ExtractZipReader(archive *zip.Reader, targetFolderPath string) error {
	return extractZipReader(archive, targetFolderPath, DefaultOptions())
}

func extractZipReader(archive *zip.Reader, targetFolderPath string, options Options) (err error) {
	defer func() {
		if err != nil {
			_ = os.RemoveAll(targetFolderPath)
		}
	}()

	var totalExtractedBytes int64
	for _, entry := range archive.File {
		extractedBytes, err := extractZipEntry(entry, targetFolderPath, MaxZipTotalUncompressedBytes-totalExtractedBytes, options)
		if err != nil {
			return err
		}
		totalExtractedBytes += extractedBytes
	}
	return nil
}

func zipEntryDestination(entry *zip.File, targetFolderPath string) (string, error) {
	_, cleanRelative := normalizeZipEntryName(entry.Name)
	destination := filepath.Join(targetFolderPath, cleanRelative)
	relative, err := filepath.Rel(targetFolderPath, destination)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", ErrZipPathTraversal
	}
	return destination, nil
}

func isZipDirectoryEntry(entry *zip.File) bool {
	return entry.Mode().IsDir() || strings.HasSuffix(entry.Name, "/") || strings.HasSuffix(entry.Name, "\\")
}

func extractZipEntry(entry *zip.File, targetFolderPath string, remainingBudgetBytes int64, options Options) (int64, error) {
	_, cleanRelative := normalizeZipEntryName(entry.Name)
	if cleanRelative == "." || cleanRelative == "" {
		return 0, nil
	}
	destination, err := zipEntryDestination(entry, targetFolderPath)
	if err != nil {
		return 0, err
	}
	if isZipDirectoryEntry(entry) {
		return 0, os.MkdirAll(destination, options.DirMode)
	}
	if remainingBudgetBytes <= 0 {
		return 0, ErrZipTotalSizeLimit
	}
	if err := os.MkdirAll(filepath.Dir(destination), options.DirMode); err != nil {
		return 0, fmt.Errorf("failed to create parent directory in zip: %w", err)
	}
	if err := ensureNotSymlink(destination); err != nil {
		return 0, err
	}
	return writeZipEntry(entry, destination, remainingBudgetBytes, options.FileMode)
}

func writeZipEntry(entry *zip.File, destination string, remainingBudgetBytes int64, mode os.FileMode) (int64, error) {
	destinationFile, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return 0, fmt.Errorf("failed to create extracted file: %w", err)
	}
	defer destinationFile.Close()

	entryReader, err := entry.Open()
	if err != nil {
		return 0, fmt.Errorf("failed to open zip entry: %w", err)
	}
	defer entryReader.Close()

	guard := &decompressionGuard{
		budgetBytes:     min(remainingBudgetBytes, MaxZipSingleFileBytes),
		isBudgetGlobal:  remainingBudgetBytes < MaxZipSingleFileBytes,
		compressedBytes: entry.CompressedSize64,
		reader:          entryReader,
	}
	written, err := io.Copy(destinationFile, guard)
	if err != nil {
		return written, err
	}
	return written, destinationFile.Close()
}

// decompressionGuard enforces size and ratio limits on the actual decompressed
// stream, because zip headers can lie about uncompressed sizes.
type decompressionGuard struct {
	reader          io.Reader
	budgetBytes     int64
	isBudgetGlobal  bool
	compressedBytes uint64
	readBytes       int64
}

func (guard *decompressionGuard) Read(buffer []byte) (int, error) {
	count, err := guard.reader.Read(buffer)
	guard.readBytes += int64(count)
	if guard.readBytes > guard.budgetBytes && guard.isBudgetGlobal {
		return count, ErrZipTotalSizeLimit
	}
	if guard.readBytes > guard.budgetBytes {
		return count, ErrZipSingleFileSizeLimit
	}
	if exceedsCompressionRatio(uint64(guard.readBytes), guard.compressedBytes) {
		return count, ErrZipCompressionRatio
	}
	if err != nil && !errors.Is(err, io.EOF) {
		return count, fmt.Errorf("error reading zip stream: %w", err)
	}
	return count, err
}

// ExtractZipFile stores an uploaded archive temporarily, validates it and extracts it into a new folder.
func (s *FileStore) ExtractZipFile(targetDirectoryRelativePath, originalFilename string, fileReader io.Reader) (string, error) {
	targetDirectory, err := s.ResolvePath(targetDirectoryRelativePath)
	if err != nil {
		return "", err
	}
	if directoryInfo, err := os.Stat(targetDirectory); err != nil || !directoryInfo.IsDir() {
		return "", ErrNotFound
	}

	spooled, err := s.spoolToTempFile(fileReader)
	if err != nil {
		return "", fmt.Errorf("failed to save uploaded zip: %w", err)
	}
	defer os.Remove(spooled.path)
	if spooled.sizeBytes == 0 {
		return "", ErrZipEmpty
	}
	return s.extractArchiveIntoNewFolder(spooled.path, targetDirectory, originalFilename)
}

// ExtractExistingZip extracts a zip file already stored in SimpleFS next to the archive.
func (s *FileStore) ExtractExistingZip(parentRelativePath, zipFilename string) (string, error) {
	archivePath, err := s.ResolvePath(filepath.Join(parentRelativePath, zipFilename))
	if err != nil {
		return "", err
	}
	if archiveInfo, err := os.Stat(archivePath); err != nil || archiveInfo.IsDir() {
		return "", ErrNotFound
	}
	return s.extractArchiveIntoNewFolder(archivePath, filepath.Dir(archivePath), zipFilename)
}

func (s *FileStore) extractArchiveIntoNewFolder(archivePath, targetDirectory, archiveFilename string) (string, error) {
	archive, err := zip.OpenReader(archivePath)
	if err != nil {
		return "", ErrNotAZipArchive
	}
	defer archive.Close()

	if err := ValidateZipReader(&archive.Reader); err != nil {
		return "", err
	}

	folderName := uniqueFolderName(targetDirectory, s.folderNameForArchive(archiveFilename))
	destinationFolder := filepath.Join(targetDirectory, folderName)
	if err := os.MkdirAll(destinationFolder, s.options.DirMode); err != nil {
		return "", fmt.Errorf("failed to create destination folder: %w", err)
	}
	if err := extractZipReader(&archive.Reader, destinationFolder, s.options); err != nil {
		return "", err
	}
	return folderName, nil
}

func (s *FileStore) folderNameForArchive(archiveFilename string) string {
	baseName := filepath.Base(archiveFilename)
	folderName, err := s.validateItemName(strings.TrimSuffix(baseName, filepath.Ext(baseName)))
	if err != nil {
		return fallbackExtractedFolderName
	}
	return folderName
}
