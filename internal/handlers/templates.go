package handlers

import (
	"errors"
	"html/template"
	"io/fs"
	"net/url"

	"simplefs/internal/assets"
	"simplefs/internal/i18n"
)

// TemplateFuncs returns the helpers available to every page template.
func TemplateFuncs(staticAssets *assets.Assets) template.FuncMap {
	funcs := template.FuncMap{
		"T": i18n.T,
		"q": url.QueryEscape,
		"inc": func(value int) int {
			return value + 1
		},
		"list": func(values ...string) []string {
			return values
		},
		"dict": func(pairs ...any) (map[string]any, error) {
			if len(pairs)%2 != 0 {
				return nil, errors.New("dict expects key/value pairs")
			}
			result := make(map[string]any, len(pairs)/2)
			for index := 0; index < len(pairs); index += 2 {
				key, ok := pairs[index].(string)
				if !ok {
					return nil, errors.New("dict keys must be strings")
				}
				result[key] = pairs[index+1]
			}
			return result, nil
		},
	}
	for name, fn := range staticAssets.FuncMap() {
		funcs[name] = fn
	}
	return funcs
}

func ParseTemplates(fsys fs.FS, staticAssets *assets.Assets) (*template.Template, error) {
	return template.New("").Funcs(TemplateFuncs(staticAssets)).ParseFS(fsys, "templates/*.html")
}
