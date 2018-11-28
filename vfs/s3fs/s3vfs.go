package s3fs

import (
	"context"
	"os"

	"github.com/shogo82148/s3ftpgateway/vfs"
)

// FileSystem implements ctxvfs.FileSystem
type FileSystem struct {
}

// Open opens the file.
func (fs *FileSystem) Open(ctx context.Context, name string) (vfs.ReadSeekCloser, error) {
	return nil, nil
}

// Lstat returns a FileInfo describing the named file.
func (fs *FileSystem) Lstat(ctx context.Context, path string) (os.FileInfo, error) {
	return nil, nil
}

// Stat returns a FileInfo describing the named file. If there is an error, it will be of type *PathError.
func (fs *FileSystem) Stat(ctx context.Context, path string) (os.FileInfo, error) {
	return nil, nil
}

// ReadDir reads the contents of the directory.
func (fs *FileSystem) ReadDir(ctx context.Context, path string) ([]os.FileInfo, error) {
	return nil, nil
}

// Create creates the named file, truncating it if it already exists.
func (fs *FileSystem) Create(ctx context.Context, name string) (vfs.WriteSeekCloser, error) {
	return nil, nil
}

// Mkdir creates a new directory. If name is already a directory, Mkdir
// returns an error (that can be detected using os.IsExist).
func (fs *FileSystem) Mkdir(ctx context.Context, name string) error {
	return nil
}

// Remove removes the named file or directory.
func (fs *FileSystem) Remove(ctx context.Context, name string) error {
	return nil
}


func (fs *FileSystem) String() string {
	return ""
}
