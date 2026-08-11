package main

import (
	"embed"
	"fmt"
	"html/template"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

//go:embed templates/* static/*
var embeddedFS embed.FS

var (
	storageDir = "."
	tmpl       *template.Template
)

type FileInfo struct {
	Name          string
	RelPath       string
	IsDir         bool
	Size          int64
	FormattedSize string
	ModTime       time.Time
	FormattedTime string
}

type Breadcrumb struct {
	Name string
	Path string
}

type PageData struct {
	Path        string
	Breadcrumbs []Breadcrumb
	Files       []FileInfo
}

type PreviewData struct {
	Name          string
	RelPath       string
	Type          string // image, video, audio, pdf, markdown, code, binary
	Extension     string
	MimeType      string
	Content       string
	FormattedSize string
	ModTime       string
	LineCount     int
	LanguageClass string
}

func main() {
	// Configure storage directory from env or default to current project root "."
	if dir := os.Getenv("STORAGE_DIR"); dir != "" {
		storageDir = dir
	}

	// Ensure storage directory exists
	if err := os.MkdirAll(storageDir, 0755); err != nil {
		log.Fatalf("Failed to create storage directory: %v", err)
	}

	// Parse templates
	var err error
	tmpl, err = template.ParseFS(embeddedFS, "templates/*.html")
	if err != nil {
		log.Fatalf("Failed to parse templates: %v", err)
	}

	mux := http.NewServeMux()

	// Static assets handler
	mux.Handle("GET /static/", http.FileServer(http.FS(embeddedFS)))

	// Web Routes
	mux.HandleFunc("GET /", handleIndex)
	mux.HandleFunc("GET /api/files", handleAPIFiles)
	mux.HandleFunc("POST /api/upload", handleAPIUpload)
	mux.HandleFunc("POST /api/folder", handleAPIFolder)
	mux.HandleFunc("POST /api/create-file", handleAPICreateFile)
	mux.HandleFunc("DELETE /api/delete", handleAPIDelete)
	mux.HandleFunc("GET /api/preview", handleAPIPreview)
	mux.HandleFunc("GET /download", handleDownload)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	absStorage, _ := filepath.Abs(storageDir)
	addr := ":" + port
	log.Printf("🚀 SimpleFS running at http://localhost%s serving directory: %s", addr, absStorage)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("Server stopped: %v", err)
	}
}

// Security: Resolve relative path safely inside storageDir
func resolvePath(rel string) (string, error) {
	cleanRel := filepath.Clean(filepath.FromSlash(rel))
	if cleanRel == "." || cleanRel == "/" {
		cleanRel = ""
	}
	if strings.HasPrefix(cleanRel, "..") {
		return "", fmt.Errorf("invalid path access")
	}

	absBase, err := filepath.Abs(storageDir)
	if err != nil {
		return "", err
	}

	targetPath := filepath.Join(absBase, cleanRel)
	if !strings.HasPrefix(targetPath, absBase) {
		return "", fmt.Errorf("access denied")
	}

	return targetPath, nil
}

func getPageData(relPath, query string) (PageData, error) {
	absPath, err := resolvePath(relPath)
	if err != nil {
		return PageData{}, err
	}

	entries, err := os.ReadDir(absPath)
	if err != nil {
		return PageData{}, err
	}

	var files []FileInfo
	queryLower := strings.ToLower(query)

	for _, entry := range entries {
		name := entry.Name()
		// Filter out internal git and devcontainer build folders
		if name == ".git" || name == ".dc_simplefs" || name == "simplefs" {
			continue
		}

		if queryLower != "" && !strings.Contains(strings.ToLower(name), queryLower) {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		entryRelPath := filepath.Join(relPath, name)
		files = append(files, FileInfo{
			Name:          name,
			RelPath:       filepath.ToSlash(entryRelPath),
			IsDir:         entry.IsDir(),
			Size:          info.Size(),
			FormattedSize: formatBytes(info.Size()),
			ModTime:       info.ModTime(),
			FormattedTime: info.ModTime().Format("02/01/2006 15:04"),
		})
	}

	// Sort folders first, then files alphabetically
	sort.Slice(files, func(i, j int) bool {
		if files[i].IsDir != files[j].IsDir {
			return files[i].IsDir
		}
		return strings.ToLower(files[i].Name) < strings.ToLower(files[j].Name)
	})

	return PageData{
		Path:        filepath.ToSlash(relPath),
		Breadcrumbs: buildBreadcrumbs(relPath),
		Files:       files,
	}, nil
}

func buildBreadcrumbs(relPath string) []Breadcrumb {
	if relPath == "" || relPath == "." {
		return nil
	}

	parts := strings.Split(filepath.ToSlash(relPath), "/")
	var crumbs []Breadcrumb
	var current string

	for _, part := range parts {
		if part == "" {
			continue
		}
		if current == "" {
			current = part
		} else {
			current = current + "/" + part
		}
		crumbs = append(crumbs, Breadcrumb{
			Name: part,
			Path: current,
		})
	}

	return crumbs
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	relPath := r.URL.Query().Get("path")
	data, err := getPageData(relPath, "")
	if err != nil {
		http.Error(w, "Directory not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "index.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func handleAPIFiles(w http.ResponseWriter, r *http.Request) {
	relPath := r.URL.Query().Get("path")
	query := r.URL.Query().Get("query")

	data, err := getPageData(relPath, query)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "file_list.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func handleAPIUpload(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(100 << 20); err != nil { // 100 MB max memory limit
		http.Error(w, "Error parsing form", http.StatusBadRequest)
		return
	}

	relPath := r.FormValue("path")
	targetDir, err := resolvePath(relPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	files := r.MultipartForm.File["files"]
	for _, fileHeader := range files {
		file, err := fileHeader.Open()
		if err != nil {
			continue
		}

		dstPath := filepath.Join(targetDir, filepath.Base(fileHeader.Filename))
		dst, err := os.Create(dstPath)
		if err != nil {
			file.Close()
			continue
		}

		io.Copy(dst, file)
		file.Close()
		dst.Close()
	}

	data, err := getPageData(relPath, "")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	tmpl.ExecuteTemplate(w, "file_list.html", data)
}

func handleAPIFolder(w http.ResponseWriter, r *http.Request) {
	relPath := r.FormValue("path")
	folderName := strings.TrimSpace(r.FormValue("name"))

	if folderName == "" || strings.Contains(folderName, "/") || strings.Contains(folderName, "\\") {
		http.Error(w, "Invalid folder name", http.StatusBadRequest)
		return
	}

	targetDir, err := resolvePath(relPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	newFolderDir := filepath.Join(targetDir, folderName)
	if err := os.MkdirAll(newFolderDir, 0755); err != nil {
		http.Error(w, "Failed to create folder", http.StatusInternalServerError)
		return
	}

	data, err := getPageData(relPath, "")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	tmpl.ExecuteTemplate(w, "file_list.html", data)
}

func handleAPICreateFile(w http.ResponseWriter, r *http.Request) {
	relPath := r.FormValue("path")
	filename := strings.TrimSpace(r.FormValue("filename"))
	content := r.FormValue("content")

	if filename == "" || strings.Contains(filename, "/") || strings.Contains(filename, "\\") {
		http.Error(w, "Invalid filename", http.StatusBadRequest)
		return
	}

	targetFile, err := resolvePath(filepath.Join(relPath, filename))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := os.WriteFile(targetFile, []byte(content), 0644); err != nil {
		http.Error(w, "Failed to save file", http.StatusInternalServerError)
		return
	}

	data, err := getPageData(relPath, "")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	tmpl.ExecuteTemplate(w, "file_list.html", data)
}

func handleAPIDelete(w http.ResponseWriter, r *http.Request) {
	relPath := r.URL.Query().Get("path")
	if relPath == "" {
		http.Error(w, "Path parameter required", http.StatusBadRequest)
		return
	}

	absPath, err := resolvePath(relPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := os.RemoveAll(absPath); err != nil {
		http.Error(w, "Failed to delete item", http.StatusInternalServerError)
		return
	}

	parentRel := filepath.ToSlash(filepath.Dir(relPath))
	if parentRel == "." {
		parentRel = ""
	}

	data, err := getPageData(parentRel, "")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	tmpl.ExecuteTemplate(w, "file_list.html", data)
}

func handleAPIPreview(w http.ResponseWriter, r *http.Request) {
	relPath := r.URL.Query().Get("path")
	absPath, err := resolvePath(relPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	info, err := os.Stat(absPath)
	if err != nil || info.IsDir() {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	ext := strings.ToLower(filepath.Ext(absPath))
	fileType := classifyFileType(ext)
	mimeType := mime.TypeByExtension(ext)
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	preview := PreviewData{
		Name:          filepath.Base(absPath),
		RelPath:       filepath.ToSlash(relPath),
		Type:          fileType,
		Extension:     ext,
		MimeType:      mimeType,
		FormattedSize: formatBytes(info.Size()),
		ModTime:       info.ModTime().Format("02/01/2006 15:04"),
		LanguageClass: getLanguageClass(ext),
	}

	if fileType == "code" || fileType == "markdown" {
		if info.Size() > 1000*1024 { // 1 MB limit for inline preview
			preview.Content = "[Archivo demasiado grande para vista previa en texto]"
		} else {
			content, err := os.ReadFile(absPath)
			if err != nil {
				preview.Content = "[Error al leer el archivo]"
			} else {
				preview.Content = string(content)
				preview.LineCount = strings.Count(preview.Content, "\n") + 1
			}
		}
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	tmpl.ExecuteTemplate(w, "preview_modal.html", preview)
}

func handleDownload(w http.ResponseWriter, r *http.Request) {
	relPath := r.URL.Query().Get("path")
	absPath, err := resolvePath(relPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	info, err := os.Stat(absPath)
	if err != nil || info.IsDir() {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	ext := filepath.Ext(absPath)
	mimeType := mime.TypeByExtension(ext)
	if mimeType != "" {
		w.Header().Set("Content-Type", mimeType)
	}

	http.ServeFile(w, r, absPath)
}

func classifyFileType(ext string) string {
	switch ext {
	case ".png", ".jpg", ".jpeg", ".gif", ".svg", ".webp", ".bmp", ".ico":
		return "image"
	case ".mp4", ".webm", ".ogv", ".mov", ".mkv":
		return "video"
	case ".mp3", ".wav", ".ogg", ".flac", ".m4a":
		return "audio"
	case ".pdf":
		return "pdf"
	case ".md", ".markdown":
		return "markdown"
	case ".txt", ".go", ".js", ".ts", ".jsx", ".tsx", ".css", ".html", ".json", ".xml", ".sh", ".py", ".yml", ".yaml", ".c", ".cpp", ".h", ".rs", ".sql", ".env", ".gitignore", ".mod", ".sum":
		return "code"
	default:
		return "binary"
	}
}

func getLanguageClass(ext string) string {
	switch ext {
	case ".go":
		return "language-go"
	case ".js", ".jsx":
		return "language-javascript"
	case ".ts", ".tsx":
		return "language-typescript"
	case ".py":
		return "language-python"
	case ".rs":
		return "language-rust"
	case ".css":
		return "language-css"
	case ".html":
		return "language-html"
	case ".json":
		return "language-json"
	case ".sh":
		return "language-bash"
	case ".sql":
		return "language-sql"
	case ".yml", ".yaml":
		return "language-yaml"
	case ".c", ".cpp", ".h":
		return "language-cpp"
	case ".md", ".markdown":
		return "language-markdown"
	default:
		return "language-plaintext"
	}
}

func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}
