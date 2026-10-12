package storage

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"simplefs/internal/filetype"
	"strings"
)

const rootArchiveFilename = "simplefs-files.zip"

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

func (s *FileStore) GetDownloadFile(relativePath string) (string, string, string, bool, error) {
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

// GetFolderDownloadInfo validates that the relativePath is a valid folder and returns the archive filename.
func (s *FileStore) GetFolderDownloadInfo(relativePath string) (string, string, error) {
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

	if cleanRelativePath(relativePath) == "" {
		return absolutePath, rootArchiveFilename, nil
	}
	return absolutePath, filepath.Base(absolutePath) + ".zip", nil
}

// StreamFolderZip compresses the folder at folderAbsPath into a zip archive streamed to output.
// Hidden entries are skipped and symlinks are only followed to files inside the storage root.
func (s *FileStore) StreamFolderZip(folderAbsPath string, output io.Writer) error {
	absoluteBase, err := s.absoluteBase()
	if err != nil {
		return err
	}
	archive := zip.NewWriter(output)
	defer archive.Close()

	return filepath.WalkDir(folderAbsPath, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == folderAbsPath {
			return nil
		}
		if s.isHidden(entry.Name()) {
			return skipHiddenEntry(entry)
		}
		info, isArchivable, err := archivableInfo(path, entry, absoluteBase)
		if err != nil || !isArchivable {
			return err
		}
		relativePath, err := filepath.Rel(folderAbsPath, path)
		if err != nil {
			return err
		}
		return addToArchive(archive, path, filepath.ToSlash(relativePath), info, entry.IsDir())
	})
}

func archivableInfo(path string, entry os.DirEntry, absoluteBase string) (os.FileInfo, bool, error) {
	info, err := entry.Info()
	if err != nil {
		return nil, false, err
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return info, true, nil
	}
	targetInfo, isInsideRoot := symlinkedFileInsideRoot(path, absoluteBase)
	return targetInfo, isInsideRoot, nil
}

func symlinkedFileInsideRoot(path, absoluteBase string) (os.FileInfo, bool) {
	target, err := filepath.EvalSymlinks(path)
	if err != nil {
		return nil, false
	}
	relativeToRoot, err := filepath.Rel(absoluteBase, target)
	if err != nil || strings.HasPrefix(relativeToRoot, "..") {
		return nil, false
	}
	targetInfo, err := os.Stat(target)
	if err != nil || targetInfo.IsDir() {
		return nil, false
	}
	return targetInfo, true
}

func addToArchive(archive *zip.Writer, path, archivePath string, info os.FileInfo, isDir bool) error {
	if isDir {
		_, err := archive.CreateHeader(&zip.FileHeader{Name: archivePath + "/", Method: zip.Deflate, Modified: info.ModTime()})
		return err
	}
	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}
	header.Name = archivePath
	header.Method = zip.Deflate
	entryWriter, err := archive.CreateHeader(header)
	if err != nil {
		return err
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = io.Copy(entryWriter, file)
	return err
}
