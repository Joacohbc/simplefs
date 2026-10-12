package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"simplefs/internal/filetype"
	"simplefs/internal/i18n"
	"sort"
	"strings"
	"time"
)

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
func (s *FileStore) List(relativePath string) (Entry, []Entry, error) {
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
func (s *FileStore) Stat(relativePath string, withChecksum bool) (Entry, error) {
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
func (s *FileStore) Search(relativePath, query string, limit int) ([]Entry, bool, error) {
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
func (s *FileStore) Recent(limit int) ([]Entry, bool, error) {
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
