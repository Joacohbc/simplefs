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

func hasSameHost(rawURL, host string) bool {
	parsed, err := url.Parse(rawURL)
	return err == nil && parsed.Host == host
}

// crossOriginRejection explains why a state-changing request must be rejected,
// or returns "" when it comes from this origin or from a non-browser client.
func crossOriginRejection(r *http.Request) string {
	origin := r.Header.Get("Origin")
	referer := r.Header.Get("Referer")
	switch {
	case r.Header.Get("Sec-Fetch-Site") == "cross-site":
		return "Forbidden: Cross-site request rejected"
	case origin == "null":
		return "Forbidden: Opaque origin rejected"
	case origin != "" && !hasSameHost(origin, r.Host):
		return "Forbidden: Cross-origin request rejected"
	case origin == "" && referer != "" && !hasSameHost(referer, r.Host):
		return "Forbidden: Invalid referer"
	}
	return ""
}

func CSRFProtectionMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isUnsafeMethod(r.Method) {
			next.ServeHTTP(w, r)
			return
		}
		if rejection := crossOriginRejection(r); rejection != "" {
			http.Error(w, rejection, http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// HostGuard defends against DNS rebinding: a browser tricked into resolving an
// attacker domain to this server sends that domain as Host. IP literals and
// localhost are always accepted; any other name must be listed in allowedHosts.
// A "*" entry disables the check. Entries starting with "." match subdomains.
type HostGuard struct {
	allowsAnyHost     bool
	exactHosts        map[string]bool
	subdomainSuffixes []string
}

func NewHostGuard(allowedHosts []string) *HostGuard {
	guard := &HostGuard{exactHosts: make(map[string]bool)}
	for _, entry := range allowedHosts {
		host := strings.ToLower(strings.TrimSpace(entry))
		if host == "" {
			continue
		}
		if host == "*" {
			guard.allowsAnyHost = true
			continue
		}
		if strings.HasPrefix(host, ".") {
			guard.subdomainSuffixes = append(guard.subdomainSuffixes, host)
			continue
		}
		guard.exactHosts[stripPort(host)] = true
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
	if g.allowsAnyHost {
		return true
	}
	host := strings.ToLower(stripPort(hostport))
	if host == "" {
		return false
	}
	if net.ParseIP(host) != nil || host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return true
	}
	if g.exactHosts[host] {
		return true
	}
	for _, suffix := range g.subdomainSuffixes {
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
