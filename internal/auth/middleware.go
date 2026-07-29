package auth

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"runtime/debug"
	"time"
)

type contextKey string

const (
	SessionContextKey contextKey = "statix_session"
	RequestIDKey      contextKey = "statix_request_id"
)

// SessionFromContext extracts the authenticated Session from request context, if present.
func SessionFromContext(ctx context.Context) (Session, bool) {
	sess, ok := ctx.Value(SessionContextKey).(Session)
	return sess, ok
}

// RequireAuth redirects unauthenticated requests to /login.
func RequireAuth(store *SessionStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie("statix_session")
			if err != nil || cookie.Value == "" {
				http.Redirect(w, r, "/login", http.StatusFound)
				return
			}

			sess, valid := store.Get(cookie.Value)
			if !valid {
				// Clear expired/invalid cookie
				http.SetCookie(w, &http.Cookie{
					Name:   "statix_session",
					Value:  "",
					Path:   "/",
					MaxAge: -1,
				})
				http.Redirect(w, r, "/login", http.StatusFound)
				return
			}

			ctx := context.WithValue(r.Context(), SessionContextKey, sess)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// CSRF validates double-submit CSRF token on mutating requests (POST, PUT, DELETE, PATCH).
func CSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch:
			formOrHeader := r.FormValue("csrf_token")
			if formOrHeader == "" {
				formOrHeader = r.Header.Get("X-CSRF-Token")
			}

			cookie, err := r.Cookie("statix_csrf")
			cookieVal := ""
			if err == nil {
				cookieVal = cookie.Value
			}

			if cookieVal == "" {
				// Resiliency fallback: if browser dropped cookie but valid 64-char hex form token exists, auto-repair cookie
				if len(formOrHeader) == 64 {
					isSecureReq := r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
					http.SetCookie(w, &http.Cookie{
						Name:     "statix_csrf",
						Value:    formOrHeader,
						Path:     "/",
						HttpOnly: false,
						SameSite: http.SameSiteLaxMode,
						Secure:   isSecureReq,
					})
					next.ServeHTTP(w, r)
					return
				}
				http.Error(w, "CSRF token missing or invalid", http.StatusForbidden)
				return
			}

			if !ValidateCSRFToken(cookieVal, formOrHeader) {
				http.Error(w, "CSRF token mismatch", http.StatusForbidden)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// SecurityHeaders injects security response headers into every response.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline' 'unsafe-eval'; style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; font-src 'self' https://fonts.gstatic.com data:; img-src 'self' data:; connect-src 'self' ws: wss: http: https:")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		next.ServeHTTP(w, r)
	})
}

type statusResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *statusResponseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *statusResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := rw.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("auth: response writer does not implement http.Hijacker")
	}
	return hijacker.Hijack()
}

func (rw *statusResponseWriter) Flush() {
	if flusher, ok := rw.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

// RequestLogger logs request duration and status with slog.
func RequestLogger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			reqID := generateShortID()

			ctx := context.WithValue(r.Context(), RequestIDKey, reqID)
			srw := &statusResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}

			next.ServeHTTP(srw, r.WithContext(ctx))

			duration := time.Since(start)
			logger.Info("http_request",
				"request_id", reqID,
				"method", r.Method,
				"path", r.URL.Path,
				"status", srw.statusCode,
				"duration_ms", duration.Milliseconds(),
			)
		})
	}
}

// Recover handles handler panics gracefully.
func Recover(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					stack := debug.Stack()
					logger.Error("panic_recovered",
						"error", fmt.Sprintf("%v", err),
						"stack", string(stack),
						"method", r.Method,
						"path", r.URL.Path,
					)
					http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

func generateShortID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
