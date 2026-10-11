package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"simplefs/internal/middleware"
)

func TestSecurityHeadersMiddleware(t *testing.T) {
	dummyHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	secured := middleware.NewSecurityHeadersMiddleware(dummyHandler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	secured.ServeHTTP(rec, req)

	headers := map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "SAMEORIGIN",
		"Referrer-Policy":        "same-origin",
	}

	for header, expected := range headers {
		if got := rec.Header().Get(header); got != expected {
			t.Errorf("header %q: got %q, want %q", header, got, expected)
		}
	}

	csp := rec.Header().Get("Content-Security-Policy")
	if csp == "" {
		t.Error("expected Content-Security-Policy header, got empty")
	}
	if strings.Contains(csp, "unsafe-inline") || strings.Contains(csp, "unsafe-eval") {
		t.Errorf("CSP must not allow inline or eval scripts: %s", csp)
	}
}

func TestCSRFProtectionMiddleware(t *testing.T) {
	dummyHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	csrfMiddleware := middleware.CSRFProtectionMiddleware(dummyHandler)

	tests := []struct {
		name           string
		method         string
		host           string
		origin         string
		referer        string
		expectedStatus int
	}{
		{
			name:           "GET request without headers allowed",
			method:         http.MethodGet,
			host:           "localhost:8080",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "POST request same origin allowed",
			method:         http.MethodPost,
			host:           "localhost:8080",
			origin:         "http://localhost:8080",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "POST request cross origin forbidden",
			method:         http.MethodPost,
			host:           "localhost:8080",
			origin:         "https://evil.com",
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "DELETE request same referer allowed",
			method:         http.MethodDelete,
			host:           "localhost:8080",
			referer:        "http://localhost:8080/?path=docs",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "PUT without Origin or Referer allowed (API clients)",
			method:         http.MethodPut,
			host:           "localhost:8080",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "POST with opaque null origin forbidden",
			method:         http.MethodPost,
			host:           "localhost:8080",
			origin:         "null",
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "DELETE request cross referer forbidden",
			method:         http.MethodDelete,
			host:           "localhost:8080",
			referer:        "https://evil.com/attack",
			expectedStatus: http.StatusForbidden,
		},
	}

	t.Run("Sec-Fetch-Site cross-site forbidden", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/upload", nil)
		req.Header.Set("Sec-Fetch-Site", "cross-site")
		rec := httptest.NewRecorder()
		csrfMiddleware.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Errorf("got status %d, want 403", rec.Code)
		}
	})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/api/create-file", nil)
			req.Host = tt.host
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}
			if tt.referer != "" {
				req.Header.Set("Referer", tt.referer)
			}

			rec := httptest.NewRecorder()
			csrfMiddleware.ServeHTTP(rec, req)

			if rec.Code != tt.expectedStatus {
				t.Errorf("got status %d, want %d", rec.Code, tt.expectedStatus)
			}
		})
	}
}

func TestHostGuard(t *testing.T) {
	guard := middleware.NewHostGuard([]string{"files.example.com", ".ts.net"})
	cases := map[string]bool{
		"localhost:8080":         true,
		"127.0.0.1:8080":         true,
		"[::1]:8080":             true,
		"100.101.102.103":        true,
		"files.example.com":      true,
		"FILES.example.com:8080": true,
		"my-pc.tailnet.ts.net":   true,
		"evil.example":           false,
		"files.example.com.evil": false,
		"":                       false,
	}
	for host, allowed := range cases {
		if got := guard.Allowed(host); got != allowed {
			t.Errorf("Allowed(%q) = %v, want %v", host, got, allowed)
		}
	}

	handler := guard.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Host = "rebind.attacker.example"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 for unknown host, got %d", rec.Code)
	}

	if !middleware.NewHostGuard([]string{"*"}).Allowed("anything.example") {
		t.Error("wildcard should allow any host")
	}
}
