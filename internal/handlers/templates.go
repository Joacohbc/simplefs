package handlers

import (
	"errors"
	"html/template"
	"io/fs"
	"net/url"

	"simplefs/internal/assets"
	"simplefs/internal/i18n"
)

var errOddDictArguments = errors.New("dict expects key/value pairs")
var errNonStringDictKey = errors.New("dict keys must be strings")

func increment(value int) int {
	return value + 1
}

func stringList(values ...string) []string {
	return values
}

func templateDict(keysAndValues ...any) (map[string]any, error) {
	if len(keysAndValues)%2 != 0 {
		return nil, errOddDictArguments
	}
	dict := make(map[string]any, len(keysAndValues)/2)
	for index := 0; index < len(keysAndValues); index += 2 {
		key, isString := keysAndValues[index].(string)
		if !isString {
			return nil, errNonStringDictKey
		}
		dict[key] = keysAndValues[index+1]
	}
	return dict, nil
}

// TemplateFuncs returns the helpers available to every page template.
func TemplateFuncs(staticAssets *assets.Bundle) template.FuncMap {
	funcs := template.FuncMap{
		"T":    i18n.T,
		"q":    url.QueryEscape,
		"inc":  increment,
		"list": stringList,
		"dict": templateDict,
	}
	for name, function := range staticAssets.FuncMap() {
		funcs[name] = function
	}
	return funcs
}

func ParseTemplates(fsys fs.FS, staticAssets *assets.Bundle) (*template.Template, error) {
	return template.New("").Funcs(TemplateFuncs(staticAssets)).ParseFS(fsys, "templates/*.html")
}
