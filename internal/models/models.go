package models

import "time"

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
	SortBy      string
	SortOrder   string
	Breadcrumbs []Breadcrumb
	Folders     []FileInfo
	Files       []FileInfo
	ViewMode    string
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
