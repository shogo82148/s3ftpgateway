package mapfs

import (
	"context"
	"errors"
	"io"
	"os"
	pathpkg "path"
	"sort"
	"strings"
	"sync"
	"time"

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
	return strings.TrimPrefix(pathpkg.Clean(p), "/")
}

// Open opens the file.
func (fs *mapFS) Open(ctx context.Context, name string) (io.ReadCloser, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	b, ok := fs.m[filename(name)]
	if !ok {
		return nil, &os.PathError{
			Op:   "open",
			Path: filename(name),
			Err:  os.ErrNotExist,
		}
	}
	return nopCloser{strings.NewReader(b)}, nil
}

func fileInfo(name, contents string) os.FileInfo {
	return mapFI{name: pathpkg.Base(name), size: len(contents)}
}

func dirInfo(name string) os.FileInfo {
	return mapFI{name: pathpkg.Base(name), dir: true}
}

// Lstat returns a FileInfo describing the named file.
func (fs *mapFS) Lstat(ctx context.Context, path string) (os.FileInfo, error) {
	path = filename(path)

	fs.mu.RLock()
	defer fs.mu.RUnlock()

	b, ok := fs.m[path]
	if ok {
		return fileInfo(path, b), nil
	}
	pathslash := path + "/"
	if _, ok := fs.m[pathslash]; ok {
		return dirInfo(path), nil
	}
	for fn := range fs.m {
		if strings.HasPrefix(fn, pathslash) {
			return dirInfo(path), nil
		}
	}
	return nil, &os.PathError{
		Op:   "stat",
		Path: filename(path),
		Err:  os.ErrNotExist,
	}
}

// Stat returns a FileInfo describing the named file. If there is an error, it will be of type *PathError.
func (fs *mapFS) Stat(ctx context.Context, path string) (os.FileInfo, error) {
	return fs.Lstat(ctx, path)
}

// slashdir returns path.Dir(p), but special-cases paths not beginning
// with a slash to be in the root.
func slashdir(p string) string {
	d := pathpkg.Dir(p)
	if d == "." {
		return "/"
	}
	if strings.HasPrefix(p, "/") {
		return d
	}
	return "/" + d
}

func (fs *mapFS) readDir(path string) ([]string, map[string]os.FileInfo) {
	var ents []string
	fim := make(map[string]os.FileInfo) // base -> fi

	for fn, b := range fs.m {
		dir := slashdir(fn)
		isFile := true
		var lastBase string
		for {
			if dir == path {
				base := lastBase
				if isFile {
					base = pathpkg.Base(fn)
				}
				if fim[base] == nil {
					var fi os.FileInfo
					if isFile {
						fi = fileInfo(fn, b)
					} else {
						fi = dirInfo(base)
					}
					ents = append(ents, base)
					fim[base] = fi
				}
			}
			if dir == "/" {
				break
			} else {
				isFile = false
				lastBase = pathpkg.Base(dir)
				dir = pathpkg.Dir(dir)
			}
		}
	}
	return ents, fim
}

// ReadDir reads the contents of the directory.
func (fs *mapFS) ReadDir(ctx context.Context, path string) ([]os.FileInfo, error) {
	path = pathpkg.Clean(path)

	fs.mu.RLock()
	defer fs.mu.RUnlock()

	ents, fim := fs.readDir(path)
	if len(ents) == 0 {
		return nil, &os.PathError{
			Op:   "readdir",
			Path: filename(path),
			Err:  os.ErrNotExist,
		}
	}

	sort.Strings(ents)
	var list []os.FileInfo
	for _, dir := range ents {
		list = append(list, fim[dir])
	}
	return list, nil
}

// Create creates the named file, truncating it if it already exists.
func (fs *mapFS) Create(ctx context.Context, name string) (io.WriteCloser, error) {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	name = filename(name)
	fs.m[name] = ""
	return &writer{
		name: name,
		fs:   fs,
	}, nil
}

// ErrTooLarge is passed to panic if memory cannot be allocated to store data in a buffer.
var ErrTooLarge = errors.New("mapfs: too large")

type writer struct {
	strings.Builder
	name string
	fs   *mapFS
}

func (w *writer) Close() error {
	w.fs.mu.Lock()
	defer w.fs.mu.Unlock()
	w.fs.m[w.name] = w.Builder.String()
	return nil
}

// Mkdir creates a new directory. If name is already a directory, Mkdir
// returns an error (that can be detected using os.IsExist).
func (fs *mapFS) Mkdir(ctx context.Context, name string) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	name = filename(name)
	if _, ok := fs.m[name]; ok {
		return &os.PathError{
			Op:   "mkdir",
			Path: filename(name),
			Err:  os.ErrExist,
		}
	}
	nameslash := name + "/"
	if _, ok := fs.m[nameslash]; ok {
		return &os.PathError{
			Op:   "mkdir",
			Path: filename(name),
			Err:  os.ErrExist,
		}
	}
	for fn := range fs.m {
		if strings.HasPrefix(fn, nameslash) {
			return &os.PathError{
				Op:   "mkdir",
				Path: filename(name),
				Err:  os.ErrExist,
			}
		}
	}
	fs.m[name+"/"] = ""
	return nil
}

// Remove removes the named file or (empty) directory.
func (fs *mapFS) Remove(ctx context.Context, name string) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	name = filename(name)

	// try to remove file
	if _, ok := fs.m[name]; ok {
		delete(fs.m, name)
		dir := pathpkg.Dir(name)
		if dir != "." {
			fs.m[dir+"/"] = ""
		}
		return nil
	}

	// try to remove directory
	nameslash := name + "/"
	for fn := range fs.m {
		if fn != nameslash && strings.HasPrefix(fn, nameslash) {
			return &os.PathError{
				Op:   "remove",
				Path: name,
				Err:  errors.New("directory is not empty"),
			}
		}
	}
	if _, ok := fs.m[nameslash]; ok {
		delete(fs.m, nameslash)
		dir := pathpkg.Dir(name)
		if dir != "." {
			fs.m[dir+"/"] = ""
		}
		return nil
	}
	return &os.PathError{
		Op:   "remove",
		Path: name,
		Err:  os.ErrNotExist,
	}
}

// mapFI is the map-based implementation of FileInfo.
type mapFI struct {
	name string
	size int
	dir  bool
}

func (fi mapFI) IsDir() bool        { return fi.dir }
func (fi mapFI) ModTime() time.Time { return time.Time{} }
func (fi mapFI) Mode() os.FileMode {
	if fi.IsDir() {
		return 0755 | os.ModeDir
	}
	return 0444
}
func (fi mapFI) Name() string     { return pathpkg.Base(fi.name) }
func (fi mapFI) Size() int64      { return int64(fi.size) }
func (fi mapFI) Sys() interface{} { return nil }

type nopCloser struct {
	io.ReadSeeker
}

func (nc nopCloser) Close() error { return nil }
