package storage

import (
	"strings"
	"unicode"

	"simplefs/internal/filetype"
)

type folderStyle struct {
	icon   string
	accent filetype.Accent
}

var defaultFolderStyle = folderStyle{icon: "folder", accent: filetype.AccentPrimary}

var folderStylesByWord = buildFolderStyleIndex(map[folderStyle][]string{
	{icon: "move_to_inbox", accent: filetype.AccentPrimary}: {"inbox", "incoming", "entrada", "recibidos"},
	{icon: "outbox", accent: filetype.AccentPrimary}:        {"outbox", "outgoing", "salida", "enviados"},
	{icon: "download", accent: filetype.AccentPrimary}:      {"downloads", "download", "descargas"},
	{icon: "folder_code", accent: filetype.AccentCode}:      {"src", "source", "code", "cmd", "pkg", "internal", "lib", "app", "apps"},
	{icon: "perm_media", accent: filetype.AccentMedia}:      {"assets", "asset", "media", "img", "imgs", "image", "images", "photo", "photos", "pictures", "static", "video", "videos"},
	{icon: "topic", accent: filetype.AccentSheet}:           {"docs", "doc", "documents", "documentos", "guide", "guides", "spec", "specs", "manual", "notes", "wiki"},
	{icon: "terminal", accent: filetype.AccentArchive}:      {"build", "dist", "bin", "out", "output", "script", "scripts", "target", "release", "releases"},
})

func buildFolderStyleIndex(wordsByStyle map[folderStyle][]string) map[string]folderStyle {
	index := make(map[string]folderStyle)
	for style, words := range wordsByStyle {
		for _, word := range words {
			index[word] = style
		}
	}
	return index
}

func styleForFolder(name string) folderStyle {
	isWordSeparator := func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) }
	for _, word := range strings.FieldsFunc(strings.ToLower(name), isWordSeparator) {
		if style, ok := folderStylesByWord[word]; ok {
			return style
		}
	}
	return defaultFolderStyle
}
