package i18n_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"simplefs/internal/i18n"
)

func TestNormalizeLang(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"en", i18n.LangEN},
		{"EN", i18n.LangEN},
		{"en-US", i18n.LangEN},
		{"en_GB", i18n.LangEN},
		{"es", i18n.LangES},
		{"ES", i18n.LangES},
		{"es-ES", i18n.LangES},
		{"es-MX", i18n.LangES},
		{"es_AR", i18n.LangES},
		{"fr", i18n.LangES},
		{"de", i18n.LangES},
		{"", i18n.LangES},
		{"  en  ", i18n.LangEN},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := i18n.NormalizeLang(tt.input)
			if result != tt.expected {
				t.Errorf("NormalizeLang(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestResolveLang(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		formLang string
		cookie   *http.Cookie
		accept   string
		expected string
	}{
		{
			name:     "query param takes highest precedence",
			url:      "/?lang=en",
			formLang: "es",
			cookie:   &http.Cookie{Name: "lang", Value: "es"},
			accept:   "es-ES,es;q=0.9",
			expected: i18n.LangEN,
		},
		{
			name:     "form value takes precedence over cookie and header",
			url:      "/",
			formLang: "en",
			cookie:   &http.Cookie{Name: "lang", Value: "es"},
			accept:   "es-ES,es;q=0.9",
			expected: i18n.LangEN,
		},
		{
			name:     "cookie takes precedence over Accept-Language header",
			url:      "/",
			cookie:   &http.Cookie{Name: "lang", Value: "en"},
			accept:   "es-ES,es;q=0.9",
			expected: i18n.LangEN,
		},
		{
			name:     "accept-language header english resolved",
			url:      "/",
			accept:   "en-US,en;q=0.9,es;q=0.8",
			expected: i18n.LangEN,
		},
		{
			name:     "accept-language header spanish resolved",
			url:      "/",
			accept:   "es-AR,es;q=0.9,en;q=0.8",
			expected: i18n.LangES,
		},
		{
			name:     "accept-language header unsupported falls back to default",
			url:      "/",
			accept:   "fr-FR,fr;q=0.9,de;q=0.8",
			expected: i18n.LangES,
		},
		{
			name:     "empty request falls back to default",
			url:      "/",
			expected: i18n.LangES,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var bodyReader *strings.Reader
			if tt.formLang != "" {
				form := url.Values{}
				form.Set("lang", tt.formLang)
				bodyReader = strings.NewReader(form.Encode())
			} else {
				bodyReader = strings.NewReader("")
			}

			req := httptest.NewRequest(http.MethodPost, tt.url, bodyReader)
			if tt.formLang != "" {
				req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			}
			if tt.cookie != nil {
				req.AddCookie(tt.cookie)
			}
			if tt.accept != "" {
				req.Header.Set("Accept-Language", tt.accept)
			}

			result := i18n.ResolveLang(req)
			if result != tt.expected {
				t.Errorf("ResolveLang() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestT(t *testing.T) {
	t.Run("existing keys in spanish and english", func(t *testing.T) {
		esTitle := i18n.T(i18n.LangES, "app.title")
		if esTitle != "simplefs - Explorador de Archivos" {
			t.Errorf("unexpected ES title: %q", esTitle)
		}

		enTitle := i18n.T(i18n.LangEN, "app.title")
		if enTitle != "simplefs - File Manager" {
			t.Errorf("unexpected EN title: %q", enTitle)
		}
	})

	t.Run("formatted strings with arguments", func(t *testing.T) {
		esFormatted := i18n.T(i18n.LangES, "folders.title", 5)
		if esFormatted != "Carpetas (5)" {
			t.Errorf("expected 'Carpetas (5)', got %q", esFormatted)
		}

		enFormatted := i18n.T(i18n.LangEN, "folders.title", 5)
		if enFormatted != "Folders (5)" {
			t.Errorf("expected 'Folders (5)', got %q", enFormatted)
		}

		deletePrompt := i18n.T(i18n.LangES, "folders.delete_confirm", "docs")
		if deletePrompt != "¿Seguro que deseas eliminar la carpeta 'docs' y todo su contenido?" {
			t.Errorf("unexpected delete prompt: %q", deletePrompt)
		}
	})

	t.Run("missing key returns key itself", func(t *testing.T) {
		missing := i18n.T(i18n.LangES, "unknown.translation.key")
		if missing != "unknown.translation.key" {
			t.Errorf("expected key itself, got %q", missing)
		}
	})
}

func TestFormatDateAndDateTime(t *testing.T) {
	testDate := time.Date(2026, time.March, 15, 14, 30, 0, 0, time.UTC)

	esDate := i18n.FormatDate(testDate, i18n.LangES)
	if esDate != "15 Mar 2026" {
		t.Errorf("expected '15 Mar 2026', got %q", esDate)
	}

	enDate := i18n.FormatDate(testDate, i18n.LangEN)
	if enDate != "15 Mar 2026" {
		t.Errorf("expected '15 Mar 2026', got %q", enDate)
	}

	esDateTime := i18n.FormatDateTime(testDate, i18n.LangES)
	if esDateTime != "15 Mar 2026, 14:30" {
		t.Errorf("expected '15 Mar 2026, 14:30', got %q", esDateTime)
	}

	enDateTime := i18n.FormatDateTime(testDate, i18n.LangEN)
	if enDateTime != "15 Mar 2026, 14:30" {
		t.Errorf("expected '15 Mar 2026, 14:30', got %q", enDateTime)
	}
}
