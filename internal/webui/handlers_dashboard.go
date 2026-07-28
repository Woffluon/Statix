package webui

import (
	"net/http"

	"github.com/statix/statix/internal/metrics"
)

type dashboardPageData struct {
	CSRFToken  string
	ShowHeader bool
	TLSEnabled bool
	Snapshot   metrics.Snapshot
}

func (s *Server) handleDashboardGet(w http.ResponseWriter, r *http.Request) {
	csrfToken, _ := s.ensureCSRFCookie(w, r)

	latest, _ := s.buf.Latest()

	s.mu.RLock()
	tlsEnabled := s.cfg.TLSEnabled
	s.mu.RUnlock()

	data := dashboardPageData{
		CSRFToken:  csrfToken,
		ShowHeader: true,
		TLSEnabled: tlsEnabled,
		Snapshot:   latest,
	}

	s.renderTemplate(w, r, "dashboard.html", data)
}

func (s *Server) handleWSGet(w http.ResponseWriter, r *http.Request) {
	s.hub.UpgradeAndServe(w, r, s.store)
}
