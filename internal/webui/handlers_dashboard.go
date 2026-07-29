package webui

import (
	"encoding/json"
	"html/template"
	"net/http"

	"github.com/statix/statix/internal/metrics"
)

type dashboardPageData struct {
	CSRFToken   string
	ShowHeader  bool
	Snapshot    metrics.Snapshot
	InitialJSON template.JS
}

func (s *Server) handleDashboardGet(w http.ResponseWriter, r *http.Request) {
	csrfToken, _ := s.ensureCSRFCookie(w, r)

	latest, _ := s.buf.Latest()
	jsonData, _ := json.Marshal(latest)

	data := dashboardPageData{
		CSRFToken:   csrfToken,
		ShowHeader:  true,
		Snapshot:    latest,
		InitialJSON: template.JS(jsonData),
	}

	s.renderTemplate(w, r, "dashboard.html", data)
}

func (s *Server) handleWSGet(w http.ResponseWriter, r *http.Request) {
	s.hub.UpgradeAndServe(w, r, s.store)
}
