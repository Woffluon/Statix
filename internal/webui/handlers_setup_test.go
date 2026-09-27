package webui_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
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

func setupTestServerForWizard(t *testing.T, setupComplete bool) (*webui.Server, *config.Config, string) {
	t.Helper()
	tempDir := t.TempDir()
	cfgPath := filepath.Join(tempDir, "statix.json")

	cfg := config.DefaultConfig()
	cfg.SetupComplete = setupComplete
	require.NoError(t, config.Save(cfgPath, cfg))

	store := auth.NewSessionStore(24 * time.Hour)
	rl := auth.NewRateLimiter(5, 10*time.Minute, 5*time.Minute)
	buf := metrics.NewRingBuffer(10)
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))

	server, err := webui.New(webui.ServerDeps{
		Config:     cfg,
		ConfigPath: cfgPath,
		Store:      store,
		RateLimit:  rl,
		Buffer:     buf,
		Logger:     logger,
	})
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	server.Start(ctx)

	return server, cfg, cfgPath
}

func TestSetupGet(t *testing.T) {
	t.Run("WizardAvailableWhenIncomplete", func(t *testing.T) {
		server, _, _ := setupTestServerForWizard(t, false)

		req := httptest.NewRequest(http.MethodGet, "/setup", nil)
		w := httptest.NewRecorder()

		server.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "Initial Setup Wizard")

		var csrfCookie *http.Cookie
		for _, c := range w.Result().Cookies() {
			if c.Name == "statix_csrf" {
				csrfCookie = c
				break
			}
		}
		require.NotNil(t, csrfCookie)
		assert.NotEmpty(t, csrfCookie.Value)
	})

	t.Run("WizardForbiddenWhenAlreadyCompleted", func(t *testing.T) {
		server, _, _ := setupTestServerForWizard(t, true)

		req := httptest.NewRequest(http.MethodGet, "/setup", nil)
		w := httptest.NewRecorder()

		server.ServeHTTP(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Contains(t, w.Body.String(), "Setup wizard has already been completed.")
	})
}

func TestSetupPostValidation(t *testing.T) {
	server, _, _ := setupTestServerForWizard(t, false)

	csrfToken, err := auth.GenerateCSRFToken()
	require.NoError(t, err)

	postWizard := func(form url.Values) *httptest.ResponseRecorder {
		form.Set("csrf_token", csrfToken)
		req := httptest.NewRequest(http.MethodPost, "/setup", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.AddCookie(&http.Cookie{Name: "statix_csrf", Value: csrfToken})

		w := httptest.NewRecorder()
		server.ServeHTTP(w, req)
		return w
	}

	t.Run("MissingUsername", func(t *testing.T) {
		w := postWizard(url.Values{
			"username":         {""},
			"password":         {"ValidPassword123!"},
			"confirm_password": {"ValidPassword123!"},
			"listen_addr":      {":8080"},
		})
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "Username is required.")
	})

	t.Run("ShortPassword", func(t *testing.T) {
		w := postWizard(url.Values{
			"username":         {"admin"},
			"password":         {"short"},
			"confirm_password": {"short"},
			"listen_addr":      {":8080"},
		})
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "Password must be at least 8 characters long.")
	})

	t.Run("PasswordMismatch", func(t *testing.T) {
		w := postWizard(url.Values{
			"username":         {"admin"},
			"password":         {"ValidPassword123!"},
			"confirm_password": {"MismatchPassword123!"},
			"listen_addr":      {":8080"},
		})
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "Passwords do not match.")
	})

	t.Run("InvalidListenAddressFormat", func(t *testing.T) {
		w := postWizard(url.Values{
			"username":         {"admin"},
			"password":         {"ValidPassword123!"},
			"confirm_password": {"ValidPassword123!"},
			"listen_addr":      {"not_a_valid_port"},
		})
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "Invalid listen address")
	})

	t.Run("InvalidListenPortRange", func(t *testing.T) {
		w := postWizard(url.Values{
			"username":         {"admin"},
			"password":         {"ValidPassword123!"},
			"confirm_password": {"ValidPassword123!"},
			"listen_addr":      {":99999"},
		})
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "Invalid listen address")
	})
}

func TestSetupPostHappyPathAndGuard(t *testing.T) {
	server, cfg, cfgPath := setupTestServerForWizard(t, false)

	csrfToken, err := auth.GenerateCSRFToken()
	require.NoError(t, err)

	form := url.Values{
		"username":         {"superadmin"},
		"password":         {"StrongPassword123!"},
		"confirm_password": {"StrongPassword123!"},
		"listen_addr":      {":9090"},
		"csrf_token":       {csrfToken},
	}
	req := httptest.NewRequest(http.MethodPost, "/setup", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "statix_csrf", Value: csrfToken})

	w := httptest.NewRecorder()
	server.ServeHTTP(w, req)

	assert.Equal(t, http.StatusFound, w.Code)
	assert.Equal(t, "/login", w.Header().Get("Location"))

	// Verify in-memory config updated
	assert.True(t, cfg.SetupComplete)
	assert.Equal(t, "superadmin", cfg.AdminUsername)
	assert.Equal(t, ":9090", cfg.ListenAddr)
	assert.NotEmpty(t, cfg.AdminPasswordHash)
	assert.NotEmpty(t, cfg.SessionSecret)

	// Verify config persisted to disk
	savedCfg, err := config.Load(cfgPath)
	require.NoError(t, err)
	assert.True(t, savedCfg.SetupComplete)
	assert.Equal(t, "superadmin", savedCfg.AdminUsername)
	assert.Equal(t, ":9090", savedCfg.ListenAddr)

	// Verify Setup Guard prevents second execution
	reqAgain := httptest.NewRequest(http.MethodPost, "/setup", strings.NewReader(form.Encode()))
	reqAgain.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	reqAgain.AddCookie(&http.Cookie{Name: "statix_csrf", Value: csrfToken})
	wAgain := httptest.NewRecorder()
	server.ServeHTTP(wAgain, reqAgain)

	assert.Equal(t, http.StatusForbidden, wAgain.Code)
	assert.Contains(t, wAgain.Body.String(), "Setup wizard has already been completed.")

	_ = os.Remove(cfgPath)
}
