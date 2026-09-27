package webui

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"

	"github.com/coder/websocket"
	"github.com/statix/statix/internal/auth"
	"github.com/statix/statix/internal/metrics"
)

type wsClient struct {
	conn   *websocket.Conn
	send   chan metrics.Snapshot
	ctx    context.Context
	cancel context.CancelFunc
}

type WSHub struct {
	register     chan *wsClient
	unregister   chan *wsClient
	broadcast    chan metrics.Snapshot
	clients      map[*wsClient]struct{}
	lastSnapshot metrics.Snapshot
	hasLast      bool
	mu           sync.RWMutex
	logger       *slog.Logger
	done         chan struct{}
	closeOnce    sync.Once
}

func NewWSHub(logger *slog.Logger) *WSHub {
	return &WSHub{
		register:   make(chan *wsClient),
		unregister: make(chan *wsClient),
		broadcast:  make(chan metrics.Snapshot, 10),
		clients:    make(map[*wsClient]struct{}),
		logger:     logger,
		done:       make(chan struct{}),
	}
}

func (h *WSHub) Run(ctx context.Context) {
	defer func() {
		h.closeOnce.Do(func() {
			close(h.done)
		})
		h.closeAll()
	}()

	for {
		select {
		case <-ctx.Done():
			return

		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = struct{}{}
			if h.hasLast {
				select {
				case client.send <- h.lastSnapshot:
				default:
				}
			}
			h.mu.Unlock()
			h.logger.Debug("wshub: client registered")

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
				_ = client.conn.CloseNow()
			}
			h.mu.Unlock()
			h.logger.Debug("wshub: client unregistered")

		case snapshot := <-h.broadcast:
			h.mu.Lock()
			h.lastSnapshot = snapshot
			h.hasLast = true
			for client := range h.clients {
				select {
				case client.send <- snapshot:
				default:
					// Ring-buffer behavior: drop oldest snapshot to make room for newest telemetry snapshot
					select {
					case <-client.send:
					default:
					}
					select {
					case client.send <- snapshot:
					default:
					}
				}
			}
			h.mu.Unlock()
		}
	}
}

func (h *WSHub) Publish(s metrics.Snapshot) {
	select {
	case <-h.done:
		return
	case h.broadcast <- s:
	default:
		h.logger.Warn("wshub: broadcast channel full, dropping snapshot")
	}
}

func (h *WSHub) closeAll() {
	h.mu.Lock()
	defer h.mu.Unlock()

	for client := range h.clients {
		client.cancel()
		_ = client.conn.CloseNow()
		close(client.send)
		delete(h.clients, client)
	}
}

func (h *WSHub) UpgradeAndServe(w http.ResponseWriter, r *http.Request, store *auth.SessionStore) {
	// Authenticate WebSocket upgrade request
	cookie, err := r.Cookie("statix_session")
	if err != nil || cookie.Value == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	_, valid := store.Get(cookie.Value)
	if !valid {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// CSWSH Protection: coder/websocket Accept validates Origin against Host header
	// when OriginPatterns is nil and InsecureSkipVerify is false.
	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		h.logger.Error("wshub: websocket accept failed", "error", err)
		return
	}

	ctx, cancel := context.WithCancel(r.Context())
	client := &wsClient{
		conn:   conn,
		send:   make(chan metrics.Snapshot, 64),
		ctx:    ctx,
		cancel: cancel,
	}

	select {
	case h.register <- client:
	case <-h.done:
		_ = conn.Close(websocket.StatusGoingAway, "server shutting down")
		cancel()
		return
	case <-ctx.Done():
		_ = conn.Close(websocket.StatusGoingAway, "request cancelled")
		cancel()
		return
	}

	defer func() {
		cancel()
		select {
		case h.unregister <- client:
		case <-h.done:
		default:
		}
	}()

	// Writer loop
	for {
		select {
		case <-ctx.Done():
			return
		case snapshot, ok := <-client.send:
			if !ok {
				return
			}
			payload, err := json.Marshal(snapshot)
			if err != nil {
				continue
			}
			err = conn.Write(ctx, websocket.MessageText, payload)
			if err != nil {
				h.logger.Debug("wshub: write error", "error", err)
				return
			}
		}
	}
}
