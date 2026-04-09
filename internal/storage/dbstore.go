// Copyright (c) 2026 Thomas Maurice
// SPDX-License-Identifier: MIT

package storage

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Blob is the GORM model for database-backed blob storage.
type Blob struct {
	Key       string `gorm:"primaryKey"`
	Data      []byte `gorm:"type:blob;not null"`
	CreatedAt time.Time
}

// DBStore implements Store by persisting blobs in a SQL database via GORM.
// It can use any GORM-compatible database (SQLite, PostgreSQL, etc.) and
// does not have to be the same database as the control plane.
type DBStore struct {
	db *gorm.DB
}

// NewDBStore creates a new database-backed blob store.
// It auto-migrates the blobs table on the provided GORM connection.
func NewDBStore(db *gorm.DB) (*DBStore, error) {
	if err := db.AutoMigrate(&Blob{}); err != nil {
		return nil, err
	}
	return &DBStore{db: db}, nil
}

// Put stores a blob. If the key already exists, the data is overwritten (upsert).
func (s *DBStore) Put(_ context.Context, key string, data []byte) error {
	blob := &Blob{Key: key, Data: data, CreatedAt: time.Now().UTC()}
	return s.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{"data", "created_at"}),
	}).Create(blob).Error
}

// Get retrieves a blob by key. Returns ErrNotFound if the key doesn't exist.
func (s *DBStore) Get(_ context.Context, key string) ([]byte, error) {
	var blob Blob
	if err := s.db.Where("`key` = ?", key).First(&blob).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return blob.Data, nil
}
