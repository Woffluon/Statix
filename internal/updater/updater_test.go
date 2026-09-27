package updater

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseSemver(t *testing.T) {
	tests := []struct {
		input       string
		expected    Semver
		expectError bool
	}{
		{"1.0.0", Semver{1, 0, 0, ""}, false},
		{"v1.2.3", Semver{1, 2, 3, ""}, false},
		{"V2.10.5", Semver{2, 10, 5, ""}, false},
		{"v1.0.0-rc1", Semver{1, 0, 0, "rc1"}, false},
		{"v1.2-alpha", Semver{1, 2, 0, "alpha"}, false},
		{"invalid", Semver{}, true},
		{"", Semver{}, true},
		{"1.2.3.4", Semver{}, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			s, err := ParseSemver(tt.input)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, s)
			}
		})
	}
}

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		v1       string
		v2       string
		expected int
	}{
		{"v1.0.0", "v1.0.0", 0},
		{"1.0.0", "v1.0.0", 0},
		{"v1.0.1", "v1.0.0", 1},
		{"v1.0.0", "v1.0.1", -1},
		{"v1.2.0", "v1.1.9", 1},
		{"v2.0.0", "v1.99.99", 1},
		{"v1.0.0", "v1.0.0-rc1", 1},
		{"v1.0.0-rc1", "v1.0.0", -1},
		{"v1.0.0-rc1", "v1.0.0-rc2", -1},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s_vs_%s", tt.v1, tt.v2), func(t *testing.T) {
			res := CompareVersions(tt.v1, tt.v2)
			assert.Equal(t, tt.expected, res)
		})
	}
}

func TestIsNewerVersion(t *testing.T) {
	// "dev" is always eligible for update
	assert.True(t, IsNewerVersion("v1.0.0", "dev"))
	assert.True(t, IsNewerVersion("v0.1.0", "dev"))
	assert.True(t, IsNewerVersion("v1.0.0", "development"))
	assert.True(t, IsNewerVersion("v1.0.0", ""))

	// Standard semver comparisons
	assert.True(t, IsNewerVersion("v1.0.1", "v1.0.0"))
	assert.True(t, IsNewerVersion("v1.1.0", "v1.0.9"))
	assert.True(t, IsNewerVersion("v2.0.0", "v1.9.9"))

	// Equal or older should return false
	assert.False(t, IsNewerVersion("v1.0.0", "v1.0.0"))
	assert.False(t, IsNewerVersion("v1.0.0", "v1.0.1"))
	assert.False(t, IsNewerVersion("v1.0.0", "v2.0.0"))
}

func TestParseChecksum(t *testing.T) {
	knownHash := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

	t.Run("standard sha256sum single line", func(t *testing.T) {
		content := fmt.Sprintf("%s  statix-linux-amd64\n", knownHash)
		hash, err := ParseChecksum([]byte(content), "statix-linux-amd64")
		require.NoError(t, err)
		assert.Equal(t, knownHash, hash)
	})

	t.Run("with binary flag asterisk", func(t *testing.T) {
		content := fmt.Sprintf("%s *statix-linux-arm64\n", knownHash)
		hash, err := ParseChecksum([]byte(content), "statix-linux-arm64")
		require.NoError(t, err)
		assert.Equal(t, knownHash, hash)
	})

	t.Run("multi-line with comments", func(t *testing.T) {
		content := fmt.Sprintf("# SHA256 sums\n1111111111111111111111111111111111111111111111111111111111111111  other-file\n%s  statix-linux-amd64\n", knownHash)
		hash, err := ParseChecksum([]byte(content), "statix-linux-amd64")
		require.NoError(t, err)
		assert.Equal(t, knownHash, hash)
	})

	t.Run("raw hash without filename", func(t *testing.T) {
		content := knownHash + "\n"
		hash, err := ParseChecksum([]byte(content), "statix-linux-amd64")
		require.NoError(t, err)
		assert.Equal(t, knownHash, hash)
	})

	t.Run("invalid hash length", func(t *testing.T) {
		content := "tooshort statix-linux-amd64\n"
		_, err := ParseChecksum([]byte(content), "statix-linux-amd64")
		assert.Error(t, err)
	})

	t.Run("empty content", func(t *testing.T) {
		_, err := ParseChecksum([]byte(""), "statix-linux-amd64")
		assert.Error(t, err)
	})
}

func TestVerifySHA256(t *testing.T) {
	data := []byte("hello statix updater")
	hasher := sha256.New()
	hasher.Write(data)
	expectedHash := hex.EncodeToString(hasher.Sum(nil))

	// Match
	err := VerifySHA256(data, expectedHash)
	assert.NoError(t, err)

	// Uppercase match
	err = VerifySHA256(data, hex.EncodeToString(hasher.Sum(nil)))
	assert.NoError(t, err)

	// Mismatch
	err = VerifySHA256(data, "0000000000000000000000000000000000000000000000000000000000000000")
	assert.ErrorIs(t, err, ErrChecksumMismatch)
}

func TestCheckUpdate_MockGitHub(t *testing.T) {
	published := time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/repos/Woffluon/Statix/releases/latest", r.URL.Path)
		assert.Equal(t, "Statix-Updater", r.Header.Get("User-Agent"))
		assert.Equal(t, "application/vnd.github.v3+json", r.Header.Get("Accept"))

		rel := GitHubRelease{
			TagName:     "v1.5.0",
			Name:        "Statix v1.5.0",
			Body:        "## Changes\n- Performance improvements\n- Self-updater added",
			PublishedAt: published,
			Assets: []GitHubAsset{
				{
					Name:               "statix-linux-amd64",
					BrowserDownloadURL: "http://example.com/download/statix-linux-amd64",
				},
				{
					Name:               "statix-linux-amd64.sha256",
					BrowserDownloadURL: "http://example.com/download/statix-linux-amd64.sha256",
				},
				{
					Name:               "statix-linux-arm64",
					BrowserDownloadURL: "http://example.com/download/statix-linux-arm64",
				},
				{
					Name:               "statix-linux-arm64.sha256",
					BrowserDownloadURL: "http://example.com/download/statix-linux-arm64.sha256",
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(rel)
	}))
	defer server.Close()

	u := &Updater{
		Repo:        DefaultGitHubRepo,
		BaseURL:     server.URL,
		Client:      server.Client(),
		GOOS:        "linux",
		GOARCH:      "amd64",
		RestartFunc: func() {},
	}

	ctx := context.Background()

	t.Run("update available when current is dev", func(t *testing.T) {
		info, err := u.CheckUpdate(ctx, "dev")
		require.NoError(t, err)
		assert.Equal(t, "1.5.0", info.Version)
		assert.Equal(t, "v1.5.0", info.TagName)
		assert.True(t, info.UpdateAvailable)
		assert.Equal(t, "http://example.com/download/statix-linux-amd64", info.AssetURL)
		assert.Equal(t, "http://example.com/download/statix-linux-amd64.sha256", info.ChecksumURL)
		assert.Equal(t, published, info.PublishedAt)
		assert.Contains(t, info.ReleaseNotes, "Self-updater added")
	})

	t.Run("update available when current is older", func(t *testing.T) {
		info, err := u.CheckUpdate(ctx, "v1.4.0")
		require.NoError(t, err)
		assert.True(t, info.UpdateAvailable)
	})

	t.Run("no update when current is same", func(t *testing.T) {
		info, err := u.CheckUpdate(ctx, "v1.5.0")
		require.NoError(t, err)
		assert.False(t, info.UpdateAvailable)
	})

	t.Run("arm64 architecture asset matching", func(t *testing.T) {
		uArm := &Updater{
			Repo:        DefaultGitHubRepo,
			BaseURL:     server.URL,
			Client:      server.Client(),
			GOOS:        "linux",
			GOARCH:      "arm64",
			RestartFunc: func() {},
		}
		info, err := uArm.CheckUpdate(ctx, "v1.0.0")
		require.NoError(t, err)
		assert.Equal(t, "http://example.com/download/statix-linux-arm64", info.AssetURL)
		assert.Equal(t, "http://example.com/download/statix-linux-arm64.sha256", info.ChecksumURL)
	})

	t.Run("missing asset error", func(t *testing.T) {
		uUnknown := &Updater{
			Repo:        DefaultGitHubRepo,
			BaseURL:     server.URL,
			Client:      server.Client(),
			GOOS:        "linux",
			GOARCH:      "mips",
			RestartFunc: func() {},
		}
		_, err := uUnknown.CheckUpdate(ctx, "v1.0.0")
		assert.ErrorIs(t, err, ErrNoMatchingAsset)
	})
}

func TestCheckUpdate_GitHubAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Not Found", http.StatusNotFound)
	}))
	defer server.Close()

	u := &Updater{
		Repo:    DefaultGitHubRepo,
		BaseURL: server.URL,
		Client:  server.Client(),
	}

	_, err := u.CheckUpdate(context.Background(), "dev")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "HTTP 404")
}

func TestApplyUpdate_Success(t *testing.T) {
	// Create dummy binary content
	fakeBinaryContent := []byte("#!/bin/sh\necho updated-statix\n")
	hasher := sha256.New()
	hasher.Write(fakeBinaryContent)
	fakeChecksum := hex.EncodeToString(hasher.Sum(nil))

	// Mock HTTP server hosting the fake binary and sha256 file
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/binary":
			w.Header().Set("Content-Type", "application/octet-stream")
			_, _ = w.Write(fakeBinaryContent)
		case "/checksum":
			w.Header().Set("Content-Type", "text/plain")
			_, _ = fmt.Fprintf(w, "%s  statix-linux-amd64\n", fakeChecksum)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	// Create temporary directory mimicking current executable
	tmpDir := t.TempDir()
	fakeExec := filepath.Join(tmpDir, "statix")
	err := os.WriteFile(fakeExec, []byte("#!/bin/sh\necho old-statix\n"), 0755)
	require.NoError(t, err)

	var restartCalled atomic.Bool
	restartDone := make(chan struct{})

	u := &Updater{
		Repo:    DefaultGitHubRepo,
		BaseURL: server.URL,
		Client:  server.Client(),
		GOOS:    "linux",
		GOARCH:  "amd64",
		ExecutablePathFunc: func() (string, error) {
			return fakeExec, nil
		},
		RestartFunc: func() {
			restartCalled.Store(true)
			close(restartDone)
		},
	}

	info := &ReleaseInfo{
		Version:         "1.5.0",
		TagName:         "v1.5.0",
		AssetURL:        server.URL + "/binary",
		ChecksumURL:     server.URL + "/checksum",
		UpdateAvailable: true,
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	err = u.ApplyUpdate(context.Background(), info, logger)
	require.NoError(t, err)

	// Verify that fakeExec was replaced with new binary content
	newContent, err := os.ReadFile(fakeExec)
	require.NoError(t, err)
	assert.Equal(t, fakeBinaryContent, newContent)

	// Wait for restart callback
	select {
	case <-restartDone:
		assert.True(t, restartCalled.Load())
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for RestartFunc to be called")
	}
}

func TestApplyUpdate_ChecksumMismatch(t *testing.T) {
	fakeBinaryContent := []byte("compromised or bad payload")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/binary":
			_, _ = w.Write(fakeBinaryContent)
		case "/checksum":
			// Return a different expected hash
			_, _ = fmt.Fprintf(w, "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855  statix-linux-amd64\n")
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	fakeExec := filepath.Join(tmpDir, "statix")
	origContent := []byte("original binary")
	err := os.WriteFile(fakeExec, origContent, 0755)
	require.NoError(t, err)

	u := &Updater{
		Repo:    DefaultGitHubRepo,
		BaseURL: server.URL,
		Client:  server.Client(),
		GOOS:    "linux",
		GOARCH:  "amd64",
		ExecutablePathFunc: func() (string, error) {
			return fakeExec, nil
		},
		RestartFunc: func() {},
	}

	info := &ReleaseInfo{
		Version:         "1.5.0",
		TagName:         "v1.5.0",
		AssetURL:        server.URL + "/binary",
		ChecksumURL:     server.URL + "/checksum",
		UpdateAvailable: true,
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	err = u.ApplyUpdate(context.Background(), info, logger)
	assert.ErrorIs(t, err, ErrChecksumMismatch)

	// Ensure the original executable remained untouched!
	content, err := os.ReadFile(fakeExec)
	require.NoError(t, err)
	assert.Equal(t, origContent, content)
}
