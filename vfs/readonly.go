package vfs

import (
	"context"
	"io"
	"os"
)

// ReadOnly makes fs read only.
func ReadOnly(fs FileSystem) FileSystem {
	if fs == nil {
		fs = Null
	}
	return readonly{fs}
}

type readonly struct {
	FileSystem
}

func (fs readonly) Lstat(ctx context.Context, path string) (os.FileInfo, error) {
	stat, err := fs.FileSystem.Lstat(ctx, path)
	if err != nil {
		return nil, err
	}
	return readonlyStat{stat}, nil
}

func (fs readonly) Stat(ctx context.Context, path string) (os.FileInfo, error) {
	stat, err := fs.FileSystem.Stat(ctx, path)
	if err != nil {
		return nil, err
	}
	return readonlyStat{stat}, nil
}

func (fs readonly) ReadDir(ctx context.Context, path string) ([]os.FileInfo, error) {
	stats, err := fs.FileSystem.ReadDir(ctx, path)
	if err != nil {
		return nil, err
	}
	for i, stat := range stats {
		stats[i] = readonlyStat{stat}
	}
	return stats, nil
}

func (fs readonly) Create(ctx context.Context, name string, body io.Reader) error {
	return &os.PathError{
		Op:   "create",
		Path: name,
		Err:  os.ErrPermission,
	}
}

func (fs readonly) Mkdir(ctx context.Context, name string) error {
	return &os.PathError{
		Op:   "mkdir",
		Path: name,
		Err:  os.ErrPermission,
	}
}

func (fs readonly) Remove(ctx context.Context, name string) error {
	return &os.PathError{
		Op:   "remove",
		Path: name,
		Err:  os.ErrPermission,
	}
}

func (fs readonly) String() string { return "readonly " + fs.FileSystem.String() }

type readonlyStat struct {
	os.FileInfo
}

func (stat readonlyStat) Mode() os.FileMode {
	return stat.FileInfo.Mode() &^ 0222
}
