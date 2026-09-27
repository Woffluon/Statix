package webui_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/statix/statix/internal/auth"
	"github.com/statix/statix/internal/metrics"
	"github.com/statix/statix/internal/webui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWSHubCSWSHOriginRejection(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	store := auth.NewSessionStore(time.Hour)
	sess, err := store.Create("admin")
	require.NoError(t, err)

	hub := webui.NewWSHub(logger)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hub.UpgradeAndServe(w, r, store)
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	t.Run("UnauthorizedWithoutSessionCookie", func(t *testing.T) {
		dialCtx, dialCancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer dialCancel()

		_, resp, err := websocket.Dial(dialCtx, wsURL, nil)
		require.Error(t, err)
		if resp != nil {
			assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
		}
	})

	t.Run("RejectedCrossSiteOriginHijack", func(t *testing.T) {
		dialCtx, dialCancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer dialCancel()

		header := http.Header{}
		header.Set("Origin", "https://malicious-attacker.com")
		header.Set("Cookie", (&http.Cookie{Name: "statix_session", Value: sess.ID}).String())

		_, resp, err := websocket.Dial(dialCtx, wsURL, &websocket.DialOptions{
			HTTPHeader: header,
		})
		require.Error(t, err, "CSWSH request must be rejected")
		if resp != nil {
			assert.Equal(t, http.StatusForbidden, resp.StatusCode)
		}
	})

	t.Run("AcceptedSameOrigin", func(t *testing.T) {
		dialCtx, dialCancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer dialCancel()

		header := http.Header{}
		header.Set("Origin", server.URL)
		header.Set("Cookie", (&http.Cookie{Name: "statix_session", Value: sess.ID}).String())

		conn, resp, err := websocket.Dial(dialCtx, wsURL, &websocket.DialOptions{
			HTTPHeader: header,
		})
		require.NoError(t, err)
		require.NotNil(t, conn)
		assert.Equal(t, http.StatusSwitchingProtocols, resp.StatusCode)
		_ = conn.CloseNow()
	})
}

func TestWSHubBroadcastAndSlowClient(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	store := auth.NewSessionStore(time.Hour)
	sess, err := store.Create("admin")
	require.NoError(t, err)

	hub := webui.NewWSHub(logger)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hub.UpgradeAndServe(w, r, store)
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	dialCtx, dialCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer dialCancel()

	header := http.Header{}
	header.Set("Origin", server.URL)
	header.Set("Cookie", (&http.Cookie{Name: "statix_session", Value: sess.ID}).String())

	conn, _, err := websocket.Dial(dialCtx, wsURL, &websocket.DialOptions{
		HTTPHeader: header,
	})
	require.NoError(t, err)
	defer func() { _ = conn.CloseNow() }()

	// Publish multiple snapshots rapidly to test ring-buffer drop behavior on client queue
	for i := 0; i < 70; i++ {
		hub.Publish(metrics.Snapshot{
			MemTotal: uint64(1000 + i),
		})
	}

	// Client should be able to read without blocking
	readCtx, readCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer readCancel()

	var s metrics.Snapshot
	_, data, err := conn.Read(readCtx)
	require.NoError(t, err)
	assert.NotEmpty(t, data)
	_ = s
}

func TestWSHubShutdownLifecycle(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	store := auth.NewSessionStore(time.Hour)
	sess, err := store.Create("admin")
	require.NoError(t, err)

	hub := webui.NewWSHub(logger)
	hubCtx, hubCancel := context.WithCancel(context.Background())
	hubDone := make(chan struct{})
	go func() {
		hub.Run(hubCtx)
		close(hubDone)
	}()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hub.UpgradeAndServe(w, r, store)
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	dialCtx, dialCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer dialCancel()

	header := http.Header{}
	header.Set("Origin", server.URL)
	header.Set("Cookie", (&http.Cookie{Name: "statix_session", Value: sess.ID}).String())

	conn, _, err := websocket.Dial(dialCtx, wsURL, &websocket.DialOptions{
		HTTPHeader: header,
	})
	require.NoError(t, err)

	// Trigger hub shutdown
	hubCancel()

	select {
	case <-hubDone:
		// Hub shut down cleanly
	case <-time.After(2 * time.Second):
		t.Fatal("hub.Run did not terminate after context cancel")
	}

	// Verify reading from conn sees server going away
	readCtx, readCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer readCancel()
	_, _, readErr := conn.Read(readCtx)
	assert.Error(t, readErr, "expected connection closure on shutdown")

	// Ensure Publish on stopped hub does not panic or hang
	hub.Publish(metrics.Snapshot{MemTotal: 999})
}

func TestWSHubConcurrency(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	hub := webui.NewWSHub(logger)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	var wg sync.WaitGroup
	// Concurrently publish snapshots
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				hub.Publish(metrics.Snapshot{
					MemTotal: uint64(id*100 + j),
				})
			}
		}(i)
	}
	wg.Wait()
}
