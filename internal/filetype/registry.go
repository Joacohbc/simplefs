package filetype

import (
	"mime"
	"path/filepath"
	"strings"
)

type Category string

const (
	CategoryDirectory Category = "directory"
	CategoryImage     Category = "image"
	CategoryVideo     Category = "video"
	CategoryAudio     Category = "audio"
	CategoryPDF       Category = "pdf"
	CategoryMarkdown  Category = "markdown"
	CategoryCode      Category = "code"
	CategoryBinary    Category = "binary"
)

type Accent string

const (
	AccentPrimary Accent = "primary"
	AccentNeutral Accent = "neutral"
	AccentCode    Accent = "code"
	AccentMedia   Accent = "media"
	AccentAudio   Accent = "audio"
	AccentArchive Accent = "archive"
	AccentSheet   Accent = "sheet"
	AccentPDF     Accent = "pdf"
)

type AccentClasses struct {
	Text string
	Tile string
	Dot  string
}

// accentClasses holds literal class strings so the Tailwind scanner (which reads Go files) keeps them.
var accentClasses = map[Accent]AccentClasses{
	AccentPrimary: {Text: "text-primary", Tile: "bg-primary/10 text-primary", Dot: "bg-primary"},
	AccentNeutral: {Text: "text-on-surface-variant", Tile: "bg-surface-container-high text-on-surface-variant", Dot: "bg-on-surface-variant"},
	AccentCode:    {Text: "text-accent-code", Tile: "bg-accent-code/15 text-accent-code", Dot: "bg-accent-code"},
	AccentMedia:   {Text: "text-accent-media", Tile: "bg-accent-media/15 text-accent-media", Dot: "bg-accent-media"},
	AccentAudio:   {Text: "text-accent-audio", Tile: "bg-accent-audio/15 text-accent-audio", Dot: "bg-accent-audio"},
	AccentArchive: {Text: "text-accent-archive", Tile: "bg-accent-archive/15 text-accent-archive", Dot: "bg-accent-archive"},
	AccentSheet:   {Text: "text-accent-sheet", Tile: "bg-accent-sheet/15 text-accent-sheet", Dot: "bg-accent-sheet"},
	AccentPDF:     {Text: "text-error", Tile: "bg-error/10 text-error", Dot: "bg-error"},
}

func ClassesFor(accent Accent) AccentClasses {
	if classes, ok := accentClasses[accent]; ok {
		return classes
	}
	return accentClasses[AccentNeutral]
}

// AccentFor derives the visual accent of a definition from its category and icon.
func AccentFor(definition Definition) Accent {
	if definition.Icon == "table_chart" {
		return AccentSheet
	}
	switch definition.Category {
	case CategoryDirectory:
		return AccentPrimary
	case CategoryImage, CategoryVideo:
		return AccentMedia
	case CategoryAudio:
		return AccentAudio
	case CategoryPDF:
		return AccentPDF
	case CategoryCode:
		if definition.LanguageClass == "language-plaintext" || definition.Label == "Environment Config" {
			return AccentNeutral
		}
		return AccentCode
	case CategoryMarkdown:
		return AccentNeutral
	}
	switch definition.Icon {
	case "folder_zip":
		return AccentArchive
	case "article":
		return AccentPrimary
	}
	return AccentNeutral
}

type Definition struct {
	Category      Category
	Label         string
	Icon          string
	LanguageClass string
	MimeType      string
}

var directoryDefinition = Definition{
	Category: CategoryDirectory,
	Label:    "Folder",
	Icon:     "folder",
}

var defaultFileDefinition = Definition{
	Category:      CategoryBinary,
	Label:         "File",
	Icon:          "description",
	LanguageClass: "language-plaintext",
	MimeType:      "application/octet-stream",
}

var registry = map[string]Definition{
	".png": {
		Category: CategoryImage,
		Label:    "PNG Image",
		Icon:     "image",
		MimeType: "image/png",
	},
	".jpg": {
		Category: CategoryImage,
		Label:    "JPEG Image",
		Icon:     "image",
		MimeType: "image/jpeg",
	},
	".jpeg": {
		Category: CategoryImage,
		Label:    "JPEG Image",
		Icon:     "image",
		MimeType: "image/jpeg",
	},
	".webp": {
		Category: CategoryImage,
		Label:    "WEBP Image",
		Icon:     "image",
		MimeType: "image/webp",
	},
	".gif": {
		Category: CategoryImage,
		Label:    "GIF Image",
		Icon:     "image",
		MimeType: "image/gif",
	},
	".svg": {
		Category: CategoryImage,
		Label:    "SVG Vector",
		Icon:     "image",
		MimeType: "image/svg+xml",
	},
	".bmp": {
		Category: CategoryImage,
		Label:    "BMP Image",
		Icon:     "image",
		MimeType: "image/bmp",
	},
	".ico": {
		Category: CategoryImage,
		Label:    "ICO Icon",
		Icon:     "image",
		MimeType: "image/x-icon",
	},
	".pdf": {
		Category: CategoryPDF,
		Label:    "PDF Document",
		Icon:     "picture_as_pdf",
		MimeType: "application/pdf",
	},
	".xlsx": {
		Category: CategoryBinary,
		Label:    "Spreadsheet",
		Icon:     "table_chart",
		MimeType: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
	},
	".xls": {
		Category: CategoryBinary,
		Label:    "Spreadsheet",
		Icon:     "table_chart",
		MimeType: "application/vnd.ms-excel",
	},
	".csv": {
		Category:      CategoryCode,
		Label:         "CSV Document",
		Icon:          "table_chart",
		LanguageClass: "language-plaintext",
		MimeType:      "text/csv",
	},
	".doc": {
		Category: CategoryBinary,
		Label:    "Word Document",
		Icon:     "article",
		MimeType: "application/msword",
	},
	".docx": {
		Category: CategoryBinary,
		Label:    "Word Document",
		Icon:     "article",
		MimeType: "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
	},
	".odt": {
		Category: CategoryBinary,
		Label:    "OpenDocument Text",
		Icon:     "article",
		MimeType: "application/vnd.oasis.opendocument.text",
	},
	".mp4": {
		Category: CategoryVideo,
		Label:    "MP4 Video",
		Icon:     "movie",
		MimeType: "video/mp4",
	},
	".mov": {
		Category: CategoryVideo,
		Label:    "QuickTime Video",
		Icon:     "movie",
		MimeType: "video/quicktime",
	},
	".webm": {
		Category: CategoryVideo,
		Label:    "WebM Video",
		Icon:     "movie",
		MimeType: "video/webm",
	},
	".mkv": {
		Category: CategoryVideo,
		Label:    "Matroska Video",
		Icon:     "movie",
		MimeType: "video/x-matroska",
	},
	".mp3": {
		Category: CategoryAudio,
		Label:    "MP3 Audio",
		Icon:     "music_note",
		MimeType: "audio/mpeg",
	},
	".wav": {
		Category: CategoryAudio,
		Label:    "WAV Audio",
		Icon:     "music_note",
		MimeType: "audio/wav",
	},
	".ogg": {
		Category: CategoryAudio,
		Label:    "OGG Audio",
		Icon:     "music_note",
		MimeType: "audio/ogg",
	},
	".flac": {
		Category: CategoryAudio,
		Label:    "FLAC Audio",
		Icon:     "music_note",
		MimeType: "audio/flac",
	},
	".m4a": {
		Category: CategoryAudio,
		Label:    "M4A Audio",
		Icon:     "music_note",
		MimeType: "audio/mp4",
	},
	".zip": {
		Category: CategoryBinary,
		Label:    "ZIP Archive",
		Icon:     "folder_zip",
		MimeType: "application/zip",
	},
	".tar": {
		Category: CategoryBinary,
		Label:    "TAR Archive",
		Icon:     "folder_zip",
		MimeType: "application/x-tar",
	},
	".gz": {
		Category: CategoryBinary,
		Label:    "GZ Archive",
		Icon:     "folder_zip",
		MimeType: "application/gzip",
	},
	".rar": {
		Category: CategoryBinary,
		Label:    "RAR Archive",
		Icon:     "folder_zip",
		MimeType: "application/vnd.rar",
	},
	".7z": {
		Category: CategoryBinary,
		Label:    "7-Zip Archive",
		Icon:     "folder_zip",
		MimeType: "application/x-7z-compressed",
	},
	".go": {
		Category:      CategoryCode,
		Label:         "Go Source Code",
		Icon:          "code",
		LanguageClass: "language-go",
		MimeType:      "text/x-go",
	},
	".js": {
		Category:      CategoryCode,
		Label:         "JavaScript Source",
		Icon:          "code",
		LanguageClass: "language-javascript",
		MimeType:      "application/javascript",
	},
	".jsx": {
		Category:      CategoryCode,
		Label:         "React JSX",
		Icon:          "code",
		LanguageClass: "language-javascript",
		MimeType:      "text/jsx",
	},
	".ts": {
		Category:      CategoryCode,
		Label:         "TypeScript Source",
		Icon:          "code",
		LanguageClass: "language-typescript",
		MimeType:      "application/typescript",
	},
	".tsx": {
		Category:      CategoryCode,
		Label:         "React TSX",
		Icon:          "code",
		LanguageClass: "language-typescript",
		MimeType:      "text/tsx",
	},
	".py": {
		Category:      CategoryCode,
		Label:         "Python Script",
		Icon:          "code",
		LanguageClass: "language-python",
		MimeType:      "text/x-python",
	},
	".rs": {
		Category:      CategoryCode,
		Label:         "Rust Source",
		Icon:          "code",
		LanguageClass: "language-rust",
		MimeType:      "text/rust",
	},
	".html": {
		Category:      CategoryCode,
		Label:         "HTML Document",
		Icon:          "html",
		LanguageClass: "language-html",
		MimeType:      "text/html",
	},
	".htm": {
		Category:      CategoryCode,
		Label:         "HTML Document",
		Icon:          "html",
		LanguageClass: "language-html",
		MimeType:      "text/html",
	},
	".css": {
		Category:      CategoryCode,
		Label:         "CSS Stylesheet",
		Icon:          "css",
		LanguageClass: "language-css",
		MimeType:      "text/css",
	},
	".json": {
		Category:      CategoryCode,
		Label:         "JSON Document",
		Icon:          "data_object",
		LanguageClass: "language-json",
		MimeType:      "application/json",
	},
	".md": {
		Category:      CategoryMarkdown,
		Label:         "Markdown Document",
		Icon:          "description",
		LanguageClass: "language-markdown",
		MimeType:      "text/markdown",
	},
	".markdown": {
		Category:      CategoryMarkdown,
		Label:         "Markdown Document",
		Icon:          "description",
		LanguageClass: "language-markdown",
		MimeType:      "text/markdown",
	},
	".sh": {
		Category:      CategoryCode,
		Label:         "Shell Script",
		Icon:          "terminal",
		LanguageClass: "language-bash",
		MimeType:      "application/x-sh",
	},
	".sql": {
		Category:      CategoryCode,
		Label:         "SQL Query",
		Icon:          "database",
		LanguageClass: "language-sql",
		MimeType:      "application/sql",
	},
	".yml": {
		Category:      CategoryCode,
		Label:         "YAML Document",
		Icon:          "settings",
		LanguageClass: "language-yaml",
		MimeType:      "application/yaml",
	},
	".yaml": {
		Category:      CategoryCode,
		Label:         "YAML Document",
		Icon:          "settings",
		LanguageClass: "language-yaml",
		MimeType:      "application/yaml",
	},
	".c": {
		Category:      CategoryCode,
		Label:         "C Source Code",
		Icon:          "code",
		LanguageClass: "language-cpp",
		MimeType:      "text/x-c",
	},
	".cpp": {
		Category:      CategoryCode,
		Label:         "C++ Source Code",
		Icon:          "code",
		LanguageClass: "language-cpp",
		MimeType:      "text/x-c++",
	},
	".h": {
		Category:      CategoryCode,
		Label:         "C/C++ Header",
		Icon:          "code",
		LanguageClass: "language-cpp",
		MimeType:      "text/x-c",
	},
	".log": {
		Category:      CategoryCode,
		Label:         "Log File",
		Icon:          "receipt_long",
		LanguageClass: "language-plaintext",
		MimeType:      "text/plain",
	},
	".toml": {
		Category:      CategoryCode,
		Label:         "TOML Document",
		Icon:          "settings",
		LanguageClass: "language-ini",
		MimeType:      "application/toml",
	},
	".ini": {
		Category:      CategoryCode,
		Label:         "INI Config",
		Icon:          "settings",
		LanguageClass: "language-ini",
		MimeType:      "text/plain",
	},
	".xml": {
		Category:      CategoryCode,
		Label:         "XML Document",
		Icon:          "code",
		LanguageClass: "language-xml",
		MimeType:      "application/xml",
	},
	".java": {
		Category:      CategoryCode,
		Label:         "Java Source",
		Icon:          "code",
		LanguageClass: "language-java",
		MimeType:      "text/x-java",
	},
	".rb": {
		Category:      CategoryCode,
		Label:         "Ruby Script",
		Icon:          "code",
		LanguageClass: "language-ruby",
		MimeType:      "text/x-ruby",
	},
	".php": {
		Category:      CategoryCode,
		Label:         "PHP Script",
		Icon:          "code",
		LanguageClass: "language-php",
		MimeType:      "text/x-php",
	},
	".txt": {
		Category:      CategoryCode,
		Label:         "Plain Text",
		Icon:          "description",
		LanguageClass: "language-plaintext",
		MimeType:      "text/plain",
	},
	".env": {
		Category:      CategoryCode,
		Label:         "Environment Config",
		Icon:          "settings",
		LanguageClass: "language-bash",
		MimeType:      "text/plain",
	},
	".gitignore": {
		Category:      CategoryCode,
		Label:         "Git Ignore",
		Icon:          "settings",
		LanguageClass: "language-plaintext",
		MimeType:      "text/plain",
	},
}

func Resolve(filenameOrExt string, isDir bool) Definition {
	if isDir {
		return directoryDefinition
	}

	cleaned := strings.TrimSpace(filenameOrExt)
	if cleaned == "" {
		return defaultFileDefinition
	}

	baseName := filepath.Base(strings.ToLower(cleaned))

	if definition, exists := registry[baseName]; exists {
		return definition
	}

	ext := filepath.Ext(baseName)
	if ext != "" {
		if definition, exists := registry[ext]; exists {
			return definition
		}
	}

	if !strings.HasPrefix(baseName, ".") {
		if definition, exists := registry["."+baseName]; exists {
			return definition
		}
	}

	fallback := defaultFileDefinition
	targetExt := ext
	if targetExt == "" {
		targetExt = "." + baseName
	}

	if detectedMime := mime.TypeByExtension(targetExt); detectedMime != "" {
		fallback.MimeType = detectedMime
	}

	return fallback
}
