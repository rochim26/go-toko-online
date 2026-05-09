package httpx

import (
	"context"
	"net/http"

	"github.com/a-h/templ"
	"github.com/justinas/nosurf"
)

func Render(w http.ResponseWriter, r *http.Request, c templ.Component) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := c.Render(r.Context(), w); err != nil {
		http.Error(w, "render error", http.StatusInternalServerError)
	}
}

func RenderStatus(w http.ResponseWriter, r *http.Request, status int, c templ.Component) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_ = c.Render(r.Context(), w)
}

type ctxKey string

const csrfCtxKey ctxKey = "csrf"

func WithCSRF(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, csrfCtxKey, token)
}

func CSRFFrom(ctx context.Context) string {
	v, _ := ctx.Value(csrfCtxKey).(string)
	return v
}

func CSRFToken(r *http.Request) string {
	return nosurf.Token(r)
}

func IsHTMX(r *http.Request) bool {
	return r.Header.Get("HX-Request") == "true"
}

func ClientIP(r *http.Request) string {
	if ip := r.Header.Get("CF-Connecting-IP"); ip != "" {
		return ip
	}
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}
	if ip := r.Header.Get("X-Forwarded-For"); ip != "" {
		// take first
		for i := 0; i < len(ip); i++ {
			if ip[i] == ',' {
				return ip[:i]
			}
		}
		return ip
	}
	return r.RemoteAddr
}

func Cookie(r *http.Request, name string) string {
	c, err := r.Cookie(name)
	if err != nil {
		return ""
	}
	return c.Value
}

func SetCookie(w http.ResponseWriter, name, value string, maxAge int, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   secure,
	})
}
