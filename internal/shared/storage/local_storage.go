package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	sharedports "github.com/yourusername/tool-inventory-api/internal/shared/ports"
)

type LocalStorage struct {
	baseDir string
	baseURL string
}

func NewLocalStorage(baseDir, baseURL string) sharedports.FileStorage {
	return &LocalStorage{baseDir: baseDir, baseURL: baseURL}
}

func (s *LocalStorage) Upload(_ context.Context, filename string, content io.Reader, _ string) (string, error) {
	fullPath := filepath.Join(s.baseDir, filename)

	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return "", fmt.Errorf("error al crear directorio: %w", err)
	}

	f, err := os.Create(fullPath)
	if err != nil {
		return "", fmt.Errorf("error al crear archivo: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, content); err != nil {
		return "", fmt.Errorf("error al guardar archivo: %w", err)
	}

	return s.baseURL + "/" + filename, nil
}

func (s *LocalStorage) Delete(_ context.Context, filename string) error {
	fullPath := filepath.Join(s.baseDir, filename)
	if err := os.Remove(fullPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("error al eliminar archivo: %w", err)
	}
	return nil
}
