package webui

import (
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/statix/statix/internal/auth"
	"github.com/statix/statix/internal/config"
)

type setupPageData struct {
	CSRFToken       string
	ShowHeader      bool
	Version         string
	Error           string
	Username        string
	ListenAddr      string
	UsernameError   string
	PasswordError   string
	ListenAddrError string
}

func validateListenAddr(addr string) error {
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("must be in host:port or :port format (e.g. :8080 or 127.0.0.1:8080)")
	}
	port, err := strconv.Atoi(portStr)
	if err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("port must be an integer between 1 and 65535")
	}
	if host != "" && net.ParseIP(host) == nil {
		if strings.ContainsAny(host, " /\\:;?#@!$%^&*()+=`~<>{}[]|\"'") {
			return fmt.Errorf("invalid host format")
		}
	}
	return nil
}

func (s *Server) setupGuard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.RLock()
		complete := s.cfg.SetupComplete
		s.mu.RUnlock()

		if complete {
			http.Error(w, "Setup wizard has already been completed.", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) handleSetupGet(w http.ResponseWriter, r *http.Request) {
	csrfToken, _ := s.ensureCSRFCookie(w, r)

	data := setupPageData{
		CSRFToken:  csrfToken,
		ShowHeader: false,
		Version:    s.version,
		ListenAddr: s.cfg.ListenAddr,
	}

	s.renderTemplate(w, r, "setup.html", data)
}

func (s *Server) handleSetupPost(w http.ResponseWriter, r *http.Request) {
	username := strings.TrimSpace(r.FormValue("username"))
	password := r.FormValue("password")
	confirmPassword := r.FormValue("confirm_password")
	listenAddr := strings.TrimSpace(r.FormValue("listen_addr"))

	csrfToken, _ := s.ensureCSRFCookie(w, r)

	data := setupPageData{
		CSRFToken:  csrfToken,
		ShowHeader: false,
		Version:    s.version,
		Username:   username,
		ListenAddr: listenAddr,
	}

	var hasErr bool
	if username == "" {
		data.UsernameError = "Username is required."
		hasErr = true
	}

	if len(password) < 8 {
		data.PasswordError = "Password must be at least 8 characters long."
		hasErr = true
	} else if password != confirmPassword {
		data.PasswordError = "Passwords do not match."
		hasErr = true
	}

	if listenAddr == "" {
		listenAddr = ":8080"
	}
	if err := validateListenAddr(listenAddr); err != nil {
		data.ListenAddrError = "Invalid listen address: " + err.Error()
		hasErr = true
	}

	if hasErr {
		s.renderTemplate(w, r, "setup.html", data)
		return
	}

	// Hash password
	pwHash, err := auth.HashPassword(password)
	if err != nil {
		data.Error = "Failed to hash password."
		s.renderTemplate(w, r, "setup.html", data)
		return
	}

	// Generate 32-byte session secret
	sessionSecret, err := auth.GenerateCSRFToken()
	if err != nil {
		data.Error = "Failed to generate session secret."
		s.renderTemplate(w, r, "setup.html", data)
		return
	}

	// Save configuration
	s.mu.Lock()
	s.cfg.AdminUsername = username
	s.cfg.AdminPasswordHash = pwHash
	s.cfg.SessionSecret = sessionSecret
	s.cfg.ListenAddr = listenAddr
	s.cfg.SetupComplete = true
	saveErr := config.Save(s.cfgPath, s.cfg)
	s.mu.Unlock()

	if saveErr != nil {
		data.Error = "Failed to save configuration: " + saveErr.Error()
		s.renderTemplate(w, r, "setup.html", data)
		return
	}

	s.logger.Info("setup_complete", "username", username, "listen_addr", listenAddr)

	http.Redirect(w, r, "/login", http.StatusFound)
}
