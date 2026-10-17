package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type Role int

const (
	RoleNone Role = iota
	RoleRead
	RoleWrite
)

func (r Role) String() string {
	switch r {
	case RoleRead:
		return "read"
	case RoleWrite:
		return "write"
	}
	return "none"
}

const (
	SessionCookieName = "simplefs_session"
	sessionMaxAge     = 30 * 24 * time.Hour
	maxLoginFailures  = 10
	loginWindow       = 15 * time.Minute
	failedLoginDelay  = 400 * time.Millisecond
	tokenEntropyBytes = 24
	privateDirMode    = 0700
	privateFileMode   = 0600
)

type contextKey struct{}

// Authenticator validates bearer tokens and signed session cookies.
type Authenticator struct {
	enabled    bool
	writeToken []byte
	readToken  []byte
	secret     []byte
	limiter    *loginLimiter
}

func New(writeToken, readToken string) *Authenticator {
	writeToken = strings.TrimSpace(writeToken)
	readToken = strings.TrimSpace(readToken)
	secret := sha256.Sum256([]byte("simplefs-session|" + writeToken + "|" + readToken))
	return &Authenticator{
		enabled:    writeToken != "",
		writeToken: []byte(writeToken),
		readToken:  []byte(readToken),
		secret:     secret[:],
		limiter:    newLoginLimiter(),
	}
}

// Disabled returns an authenticator that grants full access to every request.
func Disabled() *Authenticator {
	return &Authenticator{limiter: newLoginLimiter()}
}

func (a *Authenticator) Enabled() bool {
	return a.enabled
}

func (a *Authenticator) RoleForToken(token string) Role {
	if !a.enabled {
		return RoleWrite
	}
	candidate := []byte(strings.TrimSpace(token))
	if len(candidate) == 0 {
		return RoleNone
	}
	if subtle.ConstantTimeCompare(candidate, a.writeToken) == 1 {
		return RoleWrite
	}
	if len(a.readToken) > 0 && subtle.ConstantTimeCompare(candidate, a.readToken) == 1 {
		return RoleRead
	}
	return RoleNone
}

func (a *Authenticator) sessionValue(role Role) string {
	mac := hmac.New(sha256.New, a.secret)
	fmt.Fprintf(mac, "session:%d", role)
	return role.String() + "." + hex.EncodeToString(mac.Sum(nil))
}

func (a *Authenticator) roleForSession(value string) Role {
	for _, role := range []Role{RoleWrite, RoleRead} {
		if role == RoleRead && len(a.readToken) == 0 {
			continue
		}
		if hmac.Equal([]byte(value), []byte(a.sessionValue(role))) {
			return role
		}
	}
	return RoleNone
}

func (a *Authenticator) RoleFromRequest(r *http.Request) Role {
	if !a.enabled {
		return RoleWrite
	}
	if header := r.Header.Get("Authorization"); header != "" {
		scheme, token, found := strings.Cut(header, " ")
		if found && strings.EqualFold(scheme, "Bearer") {
			return a.RoleForToken(token)
		}
		return RoleNone
	}
	if cookie, err := r.Cookie(SessionCookieName); err == nil {
		return a.roleForSession(cookie.Value)
	}
	return RoleNone
}

func WithRole(ctx context.Context, role Role) context.Context {
	return context.WithValue(ctx, contextKey{}, role)
}

func RoleFrom(ctx context.Context) Role {
	if role, ok := ctx.Value(contextKey{}).(Role); ok {
		return role
	}
	return RoleNone
}

func isPublicPath(path string) bool {
	switch path {
	case "/healthz", "/login", "/favicon.ico", "/apple-touch-icon.png":
		return true
	}
	return strings.HasPrefix(path, "/static/")
}

func isReadMethod(method string) bool {
	return method == http.MethodGet || method == http.MethodHead || method == http.MethodOptions
}

func wantsHTML(r *http.Request) bool {
	if r.Method != http.MethodGet || r.Header.Get("HX-Request") != "" {
		return false
	}
	if strings.HasPrefix(r.URL.Path, "/api/") || r.URL.Path == "/download" {
		return false
	}
	return strings.Contains(r.Header.Get("Accept"), "text/html")
}

// Middleware rejects unauthenticated requests and read-only sessions that try to write.
func (a *Authenticator) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isPublicPath(r.URL.Path) || (r.URL.Path == "/logout" && r.Method == http.MethodPost) {
			next.ServeHTTP(w, r.WithContext(WithRole(r.Context(), a.RoleFromRequest(r))))
			return
		}

		role := a.RoleFromRequest(r)
		if role == RoleNone {
			a.rejectUnauthenticated(w, r)
			return
		}
		if role == RoleRead && !isReadMethod(r.Method) {
			writeError(w, r, http.StatusForbidden, "read-only token: write operations are not allowed")
			return
		}
		next.ServeHTTP(w, r.WithContext(WithRole(r.Context(), role)))
	})
}

func (a *Authenticator) rejectUnauthenticated(w http.ResponseWriter, r *http.Request) {
	if wantsHTML(r) {
		http.Redirect(w, r, "/login?next="+url.QueryEscape(r.URL.RequestURI()), http.StatusSeeOther)
		return
	}
	if r.Header.Get("HX-Request") != "" {
		w.Header().Set("HX-Redirect", "/login")
	}
	w.Header().Set("WWW-Authenticate", `Bearer realm="simplefs"`)
	writeError(w, r, http.StatusUnauthorized, "authentication required")
}

func writeError(w http.ResponseWriter, r *http.Request, status int, message string) {
	if strings.HasPrefix(r.URL.Path, "/api/v1/") {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		fmt.Fprintf(w, "{\"error\":%q}\n", message)
		return
	}
	http.Error(w, message, status)
}

// Login validates token and returns the session cookie to set.
func (a *Authenticator) Login(r *http.Request, token string) (*http.Cookie, error) {
	clientIP := ClientIP(r)
	if !a.limiter.allow(clientIP) {
		return nil, ErrTooManyAttempts
	}
	role := a.RoleForToken(token)
	if role == RoleNone {
		a.limiter.fail(clientIP)
		time.Sleep(failedLoginDelay)
		return nil, ErrInvalidToken
	}
	a.limiter.reset(clientIP)
	return &http.Cookie{
		Name:     SessionCookieName,
		Value:    a.sessionValue(role),
		Path:     "/",
		MaxAge:   int(sessionMaxAge.Seconds()),
		HttpOnly: true,
		Secure:   IsSecureRequest(r),
		SameSite: http.SameSiteStrictMode,
	}, nil
}

func LogoutCookie(r *http.Request) *http.Cookie {
	return &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   IsSecureRequest(r),
		SameSite: http.SameSiteStrictMode,
	}
}

var (
	ErrInvalidToken    = errors.New("invalid token")
	ErrTooManyAttempts = errors.New("too many failed attempts, try again later")
)

func IsSecureRequest(r *http.Request) bool {
	return r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

func ClientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// SafeRedirectTarget only accepts same-site absolute paths.
func SafeRedirectTarget(next string) string {
	if next == "" || !strings.HasPrefix(next, "/") || strings.HasPrefix(next, "//") || strings.HasPrefix(next, "/\\") {
		return "/"
	}
	return next
}

type loginLimiter struct {
	mu       sync.Mutex
	failures map[string]*failureWindow
}

type failureWindow struct {
	count int
	start time.Time
}

func newLoginLimiter() *loginLimiter {
	return &loginLimiter{failures: make(map[string]*failureWindow)}
}

func (l *loginLimiter) allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	window, ok := l.failures[ip]
	if !ok {
		return true
	}
	if time.Since(window.start) > loginWindow {
		delete(l.failures, ip)
		return true
	}
	return window.count < maxLoginFailures
}

func (l *loginLimiter) fail(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	window, ok := l.failures[ip]
	if !ok || time.Since(window.start) > loginWindow {
		l.failures[ip] = &failureWindow{count: 1, start: time.Now()}
		return
	}
	window.count++
}

func (l *loginLimiter) reset(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.failures, ip)
}

// GenerateToken returns a random URL-safe token.
func GenerateToken() (string, error) {
	randomBytes := make([]byte, tokenEntropyBytes)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(randomBytes), nil
}

// LoadOrCreateToken reads the token stored at path, creating a new one when missing.
// The bool result reports whether a new token was generated.
func LoadOrCreateToken(path string) (string, bool, error) {
	if storedToken, err := os.ReadFile(path); err == nil {
		if token := strings.TrimSpace(string(storedToken)); token != "" {
			return token, false, nil
		}
	}
	token, err := GenerateToken()
	if err != nil {
		return "", false, err
	}
	if err := os.MkdirAll(filepath.Dir(path), privateDirMode); err != nil {
		return "", false, err
	}
	if err := os.WriteFile(path, []byte(token+"\n"), privateFileMode); err != nil {
		return "", false, err
	}
	return token, true, nil
}
