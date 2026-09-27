package webui_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/statix/statix/internal/auth"
	"github.com/statix/statix/internal/config"
	"github.com/statix/statix/internal/metrics"
	"github.com/statix/statix/internal/webui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestServer(t *testing.T, setupComplete bool) (*webui.Server, *config.Config, *auth.SessionStore, *auth.RateLimiter) {
	t.Helper()
	cfg := config.DefaultConfig()
	cfg.SetupComplete = setupComplete
	cfg.AdminUsername = "admin"
	pwHash, err := auth.HashPassword("CorrectPassword123!")
	require.NoError(t, err)
	cfg.AdminPasswordHash = pwHash

	store := auth.NewSessionStore(24 * time.Hour)
	rl := auth.NewRateLimiter(3, 10*time.Minute, 5*time.Minute)
	buf := metrics.NewRingBuffer(10)
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))

	server, err := webui.New(webui.ServerDeps{
		Config:    cfg,
		Store:     store,
		RateLimit: rl,
		Buffer:    buf,
		Logger:    logger,
	})
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	server.Start(ctx)

	return server, cfg, store, rl
}

func TestLoginGet(t *testing.T) {
	server, _, store, _ := setupTestServer(t, true)

	t.Run("UnauthenticatedRendersLoginPageAndSetsCSRF", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/login", nil)
		w := httptest.NewRecorder()

		server.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "Sign In to Statix")

		var csrfCookie *http.Cookie
		for _, c := range w.Result().Cookies() {
			if c.Name == "statix_csrf" {
				csrfCookie = c
				break
			}
		}
		require.NotNil(t, csrfCookie, "expected statix_csrf cookie to be set")
		assert.NotEmpty(t, csrfCookie.Value)
	})

	t.Run("AuthenticatedRedirectsToDashboard", func(t *testing.T) {
		sess, err := store.Create("admin")
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodGet, "/login", nil)
		req.AddCookie(&http.Cookie{Name: "statix_session", Value: sess.ID})
		w := httptest.NewRecorder()

		server.ServeHTTP(w, req)

		assert.Equal(t, http.StatusFound, w.Code)
		assert.Equal(t, "/dashboard", w.Header().Get("Location"))
	})
}

func TestLoginPost(t *testing.T) {
	server, _, _, _ := setupTestServer(t, true)

	csrfToken, err := auth.GenerateCSRFToken()
	require.NoError(t, err)

	t.Run("CSRFRejectionWithoutCookie", func(t *testing.T) {
		form := url.Values{
			"username":   {"admin"},
			"password":   {"CorrectPassword123!"},
			"csrf_token": {csrfToken},
		}
		req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		// No cookie
		w := httptest.NewRecorder()
		server.ServeHTTP(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("InvalidCredentialsFails", func(t *testing.T) {
		form := url.Values{
			"username":   {"admin"},
			"password":   {"WrongPassword123!"},
			"csrf_token": {csrfToken},
		}
		req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.AddCookie(&http.Cookie{Name: "statix_csrf", Value: csrfToken})

		w := httptest.NewRecorder()
		server.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "Invalid username or password.")
	})

	t.Run("RateLimitLockoutAndRetryAfter", func(t *testing.T) {
		// Attempt 3 failures to trigger lockout (limit is 3)
		for i := 0; i < 2; i++ {
			form := url.Values{
				"username":   {"lockout_user"},
				"password":   {"WrongPassword"},
				"csrf_token": {csrfToken},
			}
			req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			req.AddCookie(&http.Cookie{Name: "statix_csrf", Value: csrfToken})
			req.RemoteAddr = "192.168.1.100:12345"

			w := httptest.NewRecorder()
			server.ServeHTTP(w, req)
			assert.Contains(t, w.Body.String(), "Invalid username or password.")
		}

		// 3rd failure locks it out
		form := url.Values{
			"username":   {"lockout_user"},
			"password":   {"WrongPassword"},
			"csrf_token": {csrfToken},
		}
		req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.AddCookie(&http.Cookie{Name: "statix_csrf", Value: csrfToken})
		req.RemoteAddr = "192.168.1.100:12345"
		w := httptest.NewRecorder()
		server.ServeHTTP(w, req)

		// 4th request while locked out must return lockout error and Retry-After header
		reqLocked := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
		reqLocked.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		reqLocked.AddCookie(&http.Cookie{Name: "statix_csrf", Value: csrfToken})
		reqLocked.RemoteAddr = "192.168.1.100:12345"
		wLocked := httptest.NewRecorder()
		server.ServeHTTP(wLocked, reqLocked)

		assert.Contains(t, wLocked.Body.String(), "Too many failed login attempts")
		assert.NotEmpty(t, wLocked.Header().Get("Retry-After"))
	})

	t.Run("HappyPathSuccessfulLogin", func(t *testing.T) {
		form := url.Values{
			"username":   {"admin"},
			"password":   {"CorrectPassword123!"},
			"csrf_token": {csrfToken},
		}
		req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.AddCookie(&http.Cookie{Name: "statix_csrf", Value: csrfToken})

		w := httptest.NewRecorder()
		server.ServeHTTP(w, req)

		assert.Equal(t, http.StatusFound, w.Code)
		assert.Equal(t, "/dashboard", w.Header().Get("Location"))

		var sessCookie *http.Cookie
		for _, c := range w.Result().Cookies() {
			if c.Name == "statix_session" {
				sessCookie = c
				break
			}
		}
		require.NotNil(t, sessCookie)
		assert.NotEmpty(t, sessCookie.Value)
		assert.True(t, sessCookie.HttpOnly)
	})
}

func TestLogoutPost(t *testing.T) {
	server, _, store, _ := setupTestServer(t, true)

	sess, err := store.Create("admin")
	require.NoError(t, err)

	csrfToken, err := auth.GenerateCSRFToken()
	require.NoError(t, err)

	t.Run("CSRFRejectionOnLogout", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/logout", nil)
		// Authenticated request but missing CSRF cookie and token
		req.AddCookie(&http.Cookie{Name: "statix_session", Value: sess.ID})
		w := httptest.NewRecorder()
		server.ServeHTTP(w, req)
		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("SuccessfulLogoutClearsSessionAndCookie", func(t *testing.T) {
		form := url.Values{"csrf_token": {csrfToken}}
		req := httptest.NewRequest(http.MethodPost, "/logout", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.AddCookie(&http.Cookie{Name: "statix_csrf", Value: csrfToken})
		req.AddCookie(&http.Cookie{Name: "statix_session", Value: sess.ID})

		w := httptest.NewRecorder()
		server.ServeHTTP(w, req)

		assert.Equal(t, http.StatusFound, w.Code)
		assert.Equal(t, "/login", w.Header().Get("Location"))

		// Verify session deleted in store
		_, valid := store.Get(sess.ID)
		assert.False(t, valid, "session should be deleted after logout")

		// Verify cookie expired in response
		var clearedCookie *http.Cookie
		for _, c := range w.Result().Cookies() {
			if c.Name == "statix_session" {
				clearedCookie = c
				break
			}
		}
		require.NotNil(t, clearedCookie)
		assert.Equal(t, -1, clearedCookie.MaxAge)
	})
}
