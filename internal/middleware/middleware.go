package middleware

import (
	"context"
	"net/http"

	"github.com/alexedwards/scs/v2"
	"github.com/google/uuid"
	"github.com/tokoonline/app/internal/services/security"
)

type ctxKey string

const (
	UserIDKey   ctxKey = "user_id"
	UserRoleKey ctxKey = "user_role"
	UserEmailKey ctxKey = "user_email"
	SessTokenKey ctxKey = "sess_token"
)

func InjectSessionToken(sm *scs.SessionManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tok := sm.GetString(r.Context(), "device_token")
			if tok == "" {
				tok, _ = security.RandomToken(16)
				sm.Put(r.Context(), "device_token", tok)
			}
			ctx := context.WithValue(r.Context(), SessTokenKey, tok)

			if uidStr := sm.GetString(r.Context(), "user_id"); uidStr != "" {
				if uid, err := uuid.Parse(uidStr); err == nil {
					ctx = context.WithValue(ctx, UserIDKey, uid)
					ctx = context.WithValue(ctx, UserRoleKey, sm.GetString(r.Context(), "user_role"))
					ctx = context.WithValue(ctx, UserEmailKey, sm.GetString(r.Context(), "user_email"))
				}
			}
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func RequireRole(roles ...string) func(http.Handler) http.Handler {
	allowed := map[string]bool{}
	for _, r := range roles {
		allowed[r] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			role, _ := r.Context().Value(UserRoleKey).(string)
			if !allowed[role] {
				redirect := "/login"
				switch {
				case allowed["admin"], allowed["staff"]:
					redirect = "/admin/login?next=" + r.URL.Path
				case allowed["reseller"]:
					redirect = "/reseller/login?next=" + r.URL.Path
				}
				http.Redirect(w, r, redirect, http.StatusFound)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func UserID(r *http.Request) *uuid.UUID {
	uid, ok := r.Context().Value(UserIDKey).(uuid.UUID)
	if !ok {
		return nil
	}
	return &uid
}

func UserRole(r *http.Request) string {
	v, _ := r.Context().Value(UserRoleKey).(string)
	return v
}

func UserEmail(r *http.Request) string {
	v, _ := r.Context().Value(UserEmailKey).(string)
	return v
}

func SessionToken(r *http.Request) string {
	v, _ := r.Context().Value(SessTokenKey).(string)
	return v
}

func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "SAMEORIGIN")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Permissions-Policy", "interest-cohort=()")
		next.ServeHTTP(w, r)
	})
}
