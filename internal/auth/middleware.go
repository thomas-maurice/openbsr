package auth

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/thomas-maurice/openbsr/internal/db/iface"
)

func Middleware(repos *iface.Repos) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			bearer := ExtractBearer(r.Header.Get("Authorization"))
			if bearer == "" || bearer == r.Header.Get("Authorization") {
				next.ServeHTTP(w, r)
				return
			}
			hash := HashToken(bearer)
			t, err := repos.Tokens.GetByHash(r.Context(), hash)
			if err != nil {
				if errors.Is(err, iface.ErrNotFound) {
					next.ServeHTTP(w, r)
					return
				}
				slog.Error("token lookup", "err", err)
				next.ServeHTTP(w, r)
				return
			}
			if t.ExpiresAt != nil && time.Now().After(*t.ExpiresAt) {
				next.ServeHTTP(w, r)
				return
			}
			u, err := repos.Users.GetByID(r.Context(), t.UserID)
			if err != nil {
				slog.Error("user lookup", "err", err)
				next.ServeHTTP(w, r)
				return
			}
			ctx := ContextWithUser(r.Context(), u)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
