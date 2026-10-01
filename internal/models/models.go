package models

import "time"

const (
	SortByName     = "name"
	SortBySize     = "size"
	SortByCreated  = "created"
	SortByModified = "modified"

	SortOrderAsc  = "asc"
	SortOrderDesc = "desc"

	ViewModeList = "list"
	ViewModeGrid = "grid"
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
	FolderBgClass    string
	FolderDotClass   string
	IsImage          bool
	IsZip            bool
	Category         string
	Extension        string
}

type Breadcrumb struct {
	Name string
	Path string
}

type PageData struct {
	Path        string
	Query       string
	SortBy      string
	SortOrder   string
	Breadcrumbs []Breadcrumb
	Folders     []FileInfo
	Files       []FileInfo
	ViewMode     string
	Lang         string
	NotFound     bool
	NotFoundPath string
}

func (p PageData) TotalItems() int {
	return len(p.Folders) + len(p.Files)
}

func (p PageData) HasItems() bool {
	return len(p.Folders) > 0 || len(p.Files) > 0
}

type PreviewData struct {
	Name          string
	RelPath       string
	Type          string
	Extension     string
	MimeType      string
	Content       string
	FormattedSize string
	ModTime       string
	LineCount     int
	LanguageClass string
	MaterialIcon  string
	Lang          string
}

func (p PreviewData) HasContent() bool {
	return p.Content != ""
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
	Lang          string
}
