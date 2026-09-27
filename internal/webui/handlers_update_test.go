package webui_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/statix/statix/internal/auth"
	"github.com/statix/statix/internal/config"
	"github.com/statix/statix/internal/metrics"
	"github.com/statix/statix/internal/updater"
	"github.com/statix/statix/internal/webui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockUpdateManager struct {
	checkFunc func(ctx context.Context, currentVersion string) (*updater.ReleaseInfo, error)
	applyFunc func(ctx context.Context, info *updater.ReleaseInfo, logger *slog.Logger) error
}

func (m *mockUpdateManager) CheckUpdate(ctx context.Context, currentVersion string) (*updater.ReleaseInfo, error) {
	if m.checkFunc != nil {
		return m.checkFunc(ctx, currentVersion)
	}
	return &updater.ReleaseInfo{
		Version:         "1.5.0",
		TagName:         "v1.5.0",
		ReleaseNotes:    "Changelog notes",
		PublishedAt:     time.Date(2026, 9, 27, 0, 0, 0, 0, time.UTC),
		AssetURL:        "https://example.com/statix-linux-amd64",
		ChecksumURL:     "https://example.com/statix-linux-amd64.sha256",
		UpdateAvailable: true,
	}, nil
}

func (m *mockUpdateManager) ApplyUpdate(ctx context.Context, info *updater.ReleaseInfo, logger *slog.Logger) error {
	if m.applyFunc != nil {
		return m.applyFunc(ctx, info, logger)
	}
	return nil
}

func setupUpdateTestServer(t *testing.T, mgr webui.UpdateManager) (*webui.Server, *auth.SessionStore, string) {
	t.Helper()
	cfg := config.DefaultConfig()
	cfg.SetupComplete = true
	cfg.AdminUsername = "admin"
	pwHash, err := auth.HashPassword("TestPass123!")
	require.NoError(t, err)
	cfg.AdminPasswordHash = pwHash

	store := auth.NewSessionStore(24 * time.Hour)
	rl := auth.NewRateLimiter(5, 10*time.Minute, 5*time.Minute)
	buf := metrics.NewRingBuffer(10)
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))

	server, err := webui.New(webui.ServerDeps{
		Config:    cfg,
		Store:     store,
		RateLimit: rl,
		Buffer:    buf,
		Logger:    logger,
		Version:   "1.0.0",
		Updater:   mgr,
	})
	require.NoError(t, err)

	sess, err := store.Create("admin")
	require.NoError(t, err)

	return server, store, sess.ID
}

func TestUpdateCheckEndpoint(t *testing.T) {
	t.Run("UnauthenticatedRedirectsToLogin", func(t *testing.T) {
		server, _, _ := setupUpdateTestServer(t, &mockUpdateManager{})

		req := httptest.NewRequest(http.MethodGet, "/api/update/check", nil)
		w := httptest.NewRecorder()

		server.ServeHTTP(w, req)
		assert.Equal(t, http.StatusFound, w.Code)
		assert.Equal(t, "/login", w.Header().Get("Location"))
	})

	t.Run("AuthenticatedReturnsReleaseInfo", func(t *testing.T) {
		server, _, sessID := setupUpdateTestServer(t, &mockUpdateManager{})

		req := httptest.NewRequest(http.MethodGet, "/api/update/check", nil)
		req.AddCookie(&http.Cookie{Name: "statix_session", Value: sessID})
		w := httptest.NewRecorder()

		server.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

		var resp map[string]any
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)

		assert.Equal(t, "1.0.0", resp["current_version"])
		assert.Equal(t, true, resp["update_available"])
		assert.Equal(t, "1.5.0", resp["latest_version"])
		assert.Equal(t, "v1.5.0", resp["tag_name"])
		assert.Equal(t, "Changelog notes", resp["release_notes"])
		assert.Equal(t, "https://example.com/statix-linux-amd64", resp["asset_url"])
		assert.Equal(t, "https://example.com/statix-linux-amd64.sha256", resp["checksum_url"])
	})

	t.Run("UpstreamCheckErrorReturns502", func(t *testing.T) {
		mockMgr := &mockUpdateManager{
			checkFunc: func(ctx context.Context, currentVersion string) (*updater.ReleaseInfo, error) {
				return nil, errors.New("github connection timeout")
			},
		}
		server, _, sessID := setupUpdateTestServer(t, mockMgr)

		req := httptest.NewRequest(http.MethodGet, "/api/update/check", nil)
		req.AddCookie(&http.Cookie{Name: "statix_session", Value: sessID})
		w := httptest.NewRecorder()

		server.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadGateway, w.Code)

		var resp map[string]string
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Contains(t, resp["error"], "github connection timeout")
	})
}

func TestUpdateApplyEndpoint(t *testing.T) {
	csrfToken, err := auth.GenerateCSRFToken()
	require.NoError(t, err)

	t.Run("UnauthenticatedRedirectsToLogin", func(t *testing.T) {
		server, _, _ := setupUpdateTestServer(t, &mockUpdateManager{})

		req := httptest.NewRequest(http.MethodPost, "/api/update/apply", nil)
		w := httptest.NewRecorder()

		server.ServeHTTP(w, req)
		assert.Equal(t, http.StatusFound, w.Code)
		assert.Equal(t, "/login", w.Header().Get("Location"))
	})

	t.Run("AuthenticatedMissingCSRFRejected", func(t *testing.T) {
		server, _, sessID := setupUpdateTestServer(t, &mockUpdateManager{})

		req := httptest.NewRequest(http.MethodPost, "/api/update/apply", nil)
		req.AddCookie(&http.Cookie{Name: "statix_session", Value: sessID})
		// Missing statix_csrf cookie and X-CSRF-Token header
		w := httptest.NewRecorder()

		server.ServeHTTP(w, req)
		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Contains(t, w.Body.String(), "CSRF")
	})

	t.Run("AuthenticatedWithCSRFHeaderSuccess", func(t *testing.T) {
		appliedCalled := false
		mockMgr := &mockUpdateManager{
			applyFunc: func(ctx context.Context, info *updater.ReleaseInfo, logger *slog.Logger) error {
				appliedCalled = true
				return nil
			},
		}
		server, _, sessID := setupUpdateTestServer(t, mockMgr)

		req := httptest.NewRequest(http.MethodPost, "/api/update/apply", nil)
		req.AddCookie(&http.Cookie{Name: "statix_session", Value: sessID})
		req.AddCookie(&http.Cookie{Name: "statix_csrf", Value: csrfToken})
		req.Header.Set("X-CSRF-Token", csrfToken)

		w := httptest.NewRecorder()
		server.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.True(t, appliedCalled)

		var resp map[string]string
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, "success", resp["status"])
		assert.Equal(t, "1.5.0", resp["version"])
	})

	t.Run("NoUpdateAvailableReturnsBadRequest", func(t *testing.T) {
		mockMgr := &mockUpdateManager{
			checkFunc: func(ctx context.Context, currentVersion string) (*updater.ReleaseInfo, error) {
				return &updater.ReleaseInfo{
					Version:         "1.0.0",
					UpdateAvailable: false,
				}, nil
			},
		}
		server, _, sessID := setupUpdateTestServer(t, mockMgr)

		req := httptest.NewRequest(http.MethodPost, "/api/update/apply", nil)
		req.AddCookie(&http.Cookie{Name: "statix_session", Value: sessID})
		req.AddCookie(&http.Cookie{Name: "statix_csrf", Value: csrfToken})
		req.Header.Set("X-CSRF-Token", csrfToken)

		w := httptest.NewRecorder()
		server.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "already running the latest version")
	})

	t.Run("ApplyFailureReturnsServerError", func(t *testing.T) {
		mockMgr := &mockUpdateManager{
			applyFunc: func(ctx context.Context, info *updater.ReleaseInfo, logger *slog.Logger) error {
				return errors.New("disk write failure")
			},
		}
		server, _, sessID := setupUpdateTestServer(t, mockMgr)

		req := httptest.NewRequest(http.MethodPost, "/api/update/apply", nil)
		req.AddCookie(&http.Cookie{Name: "statix_session", Value: sessID})
		req.AddCookie(&http.Cookie{Name: "statix_csrf", Value: csrfToken})
		req.Header.Set("X-CSRF-Token", csrfToken)

		w := httptest.NewRecorder()
		server.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		assert.Contains(t, w.Body.String(), "disk write failure")
	})

	t.Run("ChecksumMismatchReturnsBadRequest", func(t *testing.T) {
		mockMgr := &mockUpdateManager{
			applyFunc: func(ctx context.Context, info *updater.ReleaseInfo, logger *slog.Logger) error {
				return updater.ErrChecksumMismatch
			},
		}
		server, _, sessID := setupUpdateTestServer(t, mockMgr)

		req := httptest.NewRequest(http.MethodPost, "/api/update/apply", nil)
		req.AddCookie(&http.Cookie{Name: "statix_session", Value: sessID})
		req.AddCookie(&http.Cookie{Name: "statix_csrf", Value: csrfToken})
		req.Header.Set("X-CSRF-Token", csrfToken)

		w := httptest.NewRecorder()
		server.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "SHA-256 checksum mismatch")
	})

	t.Run("ConcurrentApplyReturnsConflict", func(t *testing.T) {
		started := make(chan struct{})
		blocker := make(chan struct{})

		mockMgr := &mockUpdateManager{
			applyFunc: func(ctx context.Context, info *updater.ReleaseInfo, logger *slog.Logger) error {
				close(started)
				<-blocker
				return nil
			},
		}
		server, _, sessID := setupUpdateTestServer(t, mockMgr)

		var wg sync.WaitGroup
		wg.Add(1)

		var firstCode int
		go func() {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodPost, "/api/update/apply", nil)
			req.AddCookie(&http.Cookie{Name: "statix_session", Value: sessID})
			req.AddCookie(&http.Cookie{Name: "statix_csrf", Value: csrfToken})
			req.Header.Set("X-CSRF-Token", csrfToken)
			w := httptest.NewRecorder()
			server.ServeHTTP(w, req)
			firstCode = w.Code
		}()

		// Wait until first update handler is inside ApplyUpdate
		<-started

		// Second concurrent request must receive 409 Conflict
		req2 := httptest.NewRequest(http.MethodPost, "/api/update/apply", nil)
		req2.AddCookie(&http.Cookie{Name: "statix_session", Value: sessID})
		req2.AddCookie(&http.Cookie{Name: "statix_csrf", Value: csrfToken})
		req2.Header.Set("X-CSRF-Token", csrfToken)
		w2 := httptest.NewRecorder()
		server.ServeHTTP(w2, req2)

		assert.Equal(t, http.StatusConflict, w2.Code)
		assert.Contains(t, w2.Body.String(), "already in progress")

		// Release first update
		close(blocker)
		wg.Wait()

		assert.Equal(t, http.StatusOK, firstCode)
	})
}
