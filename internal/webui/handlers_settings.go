package webui

import (
	"net/http"
	"strings"

	"github.com/statix/statix/internal/tlsmanager"
)

type domainSettingsPageData struct {
	CSRFToken  string
	ShowHeader bool
	Domain     string
	TLSEnabled bool
	Error      string
	Message    string
}

func (s *Server) handleSettingsDomainGet(w http.ResponseWriter, r *http.Request) {
	csrfToken, _ := s.ensureCSRFCookie(w, r)

	s.mu.RLock()
	domain := s.cfg.Domain
	tlsEnabled := s.cfg.TLSEnabled
	s.mu.RUnlock()

	data := domainSettingsPageData{
		CSRFToken:  csrfToken,
		ShowHeader: true,
		Domain:     domain,
		TLSEnabled: tlsEnabled,
	}

	s.renderTemplate(w, r, "settings_domain.html", data)
}

func (s *Server) handleSettingsDomainPost(w http.ResponseWriter, r *http.Request) {
	domain := strings.TrimSpace(r.FormValue("domain"))
	csrfToken, _ := s.ensureCSRFCookie(w, r)

	s.mu.RLock()
	tlsEnabled := s.cfg.TLSEnabled
	s.mu.RUnlock()

	data := domainSettingsPageData{
		CSRFToken:  csrfToken,
		ShowHeader: true,
		Domain:     domain,
		TLSEnabled: tlsEnabled,
	}

	if err := tlsmanager.ValidateDomain(domain); err != nil {
		data.Error = err.Error()
		s.renderTemplate(w, r, "settings_domain.html", data)
		return
	}

	if s.tls != nil {
		if err := s.tls.Bind(r.Context(), domain, nil); err != nil {
			data.Error = "Failed to initiate domain binding: " + err.Error()
			s.renderTemplate(w, r, "settings_domain.html", data)
			return
		}
	}

	data.Message = "Domain binding process initiated in background."
	s.renderTemplate(w, r, "settings_domain.html", data)
}
