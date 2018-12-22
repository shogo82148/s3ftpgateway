// Package vfs defines a virtual file system interface whose
// methods accept a context.Context parameter and write operation.
// It is otherwise similar to golang.org/x/tools/godoc/vfs and https://github.com/sourcegraph/ctxvfs .
package vfs

import (
	"context"
	"io"
	"os"
)

// The FileSystem interface specifies the methods used to access the
// file system.
type FileSystem interface {
	// Open opens the named file.
	Open(ctx context.Context, name string) (io.ReadCloser, error)

	// Lstat returns a FileInfo describing the named file.
	Lstat(ctx context.Context, path string) (os.FileInfo, error)

	// Stat returns a FileInfo describing the named file. If there is an error, it will be of type *PathError.
	Stat(ctx context.Context, path string) (os.FileInfo, error)

	// ReadDir reads the contents of the directory.
	ReadDir(ctx context.Context, path string) ([]os.FileInfo, error)

	// Create creates the named file, truncating it if it already exists.
	Create(ctx context.Context, name string) (io.WriteCloser, error)

	// Mkdir creates a new directory. If name is already a directory, Mkdir
	// returns an error (that can be detected using os.IsExist).
	Mkdir(ctx context.Context, name string) error

	// Remove removes the named file or directory.
	Remove(ctx context.Context, name string) error

	String() string
}
