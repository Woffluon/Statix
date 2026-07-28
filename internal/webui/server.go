package webui

import (
	"context"
	"embed"
	"fmt"
	"html/template"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/statix/statix/internal/auth"
	"github.com/statix/statix/internal/config"
	"github.com/statix/statix/internal/metrics"
	"github.com/statix/statix/internal/tlsmanager"
)

//go:embed static
var staticFS embed.FS

type ServerDeps struct {
	Config     *config.Config
	ConfigPath string
	Store      *auth.SessionStore
	RateLimit  *auth.RateLimiter
	Buffer     *metrics.RingBuffer
	Collector  *metrics.Collector
	TLS        *tlsmanager.Manager
	Logger     *slog.Logger
}

type Server struct {
	router    *chi.Mux
	cfg       *config.Config
	cfgPath   string
	store     *auth.SessionStore
	rl        *auth.RateLimiter
	buf       *metrics.RingBuffer
	collector *metrics.Collector
	hub       *WSHub
	tls       *tlsmanager.Manager
	logger    *slog.Logger
	tmpl      *template.Template

	mu sync.RWMutex
}

func New(deps ServerDeps) (*Server, error) {
	tmpl, err := parseTemplates(staticFS)
	if err != nil {
		return nil, fmt.Errorf("webui: failed to parse templates: %w", err)
	}

	hub := NewWSHub(deps.Logger)

	s := &Server{
		cfg:       deps.Config,
		cfgPath:   deps.ConfigPath,
		store:     deps.Store,
		rl:        deps.RateLimit,
		buf:       deps.Buffer,
		collector: deps.Collector,
		hub:       hub,
		tls:       deps.TLS,
		logger:    deps.Logger,
		tmpl:      tmpl,
	}

	s.registerRoutes()
	return s, nil
}

func (s *Server) Start(ctx context.Context) {
	// Start WSHub loop
	go s.hub.Run(ctx)

	// Start Session Purge loop every 10 mins
	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.store.Purge()
			}
		}
	}()

	// Bridge collector snapshots to WSHub broadcast
	if s.collector != nil {
		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				case snapshot, ok := <-s.collector.Snapshots:
					if !ok {
						return
					}
					s.hub.Publish(snapshot)
				}
			}
		}()
	}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

func (s *Server) registerRoutes() {
	r := chi.NewRouter()

	// Global middleware
	r.Use(auth.RequestLogger(s.logger))
	r.Use(auth.Recover(s.logger))
	r.Use(auth.SecurityHeaders)

	// Static assets
	fileServer := http.FileServer(http.FS(staticFS))
	r.Handle("/static/*", fileServer)

	// Public health probe
	r.Get("/healthz", s.handleHealthzGet)

	// Setup wizard routes (protected by setupGuard)
	r.Group(func(r chi.Router) {
		r.Use(s.setupGuard)
		r.Get("/setup", s.handleSetupGet)
		r.Post("/setup", auth.CSRF(http.HandlerFunc(s.handleSetupPost)).ServeHTTP)
	})

	// Setup redirect middleware for all other routes
	r.Group(func(r chi.Router) {
		r.Use(s.setupRedirectMiddleware)

		// Unauthenticated auth routes
		r.Get("/login", s.handleLoginGet)
		r.Post("/login", auth.CSRF(http.HandlerFunc(s.handleLoginPost)).ServeHTTP)

		// Authenticated routes
		r.Group(func(r chi.Router) {
			r.Use(auth.RequireAuth(s.store))

			r.Get("/dashboard", s.handleDashboardGet)
			r.Get("/ws", s.handleWSGet)
			r.Get("/settings/domain", s.handleSettingsDomainGet)
			r.Post("/settings/domain", auth.CSRF(http.HandlerFunc(s.handleSettingsDomainPost)).ServeHTTP)
			r.Post("/logout", auth.CSRF(http.HandlerFunc(s.handleLogoutPost)).ServeHTTP)

			r.Get("/", func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, "/dashboard", http.StatusFound)
			})
		})
	})

	s.router = r
}

func (s *Server) setupRedirectMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.RLock()
		complete := s.cfg.SetupComplete
		s.mu.RUnlock()

		if !complete && r.URL.Path != "/setup" && !strings.HasPrefix(r.URL.Path, "/static/") {
			http.Redirect(w, r, "/setup", http.StatusFound)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func parseTemplates(fs embed.FS) (*template.Template, error) {
	return template.New("").ParseFS(fs, "static/templates/*.html")
}

func (s *Server) renderTemplate(w http.ResponseWriter, _ *http.Request, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	err := s.tmpl.ExecuteTemplate(w, name, data)
	if err != nil {
		s.logger.Error("webui: template render error", "template", name, "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (s *Server) ensureCSRFCookie(w http.ResponseWriter, r *http.Request) (string, error) {
	if cookie, err := r.Cookie("statix_csrf"); err == nil && cookie.Value != "" {
		return cookie.Value, nil
	}

	token, err := auth.GenerateCSRFToken()
	if err != nil {
		return "", err
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "statix_csrf",
		Value:    token,
		Path:     "/",
		HttpOnly: false,
		SameSite: http.SameSiteStrictMode,
		Secure:   s.cfg.TLSEnabled,
	})

	return token, nil
}

func getRemoteIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
