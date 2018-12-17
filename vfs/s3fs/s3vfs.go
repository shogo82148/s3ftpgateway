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

// filekey converts the name to the key value on the S3 bucket.
func (fs *FileSystem) filekey(name string) string {
	name = pathpkg.Clean("/" + name)
	return strings.TrimPrefix(pathpkg.Join(fs.Prefix, name), "/")
}

// dirkey converts the name to the key value for directries on the S3 bucket.
func (fs *FileSystem) dirkey(name string) string {
	name = fs.filekey(name)
	if name == "" {
		return ""
	}
	return name + "/"
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

type fileReader struct {
	*os.File
}

func (f *fileReader) Close() error {
	err := f.File.Close()
	os.Remove(f.File.Name())
	return err
}

// Open opens the file.
func (fs *FileSystem) Open(ctx context.Context, name string) (vfs.ReadSeekCloser, error) {
	svc := fs.s3()
	req := svc.GetObjectRequest(&s3.GetObjectInput{
		Bucket: aws.String(fs.Bucket),
		Key:    aws.String(fs.filekey(name)),
	})
	req.SetContext(ctx)
	resp, err := req.Send()
	if err != nil {
		if err, ok := err.(awserr.RequestFailure); ok {
			switch err.StatusCode() {
			case http.StatusNotFound:
				return nil, &os.PathError{
					Op:   "open",
					Path: filename(name),
					Err:  os.ErrNotExist,
				}
			case http.StatusForbidden:
				return nil, &os.PathError{
					Op:   "open",
					Path: filename(name),
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
	f := &fileReader{
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

// Lstat returns a FileInfo describing the named file.
func (fs *FileSystem) Lstat(ctx context.Context, path string) (os.FileInfo, error) {
	if path == "" {
		return commonPrefix{s3.CommonPrefix{
			Prefix: aws.String(""),
		}}, nil
	}

	svc := fs.s3()
	file := fs.filekey(path)
	req := svc.ListObjectsV2Request(&s3.ListObjectsV2Input{
		Bucket:    aws.String(fs.Bucket),
		Prefix:    aws.String(file),
		Delimiter: aws.String("/"),
		MaxKeys:   aws.Int64(1),
	})
	req.SetContext(ctx)
	resp, err := req.Send()
	if err != nil {
		if err, ok := err.(awserr.RequestFailure); ok {
			switch err.StatusCode() {
			case http.StatusNotFound:
				return nil, &os.PathError{
					Op:   "stat",
					Path: filename(path),
					Err:  os.ErrNotExist,
				}
			case http.StatusForbidden:
				return nil, &os.PathError{
					Op:   "stat",
					Path: filename(path),
					Err:  os.ErrPermission,
				}
			}
			return nil, err
		}
		return nil, err
	}
	if len(resp.CommonPrefixes) > 0 && aws.StringValue(resp.CommonPrefixes[0].Prefix) == file+"/" {
		return commonPrefix{resp.CommonPrefixes[0]}, nil
	}
	if len(resp.Contents) > 0 && aws.StringValue(resp.Contents[0].Key) == file {
		return object{resp.Contents[0]}, nil
	}
	return nil, &os.PathError{
		Op:   "stat",
		Path: filename(path),
		Err:  os.ErrNotExist,
	}
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

// max-keys for test
var maxKeys = int64(1000)

// ReadDir reads the contents of the directory.
func (fs *FileSystem) ReadDir(ctx context.Context, path string) ([]os.FileInfo, error) {
	svc := fs.s3()
	req := svc.ListObjectsV2Request(&s3.ListObjectsV2Input{
		Bucket:    aws.String(fs.Bucket),
		Prefix:    aws.String(fs.dirkey(path)),
		Delimiter: aws.String("/"),
		MaxKeys:   &maxKeys,
	})
	req.SetContext(ctx)
	pager := req.Paginate()
	res := []os.FileInfo{}
	for pager.Next() {
		// merge Contents and CommonPrefixes
		resp := pager.CurrentPage()
		contents := resp.Contents
		prefixes := resp.CommonPrefixes
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
	}
	if err := pager.Err(); err != nil {
		if err, ok := err.(awserr.RequestFailure); ok {
			switch err.StatusCode() {
			case http.StatusNotFound:
				return nil, &os.PathError{
					Op:   "readdir",
					Path: filename(path),
					Err:  os.ErrNotExist,
				}
			case http.StatusForbidden:
				return nil, &os.PathError{
					Op:   "readdir",
					Path: filename(path),
					Err:  os.ErrPermission,
				}
			}
			return nil, err
		}
		return nil, err
	}
	return res, nil
}

type fileWriter struct {
	*os.File
	ctx  context.Context
	fs   *FileSystem
	name string
}

func (f *fileWriter) Close() error {
	defer os.Remove(f.File.Name())

	if _, err := f.File.Seek(0, io.SeekStart); err != nil {
		f.File.Close()
		return err
	}
	svc := f.fs.s3()
	req := svc.PutObjectRequest(&s3.PutObjectInput{
		Bucket: aws.String(f.fs.Bucket),
		Key:    aws.String(f.fs.filekey(f.name)),
		Body:   f.File,
	})
	req.SetContext(f.ctx)
	if _, err := req.Send(); err != nil {
		f.File.Close()
		return err
	}
	return f.File.Close()
}

// Create creates the named file, truncating it if it already exists.
func (fs *FileSystem) Create(ctx context.Context, name string) (vfs.WriteSeekCloser, error) {
	stat, err := fs.Lstat(ctx, name)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
	} else if stat.IsDir() {
		return nil, &os.PathError{
			Op:   "create",
			Path: filename(name),
			Err:  os.ErrExist,
		}
	}

	tmp, err := ioutil.TempFile("", "s3fs_")
	if err != nil {
		return nil, err
	}
	f := &fileWriter{
		File: tmp,
		ctx:  ctx,
		fs:   fs,
		name: name,
	}
	return f, nil
}

// Mkdir creates a new directory. If name is already a directory, Mkdir
// returns an error (that can be detected using os.IsExist).
func (fs *FileSystem) Mkdir(ctx context.Context, name string) error {
	_, err := fs.Lstat(ctx, name)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
	} else {
		return &os.PathError{
			Op:   "create",
			Path: filename(name),
			Err:  os.ErrExist,
		}
	}

	svc := fs.s3()
	req := svc.PutObjectRequest(&s3.PutObjectInput{
		Bucket: aws.String(fs.Bucket),
		Key:    aws.String(fs.dirkey(name)),
		Body:   strings.NewReader(""),
	})
	req.SetContext(ctx)
	if _, err := req.Send(); err != nil {
		return err
	}
	return nil
}

// Remove removes the named file or directory.
func (fs *FileSystem) Remove(ctx context.Context, name string) error {
	return nil
}

func (fs *FileSystem) String() string {
	return "s3vf"
}
