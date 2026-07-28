package tlsmanager_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/statix/statix/internal/config"
	"github.com/statix/statix/internal/tlsmanager"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockResolver struct {
	ips []string
	err error
}

func (m *mockResolver) LookupHost(ctx context.Context, host string) ([]string, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.ips, nil
}

func TestValidateDomain(t *testing.T) {
	valid := []string{
		"example.com",
		"sub.domain.co.uk",
		"my-server.org",
		"dev123.test.io",
	}

	for _, d := range valid {
		t.Run("valid_"+d, func(t *testing.T) {
			assert.NoError(t, tlsmanager.ValidateDomain(d))
		})
	}

	invalid := []string{
		"",
		"http://example.com",
		"-invalid.com",
		"invalid-.com",
		"invalid..com",
		"bad_domain.com",
	}

	for _, d := range invalid {
		t.Run("invalid_"+d, func(t *testing.T) {
			assert.Error(t, tlsmanager.ValidateDomain(d))
		})
	}
}

func TestDNSPreCheck(t *testing.T) {
	cfg := config.DefaultConfig()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	mgr := tlsmanager.New(cfg, "config.yaml", logger)

	ctx := context.Background()

	// Case 1: Match
	resSuccess := &mockResolver{ips: []string{"1.2.3.4"}}
	ok, ips, err := mgr.CheckDNS(ctx, "example.com", resSuccess)
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, []string{"1.2.3.4"}, ips)

	// Case 2: Error
	resFail := &mockResolver{err: errors.New("DNS lookup failure")}
	_, _, err = mgr.CheckDNS(ctx, "example.com", resFail)
	require.Error(t, err)
}
