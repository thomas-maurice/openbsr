package auth

import (
	"context"
	"testing"

	"github.com/thomas-maurice/openbsr/internal/model"
)

func TestHashToken(t *testing.T) {
	hash1 := HashToken("test-token")
	hash2 := HashToken("test-token")
	if hash1 != hash2 {
		t.Fatal("same input should produce same hash")
	}
	if hash1 == "" {
		t.Fatal("hash should not be empty")
	}
	hash3 := HashToken("different-token")
	if hash1 == hash3 {
		t.Fatal("different inputs should produce different hashes")
	}
}

func TestHashPassword(t *testing.T) {
	hash, err := HashPassword("mysecretpassword")
	if err != nil {
		t.Fatal(err)
	}
	if !CheckPassword(hash, "mysecretpassword") {
		t.Fatal("correct password should match")
	}
	if CheckPassword(hash, "wrongpassword") {
		t.Fatal("wrong password should not match")
	}
}

func TestExtractBearer(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Bearer abc123", "abc123"},
		{"Bearer ", ""},
		{"", ""},
		{"Basic xyz", "Basic xyz"},
	}
	for _, tt := range tests {
		got := ExtractBearer(tt.input)
		if got != tt.want {
			t.Errorf("ExtractBearer(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestContextUser(t *testing.T) {
	ctx := context.Background()
	if u := UserFromContext(ctx); u != nil {
		t.Fatal("should be nil for empty context")
	}
	user := &model.User{ID: "123", Username: "alice"}
	ctx = ContextWithUser(ctx, user)
	got := UserFromContext(ctx)
	if got == nil {
		t.Fatal("should return user from context")
	}
	if got.ID != "123" || got.Username != "alice" {
		t.Fatalf("got %v, want id=123 username=alice", got)
	}
}
