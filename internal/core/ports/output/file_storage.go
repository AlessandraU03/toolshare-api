package output

import (
	"context"
	"io"
)

type FileStorage interface {
	Upload(ctx context.Context, filename string, content io.Reader, contentType string) (string, error)
	Delete(ctx context.Context, filename string) error
}
