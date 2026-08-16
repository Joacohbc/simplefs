package middleware_test

import (
	"net/http"
	"net/http/httptest"
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
		"Referrer-Policy":        "strict-origin-when-cross-origin",
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
			name:           "DELETE request cross referer forbidden",
			method:         http.MethodDelete,
			host:           "localhost:8080",
			referer:        "https://evil.com/attack",
			expectedStatus: http.StatusForbidden,
		},
	}

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
