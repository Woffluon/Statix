package webui_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/statix/statix/internal/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCSRFTokenGeneration(t *testing.T) {
	token, err := auth.GenerateCSRFToken()
	require.NoError(t, err)
	assert.Len(t, token, 64)

	// Ensure randomness
	token2, err := auth.GenerateCSRFToken()
	require.NoError(t, err)
	assert.NotEqual(t, token, token2)
}

func TestCSRFMiddlewareDoubleSubmit(t *testing.T) {
	dummyHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	protected := auth.CSRF(dummyHandler)

	validToken, err := auth.GenerateCSRFToken()
	require.NoError(t, err)

	t.Run("SafeMethodsPassWithoutCSRF", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/any", nil)
		w := httptest.NewRecorder()
		protected.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("ValidCookieAndMatchingFormField", func(t *testing.T) {
		form := url.Values{}
		form.Set("csrf_token", validToken)
		req := httptest.NewRequest(http.MethodPost, "/action", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.AddCookie(&http.Cookie{Name: "statix_csrf", Value: validToken})

		w := httptest.NewRecorder()
		protected.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "ok", w.Body.String())
	})

	t.Run("ValidCookieAndMatchingHeader", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/action", nil)
		req.Header.Set("X-CSRF-Token", validToken)
		req.AddCookie(&http.Cookie{Name: "statix_csrf", Value: validToken})

		w := httptest.NewRecorder()
		protected.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "ok", w.Body.String())
	})

	t.Run("MissingCookieRejection", func(t *testing.T) {
		form := url.Values{}
		form.Set("csrf_token", validToken)
		req := httptest.NewRequest(http.MethodPost, "/action", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		// No cookie set

		w := httptest.NewRecorder()
		protected.ServeHTTP(w, req)
		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Contains(t, w.Body.String(), "CSRF token missing or invalid")
	})

	t.Run("MissingTokenRejection", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/action", nil)
		req.AddCookie(&http.Cookie{Name: "statix_csrf", Value: validToken})
		// No form field or header

		w := httptest.NewRecorder()
		protected.ServeHTTP(w, req)
		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Contains(t, w.Body.String(), "CSRF token mismatch")
	})

	t.Run("MismatchedTokenRejection", func(t *testing.T) {
		diffToken, _ := auth.GenerateCSRFToken()
		form := url.Values{}
		form.Set("csrf_token", diffToken)
		req := httptest.NewRequest(http.MethodPost, "/action", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.AddCookie(&http.Cookie{Name: "statix_csrf", Value: validToken})

		w := httptest.NewRecorder()
		protected.ServeHTTP(w, req)
		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Contains(t, w.Body.String(), "CSRF token mismatch")
	})
}
