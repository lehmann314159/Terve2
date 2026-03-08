package auth

import (
	"context"
	"net/http"
)

type contextKey struct{}

// Middleware reads the session cookie and stores it in the request context.
// Guests pass through with a nil session.
func Middleware(store *CookieStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sess := store.Load(r)
			ctx := context.WithValue(r.Context(), contextKey{}, sess)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetSession returns the session from the request context, or nil for guests.
func GetSession(ctx context.Context) *Session {
	sess, _ := ctx.Value(contextKey{}).(*Session)
	return sess
}

// RequireAuth returns 401 for unauthenticated requests. Wire up in Phase 2.
func RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if GetSession(r.Context()) == nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
