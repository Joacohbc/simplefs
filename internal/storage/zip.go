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
	MaxZipTotalUncompressedBytes = 500 * 1024 * 1024 // 500 MB
	MaxZipSingleFileBytes        = 200 * 1024 * 1024 // 200 MB
	MaxZipCompressionRatio       = 100               // 100:1 ratio limit for files > 1 MB
	MaxZipPathDepth              = 30
)

var (
	ErrZipEmpty                 = errors.New("zip archive is empty")
	ErrZipTooManyFiles          = errors.New("zip archive exceeds maximum file count limit (10,000 files)")
	ErrZipTotalSizeLimit        = errors.New("zip archive uncompressed size exceeds maximum limit (500 MB)")
	ErrZipSingleFileSizeLimit   = errors.New("zip entry uncompressed size exceeds limit (200 MB)")
	ErrZipCompressionRatio      = errors.New("zip entry exceeds compression ratio threshold (potential zip bomb)")
	ErrZipPathTraversal         = errors.New("illegal file path in zip entry (Zip Slip attempt detected)")
	ErrZipSymlinkForbidden      = errors.New("zip entry contains symlink or special device (forbidden for security)")
	ErrZipRestrictedSegment     = errors.New("zip entry contains protected or restricted folder name")
	ErrZipInvalidName           = errors.New("zip entry has invalid or dangerous characters in filename")
	ErrNotAZipArchive           = errors.New("uploaded file is not a valid zip archive")
	ErrNotADirectory            = errors.New("target path is not a directory")
)

// ValidateZipEntryHeader inspects a single zip file header for security threats before extraction.
func ValidateZipEntryHeader(f *zip.File) error {
	rawName := f.Name

	// 1. Check for null bytes or control characters in filename
	if strings.ContainsRune(rawName, 0) {
		return ErrZipInvalidName
	}
	for _, r := range rawName {
		if unicode.IsControl(r) && r != '\t' {
			return ErrZipInvalidName
		}
	}

	// 2. Reject absolute paths or Windows drive letters
	normalized := strings.ReplaceAll(rawName, "\\", "/")
	if strings.HasPrefix(normalized, "/") || strings.HasPrefix(normalized, "\\") {
		return ErrZipPathTraversal
	}
	if len(normalized) >= 2 && normalized[1] == ':' {
		return ErrZipPathTraversal
	}

	// 3. Reject any occurrence of ".." in the raw or normalized path
	if strings.Contains(normalized, "..") {
		return ErrZipPathTraversal
	}

	cleanRel := filepath.Clean(filepath.FromSlash(normalized))
	if cleanRel == "." || cleanRel == "" {
		// Clean root/current entry in archive is acceptable, but skip
		return nil
	}
	if strings.HasPrefix(cleanRel, "..") || filepath.IsAbs(cleanRel) {
		return ErrZipPathTraversal
	}

	// 4. Check folder nesting depth to prevent directory stack overflow attacks
	segments := strings.Split(cleanRel, string(filepath.Separator))
	if len(segments) > MaxZipPathDepth {
		return ErrZipPathTraversal
	}

	// 5. Check restricted segments (e.g. .git, .env, node_modules, simplefs, etc.)
	for _, segment := range segments {
		if isRestrictedSegment(segment) {
			return ErrZipRestrictedSegment
		}
	}

	// 6. Check file mode: reject symlinks and special files (pipes, devices, sockets)
	mode := f.Mode()
	if mode&os.ModeSymlink != 0 {
		return ErrZipSymlinkForbidden
	}
	if mode&(os.ModeDevice|os.ModeNamedPipe|os.ModeSocket|os.ModeCharDevice|os.ModeIrregular) != 0 {
		return ErrZipSymlinkForbidden
	}

	// 7. Check declared uncompressed size limits
	if f.UncompressedSize64 > MaxZipSingleFileBytes {
		return ErrZipSingleFileSizeLimit
	}

	// 8. Check declared compression ratio if file is larger than 1MB
	if f.CompressedSize64 > 0 && f.UncompressedSize64 > 1024*1024 {
		if f.UncompressedSize64/f.CompressedSize64 > MaxZipCompressionRatio {
			return ErrZipCompressionRatio
		}
	}

	return nil
}

// ValidateZipReader validates all entries in a zip archive prior to extracting anything to disk.
func ValidateZipReader(zr *zip.Reader) error {
	if len(zr.File) == 0 {
		return ErrZipEmpty
	}
	if len(zr.File) > MaxZipFileCount {
		return ErrZipTooManyFiles
	}

	var totalDeclaredUncompressed uint64
	for _, f := range zr.File {
		if err := ValidateZipEntryHeader(f); err != nil {
			return err
		}
		totalDeclaredUncompressed += f.UncompressedSize64
		if totalDeclaredUncompressed > MaxZipTotalUncompressedBytes {
			return ErrZipTotalSizeLimit
		}
	}

	return nil
}

// ExtractZipReader safely extracts all entries from a zip.Reader into targetFolderPath.
// If any error occurs or limits are exceeded during decompression, all extracted files are removed.
func ExtractZipReader(zr *zip.Reader, targetFolderPath string) error {
	var totalExtractedBytes int64
	var extractionSuccessful bool

	defer func() {
		if !extractionSuccessful {
			_ = os.RemoveAll(targetFolderPath)
		}
	}()

	for _, f := range zr.File {
		normalized := strings.ReplaceAll(f.Name, "\\", "/")
		cleanRel := filepath.Clean(filepath.FromSlash(normalized))
		if cleanRel == "." || cleanRel == "" {
			continue
		}

		destPath := filepath.Join(targetFolderPath, cleanRel)

		// Strictly verify that destPath is inside targetFolderPath
		rel, err := filepath.Rel(targetFolderPath, destPath)
		if err != nil || strings.HasPrefix(rel, "..") || rel == ".." {
			return ErrZipPathTraversal
		}

		isDir := f.Mode().IsDir() || strings.HasSuffix(f.Name, "/") || strings.HasSuffix(f.Name, "\\")

		if isDir {
			if err := os.MkdirAll(destPath, 0755); err != nil {
				return fmt.Errorf("failed to create directory in zip: %w", err)
			}
			continue
		}

		// Ensure parent directory exists
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return fmt.Errorf("failed to create parent directory in zip: %w", err)
		}

		if err := ensureNotSymlink(destPath); err != nil {
			return err
		}

		destFile, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
		if err != nil {
			return fmt.Errorf("failed to create extracted file: %w", err)
		}

		rc, err := f.Open()
		if err != nil {
			destFile.Close()
			return fmt.Errorf("failed to open zip entry: %w", err)
		}

		// Use LimitReader to prevent decompression bomb that lies in header size
		remainingGlobalBudget := MaxZipTotalUncompressedBytes - totalExtractedBytes
		if remainingGlobalBudget <= 0 {
			rc.Close()
			destFile.Close()
			return ErrZipTotalSizeLimit
		}

		limit := remainingGlobalBudget + 1
		if limit > MaxZipSingleFileBytes+1 {
			limit = MaxZipSingleFileBytes + 1
		}
		limitedReader := io.LimitReader(rc, limit)

		var singleFileBytes int64
		buf := make([]byte, 32*1024)
		var readErr error

		for {
			n, rErr := limitedReader.Read(buf)
			if n > 0 {
				singleFileBytes += int64(n)
				totalExtractedBytes += int64(n)

				if singleFileBytes > MaxZipSingleFileBytes {
					rc.Close()
					destFile.Close()
					return ErrZipSingleFileSizeLimit
				}

				if totalExtractedBytes > MaxZipTotalUncompressedBytes {
					rc.Close()
					destFile.Close()
					return ErrZipTotalSizeLimit
				}

				if f.CompressedSize64 > 0 && singleFileBytes > 1024*1024 {
					if singleFileBytes/int64(f.CompressedSize64) > MaxZipCompressionRatio {
						rc.Close()
						destFile.Close()
						return ErrZipCompressionRatio
					}
				}

				if _, wErr := destFile.Write(buf[:n]); wErr != nil {
					rc.Close()
					destFile.Close()
					return fmt.Errorf("failed writing extracted file: %w", wErr)
				}
			}

			if rErr != nil {
				if rErr == io.EOF {
					break
				}
				readErr = rErr
				break
			}
		}

		rc.Close()
		destFile.Close()

		if readErr != nil {
			return fmt.Errorf("error reading zip stream: %w", readErr)
		}
	}

	extractionSuccessful = true
	return nil
}

// findUniqueFolderName generates a non-colliding folder name in parentDir based on baseName.
func findUniqueFolderName(parentDir, baseName string) string {
	candidate := baseName
	counter := 1
	for {
		targetPath := filepath.Join(parentDir, candidate)
		if _, err := os.Stat(targetPath); errors.Is(err, os.ErrNotExist) {
			return candidate
		}
		candidate = fmt.Sprintf("%s_%d", baseName, counter)
		counter++
	}
}

// ExtractZipFile handles an uploaded zip file stream, validates it securely, and extracts it as a new folder.
func (s *Service) ExtractZipFile(targetDirectoryRelativePath, originalFilename string, fileReader io.Reader) (string, error) {
	targetDirectory, err := s.ResolvePath(targetDirectoryRelativePath)
	if err != nil {
		return "", err
	}

	dirInfo, err := os.Stat(targetDirectory)
	if err != nil || !dirInfo.IsDir() {
		return "", ErrNotFound
	}

	// 1. Save uploaded stream to a secure temporary file
	tempFile, err := os.CreateTemp("", "simplefs-upload-*.zip")
	if err != nil {
		return "", fmt.Errorf("failed to create temporary upload file: %w", err)
	}
	tempPath := tempFile.Name()
	defer os.Remove(tempPath)

	writtenSize, err := io.Copy(tempFile, fileReader)
	tempFile.Close()
	if err != nil {
		return "", fmt.Errorf("failed to save uploaded zip: %w", err)
	}

	if writtenSize == 0 {
		return "", ErrZipEmpty
	}

	// 2. Open zip reader from temp file
	zipReader, err := zip.OpenReader(tempPath)
	if err != nil {
		return "", ErrNotAZipArchive
	}
	defer zipReader.Close()

	// 3. Pre-validate zip before touching storage
	if err := ValidateZipReader(&zipReader.Reader); err != nil {
		return "", err
	}

	// 4. Determine destination folder name
	cleanZipName := strings.TrimSuffix(filepath.Base(originalFilename), filepath.Ext(originalFilename))
	safeFolderName, err := validateItemName(cleanZipName)
	if err != nil || safeFolderName == "" {
		safeFolderName = "extracted_archive"
	}

	uniqueFolderName := findUniqueFolderName(targetDirectory, safeFolderName)
	destFolderPath := filepath.Join(targetDirectory, uniqueFolderName)

	if err := os.MkdirAll(destFolderPath, 0755); err != nil {
		return "", fmt.Errorf("failed to create destination folder: %w", err)
	}

	// 5. Extract with dynamic limits and rollback
	if err := ExtractZipReader(&zipReader.Reader, destFolderPath); err != nil {
		return "", err
	}

	return uniqueFolderName, nil
}

// ExtractExistingZip extracts a zip file that is already located within SimpleFS storage.
func (s *Service) ExtractExistingZip(parentRelativePath, zipFilename string) (string, error) {
	zipRelativePath := filepath.Join(parentRelativePath, zipFilename)
	resolvedZipPath, err := s.ResolvePath(zipRelativePath)
	if err != nil {
		return "", err
	}

	fileInfo, err := os.Stat(resolvedZipPath)
	if err != nil || fileInfo.IsDir() {
		return "", ErrNotFound
	}

	zipReader, err := zip.OpenReader(resolvedZipPath)
	if err != nil {
		return "", ErrNotAZipArchive
	}
	defer zipReader.Close()

	if err := ValidateZipReader(&zipReader.Reader); err != nil {
		return "", err
	}

	targetDirectory := filepath.Dir(resolvedZipPath)
	cleanZipName := strings.TrimSuffix(filepath.Base(zipFilename), filepath.Ext(zipFilename))
	safeFolderName, err := validateItemName(cleanZipName)
	if err != nil || safeFolderName == "" {
		safeFolderName = "extracted_archive"
	}

	uniqueFolderName := findUniqueFolderName(targetDirectory, safeFolderName)
	destFolderPath := filepath.Join(targetDirectory, uniqueFolderName)

	if err := os.MkdirAll(destFolderPath, 0755); err != nil {
		return "", fmt.Errorf("failed to create destination folder: %w", err)
	}

	if err := ExtractZipReader(&zipReader.Reader, destFolderPath); err != nil {
		return "", err
	}

	return uniqueFolderName, nil
}

// GetFolderDownloadInfo validates that the relativePath is a valid folder and returns the archive filename.
func (s *Service) GetFolderDownloadInfo(relativePath string) (string, string, error) {
	absolutePath, err := s.ResolvePath(relativePath)
	if err != nil {
		return "", "", err
	}

	stat, err := os.Stat(absolutePath)
	if err != nil {
		return "", "", ErrNotFound
	}
	if !stat.IsDir() {
		return "", "", ErrNotADirectory
	}

	var archiveFilename string
	cleanRel := filepath.Clean(filepath.FromSlash(strings.TrimSpace(relativePath)))
	if cleanRel == "." || cleanRel == "" || cleanRel == "/" {
		archiveFilename = "simplefs-files.zip"
	} else {
		base := filepath.Base(absolutePath)
		archiveFilename = fmt.Sprintf("%s.zip", base)
	}

	return absolutePath, archiveFilename, nil
}

// StreamFolderZip recursively compresses the folder at folderAbsPath into a zip archive and streams it to w.
func (s *Service) StreamFolderZip(folderAbsPath string, w io.Writer) error {
	absoluteBase, err := filepath.Abs(s.baseDirectory)
	if err != nil {
		return err
	}
	if evaluatedBase, err := filepath.EvalSymlinks(absoluteBase); err == nil {
		absoluteBase = evaluatedBase
	}

	zipWriter := zip.NewWriter(w)
	defer zipWriter.Close()

	return filepath.WalkDir(folderAbsPath, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == folderAbsPath {
			return nil
		}

		name := d.Name()
		if isRestrictedSegment(name) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		relPath, err := filepath.Rel(folderAbsPath, path)
		if err != nil {
			return err
		}
		zipRelPath := filepath.ToSlash(relPath)

		info, err := d.Info()
		if err != nil {
			return err
		}

		// Security: verify symlinks don't escape storage root
		if info.Mode()&os.ModeSymlink != 0 {
			evaluated, err := filepath.EvalSymlinks(path)
			if err != nil {
				return nil // Skip broken symlinks
			}
			evalRel, err := filepath.Rel(absoluteBase, evaluated)
			if err != nil || strings.HasPrefix(evalRel, "..") {
				return nil // Skip external symlinks
			}
			// Update info to the target file if needed
			if targetInfo, err := os.Stat(evaluated); err == nil {
				info = targetInfo
			}
		}

		if d.IsDir() {
			header := &zip.FileHeader{
				Name:     zipRelPath + "/",
				Method:   zip.Deflate,
				Modified: info.ModTime(),
			}
			_, err = zipWriter.CreateHeader(header)
			return err
		}

		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = zipRelPath
		header.Method = zip.Deflate

		writer, err := zipWriter.CreateHeader(header)
		if err != nil {
			return err
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		_, err = io.Copy(writer, file)
		return err
	})
}
