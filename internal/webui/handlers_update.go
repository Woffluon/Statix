package webui

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/statix/statix/internal/updater"
)

// UpdateManager specifies the contract required for self-updates.
type UpdateManager interface {
	CheckUpdate(ctx context.Context, currentVersion string) (*updater.ReleaseInfo, error)
	ApplyUpdate(ctx context.Context, info *updater.ReleaseInfo, logger *slog.Logger) error
}

type updateCheckResponse struct {
	CurrentVersion  string `json:"current_version"`
	UpdateAvailable bool   `json:"update_available"`
	LatestVersion   string `json:"latest_version,omitempty"`
	TagName         string `json:"tag_name,omitempty"`
	ReleaseNotes    string `json:"release_notes,omitempty"`
	PublishedAt     string `json:"published_at,omitempty"`
	AssetURL        string `json:"asset_url,omitempty"`
	ChecksumURL     string `json:"checksum_url,omitempty"`
}

type updateApplyResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Version string `json:"version,omitempty"`
}

func (s *Server) handleUpdateCheck(w http.ResponseWriter, r *http.Request) {
	s.logger.Info("update_check_requested", "current_version", s.version)

	mgr := s.updater
	if mgr == nil {
		mgr = updater.DefaultUpdater
	}

	info, err := mgr.CheckUpdate(r.Context(), s.version)
	if err != nil {
		s.logger.Error("update_check_failed", "error", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error": "Failed to check for updates: " + err.Error(),
		})
		return
	}

	res := updateCheckResponse{
		CurrentVersion:  s.version,
		UpdateAvailable: info.UpdateAvailable,
		LatestVersion:   info.Version,
		TagName:         info.TagName,
		ReleaseNotes:    info.ReleaseNotes,
		PublishedAt:     info.PublishedAt.Format("2006-01-02T15:04:05Z07:00"),
		AssetURL:        info.AssetURL,
		ChecksumURL:     info.ChecksumURL,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(res)
}

func (s *Server) handleUpdateApply(w http.ResponseWriter, r *http.Request) {
	// Guard against concurrent update attempts
	if !s.isUpdating.CompareAndSwap(false, true) {
		s.logger.Warn("update_apply_rejected_concurrent")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error": "An update is already in progress.",
		})
		return
	}
	defer s.isUpdating.Store(false)

	s.logger.Info("update_apply_started", "current_version", s.version)

	mgr := s.updater
	if mgr == nil {
		mgr = updater.DefaultUpdater
	}

	// Always verify latest release info before applying
	info, err := mgr.CheckUpdate(r.Context(), s.version)
	if err != nil {
		s.logger.Error("update_apply_check_failed", "error", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error": "Failed to verify release before applying: " + err.Error(),
		})
		return
	}

	if !info.UpdateAvailable {
		s.logger.Info("update_apply_noop", "current_version", s.version, "latest", info.Version)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error": "Statix is already running the latest version.",
		})
		return
	}

	if err := mgr.ApplyUpdate(r.Context(), info, s.logger); err != nil {
		s.logger.Error("update_apply_failed", "error", err)
		w.Header().Set("Content-Type", "application/json")
		status := http.StatusInternalServerError
		if errors.Is(err, updater.ErrChecksumMismatch) {
			status = http.StatusBadRequest
		}
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error": "Failed to apply update: " + err.Error(),
		})
		return
	}

	s.logger.Info("update_apply_completed", "new_version", info.Version)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(updateApplyResponse{
		Status:  "success",
		Message: "Update applied successfully. Server restarting...",
		Version: info.Version,
	})
}
