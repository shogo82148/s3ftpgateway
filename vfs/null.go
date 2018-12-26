package vfs

import (
	"context"
	"io"
	"io/ioutil"
	"os"
)

// Null is a null file system.
// All read operations fail with os.ErrNotExist,
// and all write operations succeed, but no effect.
var Null FileSystem = null{}

type null struct{}

func (null) Open(ctx context.Context, name string) (io.ReadCloser, error) {
	return nil, &os.PathError{
		Op:   "open",
		Path: name,
		Err:  os.ErrNotExist,
	}
}

func (null) Lstat(ctx context.Context, path string) (os.FileInfo, error) {
	return nil, &os.PathError{
		Op:   "stat",
		Path: path,
		Err:  os.ErrNotExist,
	}
}

func (null) Stat(ctx context.Context, path string) (os.FileInfo, error) {
	return null{}.Lstat(ctx, path)
}

func (null) ReadDir(ctx context.Context, path string) ([]os.FileInfo, error) {
	if path == "" || path == "/" {
		return []os.FileInfo{}, nil
	}
	return nil, &os.PathError{
		Op:   "readdir",
		Path: path,
		Err:  os.ErrNotExist,
	}
}

func (null) Create(ctx context.Context, name string, body io.Reader) error {
	_, err := io.Copy(ioutil.Discard, body)
	return err
}

func (null) Mkdir(ctx context.Context, name string) error {
	return nil
}

func (null) Remove(ctx context.Context, name string) error {
	return nil
}

func (null) String() string { return "null" }
