package mapfs

import (
	"context"
	"io"
	"os"
	"path"
	"strings"
	"sync"

	"github.com/shogo82148/s3ftpgateway/vfs"
)

// New returns a new FileSystem from the provided map.
// Map keys should be forward slash-separated pathnames
// and not contain a leading slash.
func New(m map[string]string) vfs.FileSystem {
	return &mapFS{m: m}
}

// mapFS is the map based implementation of FileSystem
type mapFS struct {
	mu sync.RWMutex
	m  map[string]string
}

func (fs *mapFS) String() string {
	return "mapfs"
}

func filename(p string) string {
	return strings.TrimPrefix(path.Clean(p), "/")
}

// Open opens the file.
func (fs *mapFS) Open(ctx context.Context, name string) (vfs.ReadSeekCloser, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	b, ok := fs.m[filename(name)]
	if !ok {
		return nil, os.ErrNotExist
	}
	return nopCloser{strings.NewReader(b)}, nil
}

// Lstat returns a FileInfo describing the named file.
func (fs *mapFS) Lstat(ctx context.Context, path string) (os.FileInfo, error) {
	return nil, nil
}

// Stat returns a FileInfo describing the named file. If there is an error, it will be of type *PathError.
func (fs *mapFS) Stat(ctx context.Context, path string) (os.FileInfo, error) {
	return fs.Lstat(ctx, path)
}

// ReadDir reads the contents of the directory.
func (fs *mapFS) ReadDir(ctx context.Context, path string) ([]os.FileInfo, error) {
	return nil, nil
}

// Create creates the named file, truncating it if it already exists.
func (fs *mapFS) Create(ctx context.Context, name string) (vfs.WriteSeekCloser, error) {
	return nil, nil
}

// Mkdir creates a new directory. If name is already a directory, Mkdir
// returns an error (that can be detected using os.IsExist).
func (fs *mapFS) Mkdir(ctx context.Context, name string) error {
	return nil
}

// Remove removes the named file or directory.
func (fs *mapFS) Remove(ctx context.Context, name string) error {
	return nil
}

type nopCloser struct {
	io.ReadSeeker
}

func (nc nopCloser) Close() error { return nil }
