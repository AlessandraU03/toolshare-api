package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
	sharedports "github.com/yourusername/tool-inventory-api/internal/shared/ports"
)

type SupabaseStorage struct {
	supabaseURL string
	apiKey      string
	bucketName  string
	httpClient  *http.Client
}

func NewSupabaseStorage(supabaseURL, apiKey, bucketName string) sharedports.FileStorage {
	return &SupabaseStorage{
		supabaseURL: supabaseURL,
		apiKey:      apiKey,
		bucketName:  bucketName,
		httpClient:  &http.Client{Timeout: 30 * time.Second},
	}
}

func (s *SupabaseStorage) Upload(ctx context.Context, filename string, content io.Reader, contentType string) (string, error) {
	if s.supabaseURL == "" || s.apiKey == "" || s.bucketName == "" {
		return "", errors.New("supabase storage no esta correctamente configurado (faltan variables de entorno)")
	}

	// Endpoint para subir objeto a Supabase Storage:
	// POST /storage/v1/object/{bucket}/{path}
	url := fmt.Sprintf("%s/storage/v1/object/%s/%s", s.supabaseURL, s.bucketName, filename)

	req, err := http.NewRequestWithContext(ctx, "POST", url, content)
	if err != nil {
		return "", fmt.Errorf("error creando request de supabase: %w", err)
	}

	// Cabeceras de autenticaciÃ³n obligatorias de Supabase
	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	req.Header.Set("apikey", s.apiKey)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	} else {
		req.Header.Set("Content-Type", "application/octet-stream")
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("error al conectar con supabase storage: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("error de supabase storage (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	// URL pÃºblica para acceder al archivo
	// https://{project_id}.supabase.co/storage/v1/object/public/{bucket}/{path}
	publicURL := fmt.Sprintf("%s/storage/v1/object/public/%s/%s", s.supabaseURL, s.bucketName, filename)

	return publicURL, nil
}

func (s *SupabaseStorage) Delete(ctx context.Context, filename string) error {
	// Endpoint para eliminar objeto de Supabase Storage:
	// DELETE /storage/v1/object/{bucket}/{path}
	url := fmt.Sprintf("%s/storage/v1/object/%s/%s", s.supabaseURL, s.bucketName, filename)

	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("error creando request de borrado supabase: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	req.Header.Set("apikey", s.apiKey)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("error al eliminar archivo en supabase storage: %w", err)
	}
	defer resp.Body.Close()

	// Si no encuentra el archivo (404), lo consideramos correcto (idempotencia)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("error al eliminar en supabase storage (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}
