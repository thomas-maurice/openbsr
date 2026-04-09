package storage

import (
	"context"
	"os"
	"testing"
)

func TestLocalStore(t *testing.T) {
	dir, err := os.MkdirTemp("", "bsr-storage-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	store := NewLocalStore(dir)
	ctx := context.Background()

	data := []byte("hello proto world")
	if err := store.Put(ctx, "mod1/commit1", data); err != nil {
		t.Fatal(err)
	}

	got, err := store.Get(ctx, "mod1/commit1")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(data) {
		t.Fatalf("got %q, want %q", got, data)
	}

	_, err = store.Get(ctx, "nonexistent/key")
	if err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestLocalStore_PathTraversal(t *testing.T) {
	dir, err := os.MkdirTemp("", "bsr-storage-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	store := NewLocalStore(dir)
	ctx := context.Background()

	malicious := []string{
		"../../etc/passwd",
		"../../../tmp/evil",
		"mod1/../../outside",
	}
	for _, key := range malicious {
		if err := store.Put(ctx, key, []byte("bad")); err == nil {
			t.Fatalf("Put(%q) should have failed", key)
		}
		if _, err := store.Get(ctx, key); err == nil {
			t.Fatalf("Get(%q) should have failed", key)
		}
	}
}
