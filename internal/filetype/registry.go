package filetype

import (
	"mime"
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

type Definition struct {
	Category      Category
	Label         string
	Icon          string
	ColorClass    string
	LanguageClass string
	MimeType      string
}

var directoryDefinition = Definition{
	Category:   CategoryDirectory,
	Label:      "Carpeta",
	Icon:       "folder",
	ColorClass: "text-primary dark:text-primary-fixed-dim",
}

var defaultFileDefinition = Definition{
	Category:      CategoryBinary,
	Label:         "Archivo",
	Icon:          "description",
	ColorClass:    "text-secondary dark:text-secondary-fixed-dim",
	LanguageClass: "language-plaintext",
	MimeType:      "application/octet-stream",
}

var registry = map[string]Definition{
	".png": {
		Category:   CategoryImage,
		Label:      "PNG Image",
		Icon:       "image",
		ColorClass: "text-secondary dark:text-secondary-fixed-dim",
		MimeType:   "image/png",
	},
	".jpg": {
		Category:   CategoryImage,
		Label:      "JPEG Image",
		Icon:       "image",
		ColorClass: "text-secondary dark:text-secondary-fixed-dim",
		MimeType:   "image/jpeg",
	},
	".jpeg": {
		Category:   CategoryImage,
		Label:      "JPEG Image",
		Icon:       "image",
		ColorClass: "text-secondary dark:text-secondary-fixed-dim",
		MimeType:   "image/jpeg",
	},
	".webp": {
		Category:   CategoryImage,
		Label:      "WEBP Image",
		Icon:       "image",
		ColorClass: "text-secondary dark:text-secondary-fixed-dim",
		MimeType:   "image/webp",
	},
	".gif": {
		Category:   CategoryImage,
		Label:      "GIF Image",
		Icon:       "image",
		ColorClass: "text-secondary dark:text-secondary-fixed-dim",
		MimeType:   "image/gif",
	},
	".svg": {
		Category:   CategoryImage,
		Label:      "SVG Vector",
		Icon:       "image",
		ColorClass: "text-secondary dark:text-secondary-fixed-dim",
		MimeType:   "image/svg+xml",
	},
	".bmp": {
		Category:   CategoryImage,
		Label:      "BMP Image",
		Icon:       "image",
		ColorClass: "text-secondary dark:text-secondary-fixed-dim",
		MimeType:   "image/bmp",
	},
	".ico": {
		Category:   CategoryImage,
		Label:      "ICO Icon",
		Icon:       "image",
		ColorClass: "text-secondary dark:text-secondary-fixed-dim",
		MimeType:   "image/x-icon",
	},
	".pdf": {
		Category:   CategoryPDF,
		Label:      "PDF Document",
		Icon:       "picture_as_pdf",
		ColorClass: "text-error dark:text-error-container",
		MimeType:   "application/pdf",
	},
	".xlsx": {
		Category:   CategoryBinary,
		Label:      "Hoja de Cálculo",
		Icon:       "table_chart",
		ColorClass: "text-emerald-600 dark:text-emerald-400",
		MimeType:   "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
	},
	".xls": {
		Category:   CategoryBinary,
		Label:      "Hoja de Cálculo",
		Icon:       "table_chart",
		ColorClass: "text-emerald-600 dark:text-emerald-400",
		MimeType:   "application/vnd.ms-excel",
	},
	".csv": {
		Category:      CategoryCode,
		Label:         "CSV Document",
		Icon:          "table_chart",
		ColorClass:    "text-emerald-600 dark:text-emerald-400",
		LanguageClass: "language-plaintext",
		MimeType:      "text/csv",
	},
	".doc": {
		Category:   CategoryBinary,
		Label:      "Documento de Texto",
		Icon:       "article",
		ColorClass: "text-blue-600 dark:text-blue-400",
		MimeType:   "application/msword",
	},
	".docx": {
		Category:   CategoryBinary,
		Label:      "Documento de Texto",
		Icon:       "article",
		ColorClass: "text-blue-600 dark:text-blue-400",
		MimeType:   "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
	},
	".odt": {
		Category:   CategoryBinary,
		Label:      "Documento OpenDocument",
		Icon:       "article",
		ColorClass: "text-blue-600 dark:text-blue-400",
		MimeType:   "application/vnd.oasis.opendocument.text",
	},
	".mp4": {
		Category:   CategoryVideo,
		Label:      "Archivo de Video MP4",
		Icon:       "movie",
		ColorClass: "text-purple-600 dark:text-purple-400",
		MimeType:   "video/mp4",
	},
	".mov": {
		Category:   CategoryVideo,
		Label:      "Archivo de Video QuickTime",
		Icon:       "movie",
		ColorClass: "text-purple-600 dark:text-purple-400",
		MimeType:   "video/quicktime",
	},
	".webm": {
		Category:   CategoryVideo,
		Label:      "Archivo de Video WebM",
		Icon:       "movie",
		ColorClass: "text-purple-600 dark:text-purple-400",
		MimeType:   "video/webm",
	},
	".mkv": {
		Category:   CategoryVideo,
		Label:      "Archivo de Video Matroska",
		Icon:       "movie",
		ColorClass: "text-purple-600 dark:text-purple-400",
		MimeType:   "video/x-matroska",
	},
	".mp3": {
		Category:   CategoryAudio,
		Label:      "Archivo de Audio MP3",
		Icon:       "audiotrack",
		ColorClass: "text-amber-600 dark:text-amber-400",
		MimeType:   "audio/mpeg",
	},
	".wav": {
		Category:   CategoryAudio,
		Label:      "Archivo de Audio WAV",
		Icon:       "audiotrack",
		ColorClass: "text-amber-600 dark:text-amber-400",
		MimeType:   "audio/wav",
	},
	".ogg": {
		Category:   CategoryAudio,
		Label:      "Archivo de Audio OGG",
		Icon:       "audiotrack",
		ColorClass: "text-amber-600 dark:text-amber-400",
		MimeType:   "audio/ogg",
	},
	".flac": {
		Category:   CategoryAudio,
		Label:      "Archivo de Audio FLAC",
		Icon:       "audiotrack",
		ColorClass: "text-amber-600 dark:text-amber-400",
		MimeType:   "audio/flac",
	},
	".m4a": {
		Category:   CategoryAudio,
		Label:      "Archivo de Audio M4A",
		Icon:       "audiotrack",
		ColorClass: "text-amber-600 dark:text-amber-400",
		MimeType:   "audio/mp4",
	},
	".zip": {
		Category:   CategoryBinary,
		Label:      "Archivo ZIP",
		Icon:       "folder_zip",
		ColorClass: "text-orange-600 dark:text-orange-400",
		MimeType:   "application/zip",
	},
	".tar": {
		Category:   CategoryBinary,
		Label:      "Archivo TAR",
		Icon:       "folder_zip",
		ColorClass: "text-orange-600 dark:text-orange-400",
		MimeType:   "application/x-tar",
	},
	".gz": {
		Category:   CategoryBinary,
		Label:      "Archivo GZ Comprimido",
		Icon:       "folder_zip",
		ColorClass: "text-orange-600 dark:text-orange-400",
		MimeType:   "application/gzip",
	},
	".rar": {
		Category:   CategoryBinary,
		Label:      "Archivo RAR",
		Icon:       "folder_zip",
		ColorClass: "text-orange-600 dark:text-orange-400",
		MimeType:   "application/vnd.rar",
	},
	".7z": {
		Category:   CategoryBinary,
		Label:      "Archivo 7-Zip",
		Icon:       "folder_zip",
		ColorClass: "text-orange-600 dark:text-orange-400",
		MimeType:   "application/x-7z-compressed",
	},
	".go": {
		Category:      CategoryCode,
		Label:         "Go Source Code",
		Icon:          "code",
		ColorClass:    "text-cyan-600 dark:text-cyan-400",
		LanguageClass: "language-go",
		MimeType:      "text/x-go",
	},
	".js": {
		Category:      CategoryCode,
		Label:         "JavaScript Source",
		Icon:          "code",
		ColorClass:    "text-yellow-600 dark:text-yellow-400",
		LanguageClass: "language-javascript",
		MimeType:      "application/javascript",
	},
	".jsx": {
		Category:      CategoryCode,
		Label:         "React JSX",
		Icon:          "code",
		ColorClass:    "text-yellow-600 dark:text-yellow-400",
		LanguageClass: "language-javascript",
		MimeType:      "text/jsx",
	},
	".ts": {
		Category:      CategoryCode,
		Label:         "TypeScript Source",
		Icon:          "code",
		ColorClass:    "text-yellow-600 dark:text-yellow-400",
		LanguageClass: "language-typescript",
		MimeType:      "application/typescript",
	},
	".tsx": {
		Category:      CategoryCode,
		Label:         "React TSX",
		Icon:          "code",
		ColorClass:    "text-yellow-600 dark:text-yellow-400",
		LanguageClass: "language-typescript",
		MimeType:      "text/tsx",
	},
	".py": {
		Category:      CategoryCode,
		Label:         "Python Script",
		Icon:          "code",
		ColorClass:    "text-blue-600 dark:text-blue-400",
		LanguageClass: "language-python",
		MimeType:      "text/x-python",
	},
	".rs": {
		Category:      CategoryCode,
		Label:         "Rust Source",
		Icon:          "code",
		ColorClass:    "text-orange-700 dark:text-orange-400",
		LanguageClass: "language-rust",
		MimeType:      "text/rust",
	},
	".html": {
		Category:      CategoryCode,
		Label:         "HTML Document",
		Icon:          "html",
		ColorClass:    "text-orange-600 dark:text-orange-400",
		LanguageClass: "language-html",
		MimeType:      "text/html",
	},
	".htm": {
		Category:      CategoryCode,
		Label:         "HTML Document",
		Icon:          "html",
		ColorClass:    "text-orange-600 dark:text-orange-400",
		LanguageClass: "language-html",
		MimeType:      "text/html",
	},
	".css": {
		Category:      CategoryCode,
		Label:         "CSS Stylesheet",
		Icon:          "css",
		ColorClass:    "text-blue-500 dark:text-blue-300",
		LanguageClass: "language-css",
		MimeType:      "text/css",
	},
	".json": {
		Category:      CategoryCode,
		Label:         "JSON Document",
		Icon:          "data_object",
		ColorClass:    "text-teal-600 dark:text-teal-400",
		LanguageClass: "language-json",
		MimeType:      "application/json",
	},
	".md": {
		Category:      CategoryMarkdown,
		Label:         "Markdown Document",
		Icon:          "description",
		ColorClass:    "text-secondary dark:text-secondary-fixed-dim",
		LanguageClass: "language-markdown",
		MimeType:      "text/markdown",
	},
	".markdown": {
		Category:      CategoryMarkdown,
		Label:         "Markdown Document",
		Icon:          "description",
		ColorClass:    "text-secondary dark:text-secondary-fixed-dim",
		LanguageClass: "language-markdown",
		MimeType:      "text/markdown",
	},
	".sh": {
		Category:      CategoryCode,
		Label:         "Shell Script",
		Icon:          "terminal",
		ColorClass:    "text-green-600 dark:text-green-400",
		LanguageClass: "language-bash",
		MimeType:      "application/x-sh",
	},
	".sql": {
		Category:      CategoryCode,
		Label:         "SQL Query",
		Icon:          "database",
		ColorClass:    "text-indigo-600 dark:text-indigo-400",
		LanguageClass: "language-sql",
		MimeType:      "application/sql",
	},
	".yml": {
		Category:      CategoryCode,
		Label:         "YAML Document",
		Icon:          "settings",
		ColorClass:    "text-purple-600 dark:text-purple-400",
		LanguageClass: "language-yaml",
		MimeType:      "application/yaml",
	},
	".yaml": {
		Category:      CategoryCode,
		Label:         "YAML Document",
		Icon:          "settings",
		ColorClass:    "text-purple-600 dark:text-purple-400",
		LanguageClass: "language-yaml",
		MimeType:      "application/yaml",
	},
	".c": {
		Category:      CategoryCode,
		Label:         "C Source Code",
		Icon:          "code",
		ColorClass:    "text-blue-700 dark:text-blue-400",
		LanguageClass: "language-cpp",
		MimeType:      "text/x-c",
	},
	".cpp": {
		Category:      CategoryCode,
		Label:         "C++ Source Code",
		Icon:          "code",
		ColorClass:    "text-blue-700 dark:text-blue-400",
		LanguageClass: "language-cpp",
		MimeType:      "text/x-c++",
	},
	".h": {
		Category:      CategoryCode,
		Label:         "C/C++ Header",
		Icon:          "code",
		ColorClass:    "text-blue-700 dark:text-blue-400",
		LanguageClass: "language-cpp",
		MimeType:      "text/x-c",
	},
	".txt": {
		Category:      CategoryCode,
		Label:         "Texto Plano",
		Icon:          "description",
		ColorClass:    "text-secondary dark:text-secondary-fixed-dim",
		LanguageClass: "language-plaintext",
		MimeType:      "text/plain",
	},
	".env": {
		Category:      CategoryCode,
		Label:         "Configuración Environment",
		Icon:          "settings",
		ColorClass:    "text-amber-700 dark:text-amber-400",
		LanguageClass: "language-bash",
		MimeType:      "text/plain",
	},
	".gitignore": {
		Category:      CategoryCode,
		Label:         "Git Ignore",
		Icon:          "settings",
		ColorClass:    "text-red-600 dark:text-red-400",
		LanguageClass: "language-plaintext",
		MimeType:      "text/plain",
	},
}

func Resolve(extension string, isDirectory bool) Definition {
	if isDirectory {
		return directoryDefinition
	}

	normalizedExt := strings.ToLower(extension)
	if definition, exists := registry[normalizedExt]; exists {
		if definition.MimeType == "" {
			definition.MimeType = mime.TypeByExtension(normalizedExt)
		}
		return definition
	}

	fallback := defaultFileDefinition
	detectedMime := mime.TypeByExtension(normalizedExt)
	if detectedMime != "" {
		fallback.MimeType = detectedMime
	}
	return fallback
}
