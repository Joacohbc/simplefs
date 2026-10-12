package assets

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"html"
	"html/template"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type staticFile struct {
	content     []byte
	gzipped     []byte
	etag        string
	version     string
	contentType string
}

// Bundle serves embedded static files with strong ETags, long-lived caching for
// versioned URLs and precompressed gzip bodies.
type Bundle struct {
	files map[string]*staticFile
}

const (
	etagHexLength    = 20
	versionHexLength = 10
)

var hashedNamePattern = regexp.MustCompile(`-[A-Z0-9]{8}\.[a-z0-9]+$`)

var compressibleTypes = []string{"text/", "application/javascript", "application/json", "image/svg+xml", "application/manifest+json"}

func init() {
	_ = mime.AddExtensionType(".woff2", "font/woff2")
	_ = mime.AddExtensionType(".woff", "font/woff")
	_ = mime.AddExtensionType(".svg", "image/svg+xml")
	_ = mime.AddExtensionType(".js", "text/javascript; charset=utf-8")
	_ = mime.AddExtensionType(".webmanifest", "application/manifest+json")
}

func New(fsys fs.FS, root string) (*Bundle, error) {
	collection := &Bundle{files: make(map[string]*staticFile)}
	err := fs.WalkDir(fsys, root, func(filePath string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			return nil
		}
		content, err := fs.ReadFile(fsys, filePath)
		if err != nil {
			return err
		}
		name := strings.TrimPrefix(filePath, root+"/")
		sum := sha256.Sum256(content)
		digest := hex.EncodeToString(sum[:])
		contentType := mime.TypeByExtension(path.Ext(name))
		if contentType == "" {
			contentType = http.DetectContentType(content)
		}
		file := &staticFile{
			content:     content,
			etag:        `"` + digest[:etagHexLength] + `"`,
			version:     digest[:versionHexLength],
			contentType: contentType,
		}
		if isCompressible(contentType) {
			file.gzipped = compress(content)
		}
		collection.files[name] = file
		return nil
	})
	return collection, err
}

func isCompressible(contentType string) bool {
	for _, prefix := range compressibleTypes {
		if strings.HasPrefix(contentType, prefix) {
			return true
		}
	}
	return false
}

func compress(data []byte) []byte {
	var buffer bytes.Buffer
	writer, _ := gzip.NewWriterLevel(&buffer, gzip.BestCompression)
	_, _ = writer.Write(data)
	_ = writer.Close()
	if buffer.Len() >= len(data) {
		return nil
	}
	return buffer.Bytes()
}

// URL returns the versioned public URL of a static file.
func (a *Bundle) URL(name string) string {
	if file, ok := a.files[name]; ok {
		return "/static/" + name + "?v=" + file.version
	}
	return "/static/" + name
}

func (a *Bundle) Has(name string) bool {
	_, ok := a.files[name]
	return ok
}

func (a *Bundle) Icon(name string, classes ...string) template.HTML {
	return a.svgIcon(name, strings.Join(classes, " "))
}

func (a *Bundle) IconFill(name string, classes ...string) template.HTML {
	return a.svgIcon(name+"-fill", strings.Join(classes, " "))
}

func (a *Bundle) svgIcon(symbol, classes string) template.HTML {
	className := strings.TrimSpace("icon " + classes)
	href := a.URL("icons.svg") + "#" + symbol
	return template.HTML(`<svg class="` + html.EscapeString(className) + `" aria-hidden="true" focusable="false"><use href="` + html.EscapeString(href) + `"></use></svg>`)
}

func (a *Bundle) FuncMap() template.FuncMap {
	return template.FuncMap{
		"asset":    a.URL,
		"icon":     a.Icon,
		"iconFill": a.IconFill,
	}
}

// ServeFile writes one asset, honouring conditional requests and gzip negotiation.
func (a *Bundle) ServeFile(w http.ResponseWriter, r *http.Request, name string) {
	file, ok := a.files[name]
	if !ok {
		http.NotFound(w, r)
		return
	}

	header := w.Header()
	header.Set("Content-Type", file.contentType)
	header.Set("ETag", file.etag)
	header.Set("Vary", "Accept-Encoding")
	if r.URL.Query().Get("v") != "" || hashedNamePattern.MatchString(name) {
		header.Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		header.Set("Cache-Control", "no-cache")
	}

	body := file.content
	if file.gzipped != nil && acceptsGzip(r) {
		header.Set("Content-Encoding", "gzip")
		body = file.gzipped
	}
	http.ServeContent(w, r, name, time.Time{}, bytes.NewReader(body))
}

func acceptsGzip(r *http.Request) bool {
	for _, part := range strings.Split(r.Header.Get("Accept-Encoding"), ",") {
		encoding, params, _ := strings.Cut(strings.TrimSpace(part), ";")
		if !strings.EqualFold(strings.TrimSpace(encoding), "gzip") {
			continue
		}
		quality := strings.TrimPrefix(strings.ReplaceAll(params, " ", ""), "q=")
		if value, err := strconv.ParseFloat(quality, 64); err == nil && value == 0 {
			return false
		}
		return true
	}
	return false
}

func (a *Bundle) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		a.ServeFile(w, r, strings.TrimPrefix(r.URL.Path, "/static/"))
	})
}
