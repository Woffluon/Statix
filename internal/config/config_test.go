package config_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/statix/statix/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigLoadAndSaveRoundTrip(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")

	original := &config.Config{
		ListenAddr:             ":9090",
		AdminUsername:          "admin",
		AdminPasswordHash:      "$argon2id$v=19$m=65536,t=3,p=2$hashhashhash",
		SessionSecret:          "0123456789abcdef0123456789abcdef",
		SetupComplete:          true,
		CollectIntervalSeconds: 5,
		HistoryDurationHours:   12,
		LogFormat:              "text",
	}

	err := config.Save(configPath, original)
	require.NoError(t, err)

	info, err := os.Stat(configPath)
	require.NoError(t, err)
	if runtime.GOOS != "windows" {
		assert.Equal(t, os.FileMode(0600), info.Mode().Perm())
	}

	loaded, err := config.Load(configPath)
	require.NoError(t, err)
	assert.Equal(t, original, loaded)
}

func TestConfigLoadMissingFile(t *testing.T) {
	_, err := config.Load("/path/that/does/not/exist.yaml")
	require.Error(t, err)
}

func TestConfigLoadInvalidYAML(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "invalid.yaml")
	err := os.WriteFile(configPath, []byte("listen_addr: [invalid yaml"), 0600)
	require.NoError(t, err)

	_, err = config.Load(configPath)
	require.Error(t, err)
}

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		modify  func(cfg *config.Config)
		wantErr bool
	}{
		{
			name:    "valid config",
			modify:  func(cfg *config.Config) {},
			wantErr: false,
		},
		{
			name:    "empty listen address",
			modify:  func(cfg *config.Config) { cfg.ListenAddr = "" },
			wantErr: true,
		},
		{
			name:    "zero interval",
			modify:  func(cfg *config.Config) { cfg.CollectIntervalSeconds = 0 },
			wantErr: true,
		},
		{
			name:    "zero history duration",
			modify:  func(cfg *config.Config) { cfg.HistoryDurationHours = 0 },
			wantErr: true,
		},
		{
			name:    "invalid log format",
			modify:  func(cfg *config.Config) { cfg.LogFormat = "xml" },
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := config.DefaultConfig()
			tc.modify(cfg)
			err := cfg.Validate()
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestIsPortFallbackEnabled(t *testing.T) {
	tr := true
	fl := false

	cfgDefault := config.DefaultConfig()
	assert.True(t, cfgDefault.IsPortFallbackEnabled())

	cfgCustomPort := &config.Config{ListenAddr: ":9090"}
	assert.False(t, cfgCustomPort.IsPortFallbackEnabled())

	cfgCustomPortWithFallback := &config.Config{ListenAddr: ":9090", PortFallback: &tr}
	assert.True(t, cfgCustomPortWithFallback.IsPortFallbackEnabled())

	cfgDefaultWithDisabledFallback := &config.Config{ListenAddr: ":8080", PortFallback: &fl}
	assert.False(t, cfgDefaultWithDisabledFallback.IsPortFallbackEnabled())
}
