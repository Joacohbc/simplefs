package assets_test

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"simplefs/internal/assets"
)

func newAssets(t *testing.T) *assets.Assets {
	t.Helper()
	fsys := fstest.MapFS{
		"static/style.css":           {Data: []byte(strings.Repeat("body{color:red}", 200))},
		"static/font-ABCDEFGH.woff2": {Data: []byte("binary-font")},
		"static/icons.svg":           {Data: []byte(`<svg><symbol id="home"></symbol></svg>`)},
		"static/.gitkeep":            {Data: []byte{}},
	}
	collection, err := assets.New(fsys, "static")
	if err != nil {
		t.Fatal(err)
	}
	return collection
}

func TestVersionedURLsAreImmutable(t *testing.T) {
	collection := newAssets(t)
	url := collection.URL("style.css")
	if !strings.HasPrefix(url, "/static/style.css?v=") {
		t.Fatalf("unexpected URL %q", url)
	}

	rec := httptest.NewRecorder()
	collection.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, url, nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Header().Get("Cache-Control"), "immutable") {
		t.Errorf("versioned asset should be immutable: %d %q", rec.Code, rec.Header().Get("Cache-Control"))
	}

	rec = httptest.NewRecorder()
	collection.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/static/style.css", nil))
	if rec.Header().Get("Cache-Control") != "no-cache" {
		t.Errorf("unversioned asset should revalidate, got %q", rec.Header().Get("Cache-Control"))
	}

	rec = httptest.NewRecorder()
	collection.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/static/font-ABCDEFGH.woff2", nil))
	if !strings.Contains(rec.Header().Get("Cache-Control"), "immutable") {
		t.Error("content-hashed file names should be immutable")
	}
}

func TestConditionalAndGzip(t *testing.T) {
	collection := newAssets(t)

	first := httptest.NewRecorder()
	collection.Handler().ServeHTTP(first, httptest.NewRequest(http.MethodGet, "/static/style.css", nil))
	etag := first.Header().Get("ETag")
	if etag == "" {
		t.Fatal("expected ETag")
	}

	req := httptest.NewRequest(http.MethodGet, "/static/style.css", nil)
	req.Header.Set("If-None-Match", etag)
	rec := httptest.NewRecorder()
	collection.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusNotModified {
		t.Errorf("expected 304, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/static/style.css", nil)
	req.Header.Set("Accept-Encoding", "br, gzip;q=0.8")
	rec = httptest.NewRecorder()
	collection.Handler().ServeHTTP(rec, req)
	if rec.Header().Get("Content-Encoding") != "gzip" {
		t.Fatalf("expected gzip encoding, got %q", rec.Header().Get("Content-Encoding"))
	}
	reader, err := gzip.NewReader(bytes.NewReader(rec.Body.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	plain, _ := io.ReadAll(reader)
	if string(plain) != strings.Repeat("body{color:red}", 200) {
		t.Error("gzip body does not match original")
	}

	req = httptest.NewRequest(http.MethodGet, "/static/style.css", nil)
	req.Header.Set("Accept-Encoding", "gzip;q=0")
	rec = httptest.NewRecorder()
	collection.Handler().ServeHTTP(rec, req)
	if rec.Header().Get("Content-Encoding") != "" {
		t.Error("gzip;q=0 must disable compression")
	}

	rec = httptest.NewRecorder()
	collection.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/static/.gitkeep", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("hidden files must not be served, got %d", rec.Code)
	}
}

func TestIconMarkup(t *testing.T) {
	collection := newAssets(t)
	markup := string(collection.Icon("home", "text-xl", `"><script>`))
	if !strings.Contains(markup, `class="icon text-xl &#34;&gt;&lt;script&gt;"`) {
		t.Errorf("icon classes must be escaped: %s", markup)
	}
	if !strings.Contains(markup, "icons.svg?v=") || !strings.Contains(markup, "#home") {
		t.Errorf("icon should reference the versioned sprite: %s", markup)
	}
	if !strings.Contains(string(collection.IconFill("home")), "#home-fill") {
		t.Error("filled icon should use the -fill symbol")
	}
}
