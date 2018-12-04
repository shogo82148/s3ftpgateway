package s3fs

import (
	"context"
	"log"
	"net/http"
	"os"
	pathpkg "path"
	"strings"
	"sync"

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
	log.Println(resp)
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
	return "s3vf"
}
