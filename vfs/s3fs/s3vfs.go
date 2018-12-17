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

type dirinfo struct {
	path string
}

func (h dirinfo) Name() string {
	return pathpkg.Base(h.path)
}

func (h dirinfo) Size() int64 {
	return 0
}
func (h dirinfo) Mode() os.FileMode {
	return 0755 | os.ModeDir
}
func (h dirinfo) ModTime() time.Time {
	return time.Time{}
}

func (h dirinfo) IsDir() bool {
	return true
}

func (h dirinfo) Sys() interface{} {
	return nil
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
		// Search directory
		req := svc.ListObjectsV2Request(&s3.ListObjectsV2Input{
			Bucket:    aws.String(fs.Bucket),
			Prefix:    aws.String(pathpkg.Join(fs.Prefix, path) + "/"),
			Delimiter: aws.String("/"),
			MaxKeys:   aws.Int64(1),
		})
		req.SetContext(ctx)
		resp, err2 := req.Send()
		if err2 == nil && aws.Int64Value(resp.KeyCount) != 0 {
			return dirinfo{path: path}, nil
		}

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

type object struct {
	obj s3.Object
}

func (obj object) Name() string {
	return pathpkg.Base(aws.StringValue(obj.obj.Key))
}

func (obj object) Size() int64 {
	return aws.Int64Value(obj.obj.Size)
}
func (obj object) Mode() os.FileMode {
	return 0444
}
func (obj object) ModTime() time.Time {
	return aws.TimeValue(obj.obj.LastModified)
}

func (obj object) IsDir() bool {
	return false
}

func (obj object) Sys() interface{} {
	return obj.obj
}

type commonPrefix struct {
	prefix s3.CommonPrefix
}

func (p commonPrefix) Name() string {
	return pathpkg.Base(strings.TrimSuffix(aws.StringValue(p.prefix.Prefix), "/"))
}

func (p commonPrefix) Size() int64 {
	return 0
}
func (p commonPrefix) Mode() os.FileMode {
	return 0755 | os.ModeDir
}
func (p commonPrefix) ModTime() time.Time {
	return time.Time{}
}

func (p commonPrefix) IsDir() bool {
	return true
}

func (p commonPrefix) Sys() interface{} {
	return p.prefix
}

// ReadDir reads the contents of the directory.
func (fs *FileSystem) ReadDir(ctx context.Context, path string) ([]os.FileInfo, error) {
	svc := fs.s3()
	path = filename(path)
	prefix := strings.TrimPrefix(pathpkg.Join(fs.Prefix, path)+"/", "/")
	req := svc.ListObjectsV2Request(&s3.ListObjectsV2Input{
		Bucket:    aws.String(fs.Bucket),
		Prefix:    aws.String(prefix),
		Delimiter: aws.String("/"),
	})
	req.SetContext(ctx)
	resp, err := req.Send()
	if err != nil {
		if err, ok := err.(awserr.RequestFailure); ok {
			switch err.StatusCode() {
			case http.StatusNotFound:
				return nil, &os.PathError{
					Op:   "readdir",
					Path: path,
					Err:  os.ErrNotExist,
				}
			case http.StatusForbidden:
				return nil, &os.PathError{
					Op:   "readdir",
					Path: path,
					Err:  os.ErrPermission,
				}
			}
			return nil, err
		}
		return nil, err
	}

	// merge Contents and CommonPrefixes
	contents := resp.Contents
	prefixes := resp.CommonPrefixes
	res := make([]os.FileInfo, 0, len(contents)+len(prefixes))
	for len(contents) > 0 && len(prefixes) > 0 {
		if aws.StringValue(contents[0].Key) < aws.StringValue(prefixes[0].Prefix) {
			res = append(res, object{contents[0]})
			contents = contents[1:]
		} else {
			res = append(res, commonPrefix{prefixes[0]})
			prefixes = prefixes[1:]
		}
	}
	for _, v := range contents {
		res = append(res, object{v})
	}
	for _, v := range prefixes {
		res = append(res, commonPrefix{v})
	}
	return res, nil
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
