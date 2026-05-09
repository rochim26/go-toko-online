package admin

import (
	"net/http"

	"github.com/tokoonline/app/internal/middleware"
)

// CompleteOnboarding marks the current admin's onboarding tour as completed.
func (h *Handler) CompleteOnboarding(w http.ResponseWriter, r *http.Request) {
	uid := middleware.UserID(r)
	if uid == nil {
		http.Error(w, "unauthorized", 401)
		return
	}
	_, _ = h.Pool.Exec(r.Context(),
		`UPDATE users SET onboarding_completed=TRUE, onboarding_completed_at=now() WHERE id=$1`, *uid)
	w.WriteHeader(http.StatusNoContent)
}

// ResetTour: marks tour incomplete so user can replay it.
func (h *Handler) ResetTour(w http.ResponseWriter, r *http.Request) {
	uid := middleware.UserID(r)
	if uid == nil {
		http.Redirect(w, r, "/admin/login", http.StatusFound)
		return
	}
	_, _ = h.Pool.Exec(r.Context(),
		`UPDATE users SET onboarding_completed=FALSE, onboarding_completed_at=NULL WHERE id=$1`, *uid)
	http.Redirect(w, r, "/admin", http.StatusFound)
}
