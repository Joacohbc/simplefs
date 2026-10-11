package middleware

import (
	"context"
	"io"
	"log"
	"net"
	"net/http"
	"time"

	"simplefs/internal/auth"
)

type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int64
}

func (s *statusRecorder) WriteHeader(status int) {
	if s.status == 0 {
		s.status = status
	}
	s.ResponseWriter.WriteHeader(status)
}

func (s *statusRecorder) Write(data []byte) (int, error) {
	if s.status == 0 {
		s.status = http.StatusOK
	}
	written, err := s.ResponseWriter.Write(data)
	s.bytes += int64(written)
	return written, err
}

// ReadFrom keeps the sendfile fast path of the underlying writer available to http.ServeContent.
func (s *statusRecorder) ReadFrom(reader io.Reader) (int64, error) {
	if s.status == 0 {
		s.status = http.StatusOK
	}
	var written int64
	var err error
	if readerFrom, ok := s.ResponseWriter.(io.ReaderFrom); ok {
		written, err = readerFrom.ReadFrom(reader)
	} else {
		written, err = io.Copy(s.ResponseWriter, reader)
	}
	s.bytes += written
	return written, err
}

func (s *statusRecorder) Flush() {
	if flusher, ok := s.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (s *statusRecorder) Unwrap() http.ResponseWriter {
	return s.ResponseWriter
}

// AccessLog writes one audit line per request: client, role, method, path, status, bytes and duration.
func AccessLog(logger *log.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		recorder := &statusRecorder{ResponseWriter: w}
		roleHolder := &roleCapture{}
		next.ServeHTTP(recorder, r.WithContext(withRoleCapture(r.Context(), roleHolder)))
		if r.URL.Path == "/healthz" || isStaticPath(r.URL.Path) {
			return
		}
		status := recorder.status
		if status == 0 {
			status = http.StatusOK
		}
		client, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			client = r.RemoteAddr
		}
		logger.Printf("access client=%s role=%s method=%s uri=%q status=%d bytes=%d duration=%s",
			client, roleHolder.role, r.Method, r.URL.RequestURI(), status, recorder.bytes, time.Since(started).Round(time.Millisecond))
	})
}

func isStaticPath(path string) bool {
	return len(path) >= 8 && path[:8] == "/static/"
}

// RecordRole lets the access log know which role the auth middleware assigned.
func RecordRole(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if holder := roleCaptureFrom(r.Context()); holder != nil {
			holder.role = auth.RoleFrom(r.Context()).String()
		}
		next.ServeHTTP(w, r)
	})
}

type roleCapture struct {
	role string
}

type roleCaptureKey struct{}

func withRoleCapture(ctx context.Context, holder *roleCapture) context.Context {
	holder.role = "none"
	return context.WithValue(ctx, roleCaptureKey{}, holder)
}

func roleCaptureFrom(ctx context.Context) *roleCapture {
	holder, _ := ctx.Value(roleCaptureKey{}).(*roleCapture)
	return holder
}
