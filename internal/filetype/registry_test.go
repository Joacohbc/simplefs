package filetype_test

import (
	"testing"

	"simplefs/internal/filetype"
)

func TestResolve_Directory(t *testing.T) {
	definition := filetype.Resolve("", true)
	if definition.Category != filetype.CategoryDirectory {
		t.Errorf("expected CategoryDirectory, got %s", definition.Category)
	}
	if definition.Icon != "folder" {
		t.Errorf("expected icon 'folder', got %s", definition.Icon)
	}
	if definition.Label != "Folder" {
		t.Errorf("expected label 'Folder', got %s", definition.Label)
	}
}

func TestResolve_KnownExtensions(t *testing.T) {
	tests := []struct {
		extension        string
		expectedCategory filetype.Category
		expectedIcon     string
		expectedMime     string
	}{
		{".png", filetype.CategoryImage, "image", "image/png"},
		{".PNG", filetype.CategoryImage, "image", "image/png"},
		{".jpg", filetype.CategoryImage, "image", "image/jpeg"},
		{".jpeg", filetype.CategoryImage, "image", "image/jpeg"},
		{".webp", filetype.CategoryImage, "image", "image/webp"},
		{".gif", filetype.CategoryImage, "image", "image/gif"},
		{".svg", filetype.CategoryImage, "image", "image/svg+xml"},
		{".bmp", filetype.CategoryImage, "image", "image/bmp"},
		{".ico", filetype.CategoryImage, "image", "image/x-icon"},
		{".pdf", filetype.CategoryPDF, "picture_as_pdf", "application/pdf"},
		{".xlsx", filetype.CategoryBinary, "table_chart", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"},
		{".xls", filetype.CategoryBinary, "table_chart", "application/vnd.ms-excel"},
		{".csv", filetype.CategoryCode, "table_chart", "text/csv"},
		{".doc", filetype.CategoryBinary, "article", "application/msword"},
		{".docx", filetype.CategoryBinary, "article", "application/vnd.openxmlformats-officedocument.wordprocessingml.document"},
		{".odt", filetype.CategoryBinary, "article", "application/vnd.oasis.opendocument.text"},
		{".mp4", filetype.CategoryVideo, "movie", "video/mp4"},
		{".mov", filetype.CategoryVideo, "movie", "video/quicktime"},
		{".webm", filetype.CategoryVideo, "movie", "video/webm"},
		{".mkv", filetype.CategoryVideo, "movie", "video/x-matroska"},
		{".mp3", filetype.CategoryAudio, "audiotrack", "audio/mpeg"},
		{".wav", filetype.CategoryAudio, "audiotrack", "audio/wav"},
		{".ogg", filetype.CategoryAudio, "audiotrack", "audio/ogg"},
		{".flac", filetype.CategoryAudio, "audiotrack", "audio/flac"},
		{".m4a", filetype.CategoryAudio, "audiotrack", "audio/mp4"},
		{".zip", filetype.CategoryBinary, "folder_zip", "application/zip"},
		{".tar", filetype.CategoryBinary, "folder_zip", "application/x-tar"},
		{".gz", filetype.CategoryBinary, "folder_zip", "application/gzip"},
		{".rar", filetype.CategoryBinary, "folder_zip", "application/vnd.rar"},
		{".7z", filetype.CategoryBinary, "folder_zip", "application/x-7z-compressed"},
		{".go", filetype.CategoryCode, "code", "text/x-go"},
		{".js", filetype.CategoryCode, "code", "application/javascript"},
		{".jsx", filetype.CategoryCode, "code", "text/jsx"},
		{".ts", filetype.CategoryCode, "code", "application/typescript"},
		{".tsx", filetype.CategoryCode, "code", "text/tsx"},
		{".py", filetype.CategoryCode, "code", "text/x-python"},
		{".rs", filetype.CategoryCode, "code", "text/rust"},
		{".html", filetype.CategoryCode, "html", "text/html"},
		{".htm", filetype.CategoryCode, "html", "text/html"},
		{".css", filetype.CategoryCode, "css", "text/css"},
		{".json", filetype.CategoryCode, "data_object", "application/json"},
		{".md", filetype.CategoryMarkdown, "description", "text/markdown"},
		{".markdown", filetype.CategoryMarkdown, "description", "text/markdown"},
		{".sh", filetype.CategoryCode, "terminal", "application/x-sh"},
		{".sql", filetype.CategoryCode, "database", "application/sql"},
		{".yml", filetype.CategoryCode, "settings", "application/yaml"},
		{".yaml", filetype.CategoryCode, "settings", "application/yaml"},
		{".c", filetype.CategoryCode, "code", "text/x-c"},
		{".cpp", filetype.CategoryCode, "code", "text/x-c++"},
		{".h", filetype.CategoryCode, "code", "text/x-c"},
		{".txt", filetype.CategoryCode, "description", "text/plain"},
		{".env", filetype.CategoryCode, "settings", "text/plain"},
		{".gitignore", filetype.CategoryCode, "settings", "text/plain"},
	}

	for _, tt := range tests {
		t.Run(tt.extension, func(t *testing.T) {
			definition := filetype.Resolve(tt.extension, false)
			if definition.Category != tt.expectedCategory {
				t.Errorf("expected category %s, got %s", tt.expectedCategory, definition.Category)
			}
			if definition.Icon != tt.expectedIcon {
				t.Errorf("expected icon %s, got %s", tt.expectedIcon, definition.Icon)
			}
			if tt.expectedMime != "" && definition.MimeType != tt.expectedMime {
				t.Errorf("expected mime %s, got %s", tt.expectedMime, definition.MimeType)
			}
		})
	}
}

func TestResolve_UnknownAndFallbackExtensions(t *testing.T) {
	definition := filetype.Resolve(".completely_unknown_extension_xyz", false)
	if definition.Category != filetype.CategoryBinary {
		t.Errorf("expected CategoryBinary fallback, got %s", definition.Category)
	}
	if definition.Icon != "description" {
		t.Errorf("expected icon 'description', got %s", definition.Icon)
	}
	if definition.Label != "File" {
		t.Errorf("expected label 'File', got %s", definition.Label)
	}

	emptyDef := filetype.Resolve("", false)
	if emptyDef.Category != filetype.CategoryBinary {
		t.Errorf("expected CategoryBinary for empty extension, got %s", emptyDef.Category)
	}
}
