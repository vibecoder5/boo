package drive

import (
	"context"
	"io"
	"time"
)

type remoteFile struct {
	ID      string
	Name    string
	MD5     string
	Size    int64
	ModTime time.Time
	Folder  bool
}

type Backend interface {
	About(ctx context.Context) (string, error)
	EnsureDir(ctx context.Context, parentID, name string) (string, error)
	List(ctx context.Context, parentID string) ([]remoteFile, error)
	Upload(ctx context.Context, parentID, name string, mod time.Time, r io.Reader, size int64) (remoteFile, error)
	Update(ctx context.Context, id string, mod time.Time, r io.Reader, size int64) (remoteFile, error)
	Download(ctx context.Context, id string) (io.ReadCloser, error)
}
