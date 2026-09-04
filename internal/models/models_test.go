package models_test

import (
	"testing"

	"simplefs/internal/models"
)

func TestPageData_TotalItems(t *testing.T) {
	emptyPage := models.PageData{}
	if emptyPage.TotalItems() != 0 {
		t.Errorf("expected 0 total items, got %d", emptyPage.TotalItems())
	}

	page := models.PageData{
		Folders: []models.FileInfo{{Name: "docs", IsDir: true}},
		Files:   []models.FileInfo{{Name: "a.txt"}, {Name: "b.txt"}},
	}
	if page.TotalItems() != 3 {
		t.Errorf("expected 3 total items, got %d", page.TotalItems())
	}
}

func TestPageData_HasItems(t *testing.T) {
	emptyPage := models.PageData{}
	if emptyPage.HasItems() {
		t.Error("expected empty page HasItems to be false")
	}

	folderOnlyPage := models.PageData{
		Folders: []models.FileInfo{{Name: "docs", IsDir: true}},
	}
	if !folderOnlyPage.HasItems() {
		t.Error("expected folderOnlyPage HasItems to be true")
	}

	fileOnlyPage := models.PageData{
		Files: []models.FileInfo{{Name: "readme.md"}},
	}
	if !fileOnlyPage.HasItems() {
		t.Error("expected fileOnlyPage HasItems to be true")
	}
}

func TestPreviewData_HasContent(t *testing.T) {
	emptyPreview := models.PreviewData{}
	if emptyPreview.HasContent() {
		t.Error("expected empty preview HasContent to be false")
	}

	contentPreview := models.PreviewData{Content: "package main"}
	if !contentPreview.HasContent() {
		t.Error("expected content preview HasContent to be true")
	}
}
