package webui

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/statix/statix/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetupTemplateRendering(t *testing.T) {
	cfg := config.DefaultConfig()
	server, err := New(ServerDeps{
		Config: cfg,
	})
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/setup", nil)
	w := httptest.NewRecorder()

	server.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, "<!DOCTYPE html>")
	assert.Contains(t, body, "Initial Setup Wizard")
	assert.Contains(t, body, "Admin Username")
}
