package s3fs

import (
	"context"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	pathpkg "path"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/awserr"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/s3iface"
	"github.com/shogo82148/s3ftpgateway/vfs"
)

// FileSystem implements ctxvfs.FileSystem
type FileSystem struct {
	Config aws.Config
	Bucket string
	Prefix string

	mu    sync.Mutex
	s3api s3iface.S3API
}

func filename(p string) string {
	return strings.TrimPrefix(pathpkg.Clean(p), "/")
}

func (fs *FileSystem) s3() s3iface.S3API {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	if fs.s3api == nil {
		fs.s3api = s3.New(fs.Config)
	}
	return fs.s3api
}

type file struct {
	*os.File
}

func (f *file) Close() error {
	err := f.File.Close()
	os.Remove(f.File.Name())
	return err
}

// Open opens the file.
func (fs *FileSystem) Open(ctx context.Context, name string) (vfs.ReadSeekCloser, error) {
	svc := fs.s3()
	name = filename(name)
	req := svc.GetObjectRequest(&s3.GetObjectInput{
		Bucket: aws.String(fs.Bucket),
		Key:    aws.String(pathpkg.Join(fs.Prefix, name)),
	})
	req.SetContext(ctx)
	resp, err := req.Send()
	if err != nil {
		if err, ok := err.(awserr.RequestFailure); ok {
			switch err.StatusCode() {
			case http.StatusNotFound:
				return nil, &os.PathError{
					Op:   "open",
					Path: name,
					Err:  os.ErrNotExist,
				}
			case http.StatusForbidden:
				return nil, &os.PathError{
					Op:   "open",
					Path: name,
					Err:  os.ErrPermission,
				}
			}
			return nil, err
		}
		return nil, err
	}
	defer resp.Body.Close()

	tmp, err := ioutil.TempFile("", "s3fs_")
	if err != nil {
		return nil, err
	}
	f := &file{
		File: tmp,
	}
	defer func() {
		if err != nil {
			f.Close()
		}
	}()

	if _, err := io.Copy(tmp, resp.Body); err != nil {
		return nil, err
	}
	if _, err := tmp.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	return f, nil
}

type headoutput struct {
	path string
	resp *s3.HeadObjectOutput
}

func (h headoutput) Name() string {
	return pathpkg.Base(h.path)
}

func (h headoutput) Size() int64 {
	return aws.Int64Value(h.resp.ContentLength)
}
func (h headoutput) Mode() os.FileMode {
	return 0444
}
func (h headoutput) ModTime() time.Time {
	return aws.TimeValue(h.resp.LastModified)
}

func (h headoutput) IsDir() bool {
	return false
}

func (h headoutput) Sys() interface{} {
	return h.resp
}

// Lstat returns a FileInfo describing the named file.
func (fs *FileSystem) Lstat(ctx context.Context, path string) (os.FileInfo, error) {
	svc := fs.s3()
	path = filename(path)
	req := svc.HeadObjectRequest(&s3.HeadObjectInput{
		Bucket: aws.String(fs.Bucket),
		Key:    aws.String(pathpkg.Join(fs.Prefix, path)),
	})
	req.SetContext(ctx)
	resp, err := req.Send()
	if err != nil {
		if err, ok := err.(awserr.RequestFailure); ok {
			switch err.StatusCode() {
			case http.StatusNotFound:
				return nil, &os.PathError{
					Op:   "stat",
					Path: path,
					Err:  os.ErrNotExist,
				}
			case http.StatusForbidden:
				return nil, &os.PathError{
					Op:   "stat",
					Path: path,
					Err:  os.ErrPermission,
				}
			}
			return nil, err
		}
		return nil, err
	}
	return headoutput{
		path: path,
		resp: resp,
	}, nil
}

// Stat returns a FileInfo describing the named file. If there is an error, it will be of type *PathError.
func (fs *FileSystem) Stat(ctx context.Context, path string) (os.FileInfo, error) {
	return fs.Lstat(ctx, path)
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
	return "s3vf"
}
