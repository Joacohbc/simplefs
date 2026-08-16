package middleware

import (
	"net/http"
	"net/url"
)

func NewSecurityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "SAMEORIGIN")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; font-src 'self'; img-src 'self' data: blob:; media-src 'self' data: blob:; frame-src 'self'; frame-ancestors 'self'; base-uri 'self'; form-action 'self'; object-src 'none';")
		next.ServeHTTP(w, r)
	})
}

func CSRFProtectionMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost || r.Method == http.MethodDelete || r.Method == http.MethodPut || r.Method == http.MethodPatch {
			origin := r.Header.Get("Origin")
			referer := r.Header.Get("Referer")
			host := r.Host

			if origin != "" {
				parsedOrigin, err := url.Parse(origin)
				if err != nil || parsedOrigin.Host != host {
					http.Error(w, "Forbidden: Cross-origin request rejected", http.StatusForbidden)
					return
				}
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
