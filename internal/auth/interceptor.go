package auth

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/thomas-maurice/openbsr/internal/db/iface"
)

func NewConnectInterceptor(repos *iface.Repos) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			raw := strings.TrimPrefix(req.Header().Get("Authorization"), "Bearer ")
			if raw == "" || raw == req.Header().Get("Authorization") {
				return next(ctx, req)
			}
			hash := HashToken(raw)
			t, err := repos.Tokens.GetByHash(ctx, hash)
			if err != nil {
				if errors.Is(err, iface.ErrNotFound) {
					return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("invalid token"))
				}
				slog.Error("token lookup", "err", err)
				return nil, connect.NewError(connect.CodeInternal, errors.New("internal error"))
			}
			if t.ExpiresAt != nil && time.Now().After(*t.ExpiresAt) {
				return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("token expired"))
			}
			u, err := repos.Users.GetByID(ctx, t.UserID)
			if err != nil {
				slog.Error("user lookup", "err", err)
				return nil, connect.NewError(connect.CodeInternal, errors.New("internal error"))
			}
			ctx = ContextWithUser(ctx, u)
			return next(ctx, req)
		}
	}
}
