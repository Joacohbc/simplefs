package main

import (
	"embed"
	"encoding/base64"
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
	Name             string
	RelPath          string
	IsDir            bool
	Size             int64
	FormattedSize    string
	ModTime          time.Time
	FormattedMod     string
	FormattedCreated string
	ItemCount        int
	TypeLabel        string
	MaterialIcon     string
	IconColorClass   string
	IsImage          bool
}

type Breadcrumb struct {
	Name string
	Path string
}

type PageData struct {
	Path        string
	Query       string
	Breadcrumbs []Breadcrumb
	Folders     []FileInfo
	Files       []FileInfo
	ViewMode    string
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
	MaterialIcon  string
}

type FileDetailsData struct {
	Name          string
	RelPath       string
	DirLocation   string
	TypeLabel     string
	MaterialIcon  string
	FormattedSize string
	CreatedDate   string
	ModifiedDate  string
	IsImage       bool
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
	mux.HandleFunc("GET /api/file-details", handleAPIFileDetails)
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

	server := &http.Server{
		Addr:           addr,
		Handler:        securityHeadersMiddleware(mux),
		ReadTimeout:    15 * time.Second,
		WriteTimeout:   30 * time.Second,
		IdleTimeout:    60 * time.Second,
		MaxHeaderBytes: 1 << 20, // 1 MB
	}

	log.Printf("🚀 SimpleFS running at http://localhost%s serving directory: %s", addr, absStorage)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server stopped: %v", err)
	}
}

// Security Middleware: Injects standard defensive headers
func securityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "SAMEORIGIN")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline' https://unpkg.com https://cdnjs.cloudflare.com https://cdn.jsdelivr.net; style-src 'self' 'unsafe-inline' https://fonts.googleapis.com https://cdnjs.cloudflare.com; font-src 'self' https://fonts.gstatic.com; img-src 'self' data: blob:; media-src 'self' data: blob:; frame-src 'self'; object-src 'none';")
		next.ServeHTTP(w, r)
	})
}

// Security: Resolve relative path safely inside storageDir with symlink boundary check
func resolvePath(rel string) (string, error) {
	absBase, err := filepath.Abs(storageDir)
	if err != nil {
		return "", fmt.Errorf("storage directory error: %w", err)
	}
	if evalBase, err := filepath.EvalSymlinks(absBase); err == nil {
		absBase = evalBase
	}

	cleanRel := filepath.Clean(filepath.FromSlash(rel))
	if cleanRel == "." || cleanRel == "/" || cleanRel == "" {
		cleanRel = ""
	}
	if strings.HasPrefix(cleanRel, "..") {
		return "", fmt.Errorf("invalid path access")
	}

	targetPath := filepath.Join(absBase, cleanRel)

	// Boundary check with filepath.Rel
	relFromBase, err := filepath.Rel(absBase, targetPath)
	if err != nil || strings.HasPrefix(relFromBase, "..") || relFromBase == ".." {
		return "", fmt.Errorf("access denied")
	}

	// If target exists, evaluate symlinks to ensure the real target is within absBase
	if evalTarget, err := filepath.EvalSymlinks(targetPath); err == nil {
		evalRel, err := filepath.Rel(absBase, evalTarget)
		if err != nil || strings.HasPrefix(evalRel, "..") || evalRel == ".." {
			return "", fmt.Errorf("symlink target outside storage root")
		}
		return evalTarget, nil
	}

	return targetPath, nil
}

func getPageData(relPath, query, viewMode string) (PageData, error) {
	absPath, err := resolvePath(relPath)
	if err != nil {
		return PageData{}, err
	}

	entries, err := os.ReadDir(absPath)
	if err != nil {
		return PageData{}, err
	}

	var folders []FileInfo
	var files []FileInfo
	queryLower := strings.ToLower(query)

	for _, entry := range entries {
		name := entry.Name()
		// Filter out internal git and devcontainer build folders
		if name == ".git" || name == ".dc_simplefs" || name == "simplefs" || name == "node_modules" || name == ".stitch" {
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
		ext := strings.ToLower(filepath.Ext(name))
		typeLabel, matIcon, colorClass := getFileInfoMeta(name, entry.IsDir(), ext)

		itemCount := 0
		if entry.IsDir() {
			subEntries, err := os.ReadDir(filepath.Join(absPath, name))
			if err == nil {
				for _, sub := range subEntries {
					subName := sub.Name()
					if !strings.HasPrefix(subName, ".") && subName != "node_modules" {
						itemCount++
					}
				}
			}
		}

		fileObj := FileInfo{
			Name:             name,
			RelPath:          filepath.ToSlash(entryRelPath),
			IsDir:            entry.IsDir(),
			Size:             info.Size(),
			FormattedSize:    formatBytes(info.Size()),
			ModTime:          info.ModTime(),
			FormattedMod:     info.ModTime().Format("02 Jan 2006"),
			FormattedCreated: info.ModTime().Format("02 Jan 2006"),
			ItemCount:        itemCount,
			TypeLabel:        typeLabel,
			MaterialIcon:     matIcon,
			IconColorClass:   colorClass,
			IsImage:          classifyFileType(ext) == "image",
		}

		if entry.IsDir() {
			folders = append(folders, fileObj)
		} else {
			files = append(files, fileObj)
		}
	}

	// Sort folders alphabetically
	sort.Slice(folders, func(i, j int) bool {
		return strings.ToLower(folders[i].Name) < strings.ToLower(folders[j].Name)
	})

	// Sort files alphabetically
	sort.Slice(files, func(i, j int) bool {
		return strings.ToLower(files[i].Name) < strings.ToLower(files[j].Name)
	})

	if viewMode == "" {
		viewMode = "list"
	}

	return PageData{
		Path:        filepath.ToSlash(relPath),
		Query:       query,
		Breadcrumbs: buildBreadcrumbs(relPath),
		Folders:     folders,
		Files:       files,
		ViewMode:    viewMode,
	}, nil
}

func getFileInfoMeta(name string, isDir bool, ext string) (typeLabel, icon, colorClass string) {
	if isDir {
		return "Carpeta", "folder", "text-primary dark:text-primary-fixed-dim"
	}

	switch ext {
	case ".png", ".jpg", ".jpeg", ".webp", ".gif", ".svg", ".bmp", ".ico":
		return strings.ToUpper(strings.TrimPrefix(ext, ".")) + " Image", "image", "text-secondary dark:text-secondary-fixed-dim"
	case ".pdf":
		return "PDF Document", "picture_as_pdf", "text-error dark:text-error-container"
	case ".xlsx", ".xls", ".csv":
		return "Hoja de Cálculo", "table_chart", "text-emerald-600 dark:text-emerald-400"
	case ".doc", ".docx", ".odt":
		return "Documento de Texto", "article", "text-blue-600 dark:text-blue-400"
	case ".mp4", ".mov", ".webm", ".mkv":
		return "Archivo de Video", "movie", "text-purple-600 dark:text-purple-400"
	case ".mp3", ".wav", ".ogg", ".flac", ".m4a":
		return "Archivo de Audio", "audiotrack", "text-amber-600 dark:text-amber-400"
	case ".zip", ".tar", ".gz", ".rar", ".7z":
		return "Archivo Comprimido", "folder_zip", "text-orange-600 dark:text-orange-400"
	case ".go":
		return "Go Source Code", "code", "text-cyan-600 dark:text-cyan-400"
	case ".js", ".ts", ".jsx", ".tsx":
		return "JavaScript / TypeScript", "code", "text-yellow-600 dark:text-yellow-400"
	case ".py":
		return "Python Script", "code", "text-blue-600 dark:text-blue-400"
	case ".html", ".htm":
		return "HTML Document", "html", "text-orange-600 dark:text-orange-400"
	case ".css":
		return "CSS Stylesheet", "css", "text-blue-500 dark:text-blue-300"
	case ".json":
		return "JSON Document", "data_object", "text-teal-600 dark:text-teal-400"
	case ".md", ".markdown":
		return "Markdown Note", "description", "text-secondary dark:text-secondary-fixed-dim"
	default:
		return "Archivo", "description", "text-secondary dark:text-secondary-fixed-dim"
	}
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
	viewMode := r.URL.Query().Get("view")
	data, err := getPageData(relPath, "", viewMode)
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
	viewMode := r.URL.Query().Get("view")

	data, err := getPageData(relPath, query, viewMode)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "file_list.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func handleAPIFileDetails(w http.ResponseWriter, r *http.Request) {
	relPath := r.URL.Query().Get("path")
	absPath, err := resolvePath(relPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	info, err := os.Stat(absPath)
	if err != nil {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	ext := strings.ToLower(filepath.Ext(absPath))
	typeLabel, matIcon, _ := getFileInfoMeta(info.Name(), info.IsDir(), ext)

	parentDir := filepath.ToSlash(filepath.Dir(relPath))
	if parentDir == "." || parentDir == "" {
		parentDir = "/"
	} else {
		parentDir = "/" + parentDir
	}

	details := FileDetailsData{
		Name:          filepath.Base(absPath),
		RelPath:       filepath.ToSlash(relPath),
		DirLocation:   parentDir,
		TypeLabel:     typeLabel,
		MaterialIcon:  matIcon,
		FormattedSize: formatBytes(info.Size()),
		CreatedDate:   info.ModTime().Format("02 Jan 2006"),
		ModifiedDate:  info.ModTime().Format("02 Jan 2006, 15:04"),
		IsImage:       classifyFileType(ext) == "image",
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "details_modal.html", details); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func handleAPIUpload(w http.ResponseWriter, r *http.Request) {
	// Defend against DoS with max 100 MB request limit
	r.Body = http.MaxBytesReader(w, r.Body, 100<<20)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, "Upload payload too large or invalid form", http.StatusBadRequest)
		return
	}

	relPath := r.FormValue("path")
	viewMode := r.FormValue("view")
	targetDir, err := resolvePath(relPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	files := r.MultipartForm.File["files"]
	for _, fileHeader := range files {
		// Clean and secure uploaded filename
		cleanFilename := filepath.Base(filepath.Clean(fileHeader.Filename))
		if cleanFilename == "" || cleanFilename == "." || cleanFilename == ".." {
			continue
		}

		file, err := fileHeader.Open()
		if err != nil {
			continue
		}

		dstPath := filepath.Join(targetDir, cleanFilename)
		dst, err := os.Create(dstPath)
		if err != nil {
			file.Close()
			continue
		}

		io.Copy(dst, file)
		file.Close()
		dst.Close()
	}

	data, err := getPageData(relPath, "", viewMode)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	tmpl.ExecuteTemplate(w, "file_list.html", data)
}

func handleAPIFolder(w http.ResponseWriter, r *http.Request) {
	relPath := r.FormValue("path")
	viewMode := r.FormValue("view")
	folderName := strings.TrimSpace(r.FormValue("name"))

	// Strict filename sanitization
	if folderName == "" || strings.Contains(folderName, "/") || strings.Contains(folderName, "\\") || strings.Contains(folderName, "..") {
		http.Error(w, "Invalid folder name", http.StatusBadRequest)
		return
	}

	cleanFolderName := filepath.Base(filepath.Clean(folderName))
	if cleanFolderName == "" || cleanFolderName == "." || cleanFolderName == ".." {
		http.Error(w, "Invalid folder name", http.StatusBadRequest)
		return
	}

	targetDir, err := resolvePath(relPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	newFolderDir := filepath.Join(targetDir, cleanFolderName)
	if err := os.MkdirAll(newFolderDir, 0755); err != nil {
		http.Error(w, "Failed to create folder", http.StatusInternalServerError)
		return
	}

	data, err := getPageData(relPath, "", viewMode)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	tmpl.ExecuteTemplate(w, "file_list.html", data)
}

func handleAPICreateFile(w http.ResponseWriter, r *http.Request) {
	relPath := r.FormValue("path")
	viewMode := r.FormValue("view")
	filename := strings.TrimSpace(r.FormValue("filename"))
	content := r.FormValue("content")
	isBase64 := r.FormValue("is_base64") == "true"

	// Strict filename sanitization
	if filename == "" || strings.Contains(filename, "/") || strings.Contains(filename, "\\") || strings.Contains(filename, "..") {
		http.Error(w, "Invalid filename", http.StatusBadRequest)
		return
	}

	cleanFilename := filepath.Base(filepath.Clean(filename))
	if cleanFilename == "" || cleanFilename == "." || cleanFilename == ".." {
		http.Error(w, "Invalid filename", http.StatusBadRequest)
		return
	}

	targetFile, err := resolvePath(filepath.Join(relPath, cleanFilename))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var fileBytes []byte
	if isBase64 {
		// Strip DataURL prefix if present e.g. "data:image/png;base64,"
		if idx := strings.Index(content, ","); idx != -1 {
			content = content[idx+1:]
		}
		var err error
		fileBytes, err = base64.StdEncoding.DecodeString(content)
		if err != nil {
			http.Error(w, "Invalid base64 data", http.StatusBadRequest)
			return
		}
	} else {
		fileBytes = []byte(content)
	}

	if err := os.WriteFile(targetFile, fileBytes, 0644); err != nil {
		http.Error(w, "Failed to save file", http.StatusInternalServerError)
		return
	}

	data, err := getPageData(relPath, "", viewMode)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	tmpl.ExecuteTemplate(w, "file_list.html", data)
}

func handleAPIDelete(w http.ResponseWriter, r *http.Request) {
	relPath := strings.TrimSpace(r.URL.Query().Get("path"))
	viewMode := r.URL.Query().Get("view")

	cleanRel := filepath.Clean(filepath.FromSlash(relPath))
	if relPath == "" || cleanRel == "." || cleanRel == "/" || cleanRel == "" {
		http.Error(w, "Cannot delete root directory", http.StatusForbidden)
		return
	}

	absPath, err := resolvePath(relPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Double-check target is not the base storage directory
	absBase, _ := filepath.Abs(storageDir)
	if absBaseEval, err := filepath.EvalSymlinks(absBase); err == nil {
		absBase = absBaseEval
	}
	if absPath == absBase {
		http.Error(w, "Cannot delete root directory", http.StatusForbidden)
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

	data, err := getPageData(parentRel, "", viewMode)
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

	_, matIcon, _ := getFileInfoMeta(info.Name(), false, ext)

	preview := PreviewData{
		Name:          filepath.Base(absPath),
		RelPath:       filepath.ToSlash(relPath),
		Type:          fileType,
		Extension:     ext,
		MimeType:      mimeType,
		FormattedSize: formatBytes(info.Size()),
		ModTime:       info.ModTime().Format("02 Jan 2006 15:04"),
		LanguageClass: getLanguageClass(ext),
		MaterialIcon:  matIcon,
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

	ext := strings.ToLower(filepath.Ext(absPath))
	filename := filepath.Base(absPath)
	mimeType := mime.TypeByExtension(ext)
	if mimeType != "" {
		w.Header().Set("Content-Type", mimeType)
	}

	// Prevent browser execution of potentially active types (HTML, SVG, XML) by forcing attachment
	if r.URL.Query().Get("download") == "true" || ext == ".html" || ext == ".htm" || ext == ".svg" || ext == ".xml" {
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
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
	case ".txt", ".go", ".js", ".ts", ".jsx", ".tsx", ".css", ".html", ".json", ".xml", ".sh", ".py", ".yml", ".yaml", ".c", ".cpp", ".h", ".rs", ".sql", ".env", ".gitignore", ".mod", ".sum", ".toml":
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
