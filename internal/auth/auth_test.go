package auth_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"simplefs/internal/auth"
)

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(auth.RoleFrom(r.Context()).String()))
	})
}

func TestRoleForToken(t *testing.T) {
	authenticator := auth.New("write-token", "read-token")
	cases := map[string]auth.Role{
		"write-token": auth.RoleWrite,
		"read-token":  auth.RoleRead,
		"wrong":       auth.RoleNone,
		"":            auth.RoleNone,
	}
	for token, expected := range cases {
		if got := authenticator.RoleForToken(token); got != expected {
			t.Errorf("RoleForToken(%q) = %v, want %v", token, got, expected)
		}
	}
	if auth.Disabled().RoleForToken("") != auth.RoleWrite {
		t.Error("disabled authenticator should grant write access")
	}
}

func TestMiddleware(t *testing.T) {
	authenticator := auth.New("write-token", "read-token")
	handler := authenticator.Middleware(okHandler())

	tests := []struct {
		name         string
		method       string
		path         string
		header       string
		accept       string
		expectStatus int
		expectBody   string
	}{
		{"health is public", http.MethodGet, "/healthz", "", "", http.StatusOK, "none"},
		{"static is public", http.MethodGet, "/static/style.css", "", "", http.StatusOK, "none"},
		{"api without token", http.MethodGet, "/api/v1/whoami", "", "", http.StatusUnauthorized, ""},
		{"browser redirected to login", http.MethodGet, "/", "", "text/html", http.StatusSeeOther, ""},
		{"wrong token", http.MethodGet, "/api/v1/whoami", "Bearer nope", "", http.StatusUnauthorized, ""},
		{"basic scheme rejected", http.MethodGet, "/api/v1/whoami", "Basic write-token", "", http.StatusUnauthorized, ""},
		{"write token", http.MethodPut, "/api/v1/files/a.txt", "Bearer write-token", "", http.StatusOK, "write"},
		{"read token can read", http.MethodGet, "/api/v1/files/a.txt", "Bearer read-token", "", http.StatusOK, "read"},
		{"read token cannot write", http.MethodPut, "/api/v1/files/a.txt", "Bearer read-token", "", http.StatusForbidden, ""},
		{"read token cannot delete", http.MethodDelete, "/api/delete", "Bearer read-token", "", http.StatusForbidden, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			if tt.header != "" {
				req.Header.Set("Authorization", tt.header)
			}
			if tt.accept != "" {
				req.Header.Set("Accept", tt.accept)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != tt.expectStatus {
				t.Fatalf("status %d, want %d (%s)", rec.Code, tt.expectStatus, rec.Body.String())
			}
			if tt.expectBody != "" && rec.Body.String() != tt.expectBody {
				t.Errorf("body %q, want %q", rec.Body.String(), tt.expectBody)
			}
		})
	}
}

func TestLoginCookieGrantsRole(t *testing.T) {
	authenticator := auth.New("write-token", "read-token")
	req := httptest.NewRequest(http.MethodPost, "/login", nil)
	req.RemoteAddr = "10.0.0.1:1234"

	cookie, err := authenticator.Login(req, "read-token")
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}
	if !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode {
		t.Errorf("session cookie must be HttpOnly and SameSite=Strict: %+v", cookie)
	}
	if cookie.Value == "read-token" {
		t.Error("cookie must not contain the raw token")
	}

	next := httptest.NewRequest(http.MethodGet, "/", nil)
	next.AddCookie(cookie)
	if role := authenticator.RoleFromRequest(next); role != auth.RoleRead {
		t.Errorf("expected read role from cookie, got %v", role)
	}

	forged := httptest.NewRequest(http.MethodGet, "/", nil)
	forged.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: "write.deadbeef"})
	if role := authenticator.RoleFromRequest(forged); role != auth.RoleNone {
		t.Errorf("forged cookie accepted with role %v", role)
	}

	rotated := auth.New("new-write-token", "")
	if role := rotated.RoleFromRequest(next); role != auth.RoleNone {
		t.Error("sessions must be invalidated when tokens change")
	}
}

func TestLoginRateLimit(t *testing.T) {
	authenticator := auth.New("write-token", "")
	req := httptest.NewRequest(http.MethodPost, "/login", nil)
	req.RemoteAddr = "10.0.0.2:1234"
	for attempt := 0; attempt < 10; attempt++ {
		if _, err := authenticator.Login(req, "bad"); err != auth.ErrInvalidToken {
			t.Fatalf("attempt %d: expected ErrInvalidToken, got %v", attempt, err)
		}
	}
	if _, err := authenticator.Login(req, "write-token"); err != auth.ErrTooManyAttempts {
		t.Errorf("expected lockout after repeated failures, got %v", err)
	}
}

func TestSafeRedirectTarget(t *testing.T) {
	cases := map[string]string{
		"":                      "/",
		"/?path=docs":           "/?path=docs",
		"//evil.example":        "/",
		"/\\evil.example":       "/",
		"https://evil.example/": "/",
	}
	for input, expected := range cases {
		if got := auth.SafeRedirectTarget(input); got != expected {
			t.Errorf("SafeRedirectTarget(%q) = %q, want %q", input, got, expected)
		}
	}
}

func TestLoadOrCreateToken(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".simplefs", "token")
	token, created, err := auth.LoadOrCreateToken(path)
	if err != nil || !created || len(token) < 32 {
		t.Fatalf("expected new token, got %q created=%v err=%v", token, created, err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0077 != 0 {
		t.Errorf("token file must not be readable by others: %v", info.Mode().Perm())
	}
	again, created, err := auth.LoadOrCreateToken(path)
	if err != nil || created || again != token {
		t.Errorf("expected the stored token to be reused, got %q created=%v err=%v", again, created, err)
	}
}
