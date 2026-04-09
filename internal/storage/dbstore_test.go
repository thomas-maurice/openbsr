// Copyright (c) 2026 Thomas Maurice
// SPDX-License-Identifier: MIT

package storage

import (
	"context"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupDBStore(t *testing.T) *DBStore {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	store, err := NewDBStore(db)
	if err != nil {
		t.Fatal(err)
	}
	return store
}

func TestDBStore_PutGet(t *testing.T) {
	store := setupDBStore(t)
	ctx := context.Background()

	if err := store.Put(ctx, "mod1/commit1", []byte("hello proto")); err != nil {
		t.Fatal(err)
	}
	data, err := store.Get(ctx, "mod1/commit1")
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello proto" {
		t.Fatalf("expected 'hello proto', got '%s'", string(data))
	}
}

func TestDBStore_Overwrite(t *testing.T) {
	store := setupDBStore(t)
	ctx := context.Background()

	store.Put(ctx, "key1", []byte("v1"))
	store.Put(ctx, "key1", []byte("v2"))

	data, _ := store.Get(ctx, "key1")
	if string(data) != "v2" {
		t.Fatalf("expected 'v2' after overwrite, got '%s'", string(data))
	}
}

func TestDBStore_NotFound(t *testing.T) {
	store := setupDBStore(t)
	ctx := context.Background()

	_, err := store.Get(ctx, "nonexistent")
	if err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
