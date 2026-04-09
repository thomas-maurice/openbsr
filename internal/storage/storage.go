package storage

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var (
	ErrNotFound    = errors.New("not found")
	ErrInvalidPath = errors.New("invalid storage key")
)

type Store interface {
	Put(ctx context.Context, key string, data []byte) error
	Get(ctx context.Context, key string) ([]byte, error)
}

type LocalStore struct {
	root string
}

func NewLocalStore(root string) *LocalStore {
	return &LocalStore{root: root}
}

func (s *LocalStore) safePath(key string) (string, error) {
	p := filepath.Join(s.root, key)
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrInvalidPath, err)
	}
	root, err := filepath.Abs(s.root)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrInvalidPath, err)
	}
	if !strings.HasPrefix(abs, root+string(filepath.Separator)) && abs != root {
		return "", ErrInvalidPath
	}
	return abs, nil
}

func (s *LocalStore) Put(_ context.Context, key string, data []byte) error {
	p, err := s.safePath(key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	return os.WriteFile(p, data, 0o644)
}

func (s *LocalStore) Get(_ context.Context, key string) ([]byte, error) {
	p, err := s.safePath(key)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p)
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNotFound
	}
	return data, err
}
