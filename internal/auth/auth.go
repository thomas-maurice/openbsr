package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"github.com/thomas-maurice/openbsr/internal/model"
	"golang.org/x/crypto/bcrypt"
)

type contextKey string

const userKey contextKey = "user"

func HashToken(raw string) string {
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}

func HashPassword(password string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(b), err
}

func CheckPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

func ExtractBearer(authHeader string) string {
	return strings.TrimPrefix(authHeader, "Bearer ")
}

func ContextWithUser(ctx context.Context, u *model.User) context.Context {
	return context.WithValue(ctx, userKey, u)
}

func UserFromContext(ctx context.Context) *model.User {
	u, _ := ctx.Value(userKey).(*model.User)
	return u
}
