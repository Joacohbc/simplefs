package web_test

import (
	"io/fs"
	"testing"

	"simplefs/web"
)

func TestAssets_EmbeddedFiles(t *testing.T) {
	requiredFiles := []string{
		"templates/index.html",
		"templates/file_list.html",
		"templates/details_modal.html",
		"templates/preview_modal.html",
	}

	for _, file := range requiredFiles {
		data, err := fs.ReadFile(web.Assets, file)
		if err != nil {
			t.Errorf("failed to read embedded file %q: %v", file, err)
		}
		if len(data) == 0 {
			t.Errorf("embedded file %q is empty", file)
		}
	}
}
