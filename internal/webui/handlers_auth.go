package webui

import (
	"net/http"

	"github.com/statix/statix/internal/auth"
)

type loginPageData struct {
	CSRFToken  string
	ShowHeader bool
	Error      string
}

func (s *Server) handleLoginGet(w http.ResponseWriter, r *http.Request) {
	// If already authenticated, redirect to /dashboard
	if cookie, err := r.Cookie("statix_session"); err == nil && cookie.Value != "" {
		if _, ok := s.store.Get(cookie.Value); ok {
			http.Redirect(w, r, "/dashboard", http.StatusFound)
			return
		}
	}

	csrfToken, _ := s.ensureCSRFCookie(w, r)

	data := loginPageData{
		CSRFToken:  csrfToken,
		ShowHeader: false,
	}

	s.renderTemplate(w, r, "login.html", data)
}

func (s *Server) handleLoginPost(w http.ResponseWriter, r *http.Request) {
	ip := getRemoteIP(r)
	username := r.FormValue("username")
	password := r.FormValue("password")

	allowed, retryAfter := s.rl.Allow(ip, username)
	if !allowed {
		w.Header().Set("Retry-After", retryAfter.String())
		csrfToken, _ := s.ensureCSRFCookie(w, r)
		s.renderTemplate(w, r, "login.html", loginPageData{
			CSRFToken:  csrfToken,
			ShowHeader: false,
			Error:      "Too many failed login attempts. Account temporarily locked.",
		})
		return
	}

	if username != s.cfg.AdminUsername {
		s.rl.Record(ip, username)
		csrfToken, _ := s.ensureCSRFCookie(w, r)
		s.renderTemplate(w, r, "login.html", loginPageData{
			CSRFToken:  csrfToken,
			ShowHeader: false,
			Error:      "Invalid username or password.",
		})
		return
	}

	valid, err := auth.VerifyPassword(password, s.cfg.AdminPasswordHash)
	if err != nil || !valid {
		s.rl.Record(ip, username)
		csrfToken, _ := s.ensureCSRFCookie(w, r)
		s.renderTemplate(w, r, "login.html", loginPageData{
			CSRFToken:  csrfToken,
			ShowHeader: false,
			Error:      "Invalid username or password.",
		})
		return
	}

	// Login successful
	s.rl.Reset(ip, username)
	sess, err := s.store.Create(username)
	if err != nil {
		http.Error(w, "Failed to create session", http.StatusInternalServerError)
		return
	}

	// Set session cookie
	cookie := &http.Cookie{
		Name:     "statix_session",
		Value:    sess.ID,
		Path:     "/",
		HttpOnly: true,
		Secure:   isSecure(r),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   86400,
	}
	http.SetCookie(w, cookie)

	http.Redirect(w, r, "/dashboard", http.StatusFound)
}

func (s *Server) handleLogoutPost(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie("statix_session"); err == nil && cookie.Value != "" {
		s.store.Delete(cookie.Value)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "statix_session",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})

	http.Redirect(w, r, "/login", http.StatusFound)
}
