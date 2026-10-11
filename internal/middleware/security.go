package middleware

import (
	"net"
	"net/http"
	"net/url"
	"strings"
)

const ContentSecurityPolicy = "default-src 'self'; script-src 'self'; style-src 'self'; font-src 'self'; img-src 'self' data: blob:; media-src 'self' blob:; frame-src 'self'; frame-ancestors 'self'; base-uri 'self'; form-action 'self'; object-src 'none'"

func NewSecurityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "SAMEORIGIN")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		w.Header().Set("Cross-Origin-Opener-Policy", "same-origin")
		w.Header().Set("Content-Security-Policy", ContentSecurityPolicy)
		next.ServeHTTP(w, r)
	})
}

func isUnsafeMethod(method string) bool {
	return method == http.MethodPost || method == http.MethodDelete || method == http.MethodPut || method == http.MethodPatch
}

func CSRFProtectionMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isUnsafeMethod(r.Method) {
			if r.Header.Get("Sec-Fetch-Site") == "cross-site" {
				http.Error(w, "Forbidden: Cross-site request rejected", http.StatusForbidden)
				return
			}

			origin := r.Header.Get("Origin")
			referer := r.Header.Get("Referer")
			host := r.Host

			if origin != "" && origin != "null" {
				parsedOrigin, err := url.Parse(origin)
				if err != nil || parsedOrigin.Host != host {
					http.Error(w, "Forbidden: Cross-origin request rejected", http.StatusForbidden)
					return
				}
			} else if origin == "null" {
				http.Error(w, "Forbidden: Opaque origin rejected", http.StatusForbidden)
				return
			} else if referer != "" {
				parsedReferer, err := url.Parse(referer)
				if err != nil || parsedReferer.Host != host {
					http.Error(w, "Forbidden: Invalid referer", http.StatusForbidden)
					return
				}
			}
		}
		next.ServeHTTP(w, r)
	})
}

// HostGuard defends against DNS rebinding: a browser tricked into resolving an
// attacker domain to this server sends that domain as Host. IP literals and
// localhost are always accepted; any other name must be listed in allowedHosts.
// A "*" entry disables the check. Entries starting with "." match subdomains.
type HostGuard struct {
	allowAll bool
	exact    map[string]bool
	suffixes []string
}

func NewHostGuard(allowedHosts []string) *HostGuard {
	guard := &HostGuard{exact: make(map[string]bool)}
	for _, entry := range allowedHosts {
		host := strings.ToLower(strings.TrimSpace(entry))
		if host == "" {
			continue
		}
		if host == "*" {
			guard.allowAll = true
			continue
		}
		if strings.HasPrefix(host, ".") {
			guard.suffixes = append(guard.suffixes, host)
			continue
		}
		guard.exact[stripPort(host)] = true
	}
	return guard
}

func stripPort(hostport string) string {
	if host, _, err := net.SplitHostPort(hostport); err == nil {
		return strings.Trim(host, "[]")
	}
	return strings.Trim(hostport, "[]")
}

func (g *HostGuard) Allowed(hostport string) bool {
	if g.allowAll {
		return true
	}
	host := strings.ToLower(stripPort(hostport))
	if host == "" {
		return false
	}
	if net.ParseIP(host) != nil || host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return true
	}
	if g.exact[host] {
		return true
	}
	for _, suffix := range g.suffixes {
		if strings.HasSuffix(host, suffix) {
			return true
		}
	}
	return false
}

func (g *HostGuard) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !g.Allowed(r.Host) {
			http.Error(w, "Forbidden: host not allowed (add it to ALLOWED_HOSTS / --allowed-hosts)", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}
