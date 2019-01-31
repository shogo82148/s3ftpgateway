package s3fs

import (
	"context"
	"errors"
	"io"
	"mime"
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
	"github.com/aws/aws-sdk-go-v2/service/s3/s3manager"
	"github.com/aws/aws-sdk-go-v2/service/s3/s3manager/s3manageriface"
)

// FileSystem implements ctxvfs.FileSystem
type FileSystem struct {
	Config aws.Config
	Bucket string
	Prefix string

	mu          sync.Mutex
	s3api       s3iface.S3API
	uploaderapi s3manageriface.UploaderAPI
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
		cfg := fs.Config
		if cfg.Region == "" {
			region, err := s3manager.GetBucketRegion(context.Background(), fs.Config, fs.Bucket, "us-west-2")
			if err != nil {
				panic(err)
			}
			cfg = cfg.Copy()
			cfg.Region = region
		}
		fs.s3api = s3.New(cfg)
	}
	return fs.s3api
}

func (fs *FileSystem) uploader() s3manageriface.UploaderAPI {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	if fs.uploaderapi == nil {
		cfg := fs.Config
		if cfg.Region == "" {
			region, err := s3manager.GetBucketRegion(context.Background(), fs.Config, fs.Bucket, "us-west-2")
			if err != nil {
				panic(err)
			}
			cfg = cfg.Copy()
			cfg.Region = region
		}
		fs.uploaderapi = s3manager.NewUploader(cfg)
	}
	return fs.uploaderapi
}

// Open opens the file.
func (fs *FileSystem) Open(ctx context.Context, name string) (io.ReadCloser, error) {
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
		}
		return nil, &os.PathError{
			Op:   "open",
			Path: filename(name),
			Err:  err,
		}
	}
	return resp.Body, nil
}

// Lstat returns a FileInfo describing the named file.
func (fs *FileSystem) Lstat(ctx context.Context, path string) (os.FileInfo, error) {
	// root is always exists.
	if path == "" || path == "/" {
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
		}
		return nil, &os.PathError{
			Op:   "stat",
			Path: filename(path),
			Err:  err,
		}
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
	return 0644
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
		}
		return nil, &os.PathError{
			Op:   "readdir",
			Path: filename(path),
			Err:  err,
		}
	}
	return res, nil
}

// Create creates the named file, truncating it if it already exists.
func (fs *FileSystem) Create(ctx context.Context, name string, body io.Reader) error {
	stat, err := fs.Lstat(ctx, name)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
	} else if stat.IsDir() {
		return &os.PathError{
			Op:   "create",
			Path: filename(name),
			Err:  os.ErrExist,
		}
	}

	ext := pathpkg.Ext(name)
	typ := mime.TypeByExtension(ext)
	if typ == "" {
		typ = "application/octet-stream"
	}

	svc := fs.uploader()
	_, err = svc.UploadWithContext(ctx, &s3manager.UploadInput{
		Bucket:      aws.String(fs.Bucket),
		Key:         aws.String(fs.filekey(name)),
		Body:        body,
		ContentType: aws.String(typ),
	})
	if err != nil {
		return &os.PathError{
			Op:   "create",
			Path: filename(name),
			Err:  err,
		}
	}
	return nil
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
		return &os.PathError{
			Op:   "mkdir",
			Path: filename(name),
			Err:  err,
		}
	}
	return nil
}

// Remove removes the named file or (empty) directory.
func (fs *FileSystem) Remove(ctx context.Context, name string) error {
	// the file or directory is exists?
	stat, err := fs.Lstat(ctx, name)
	if err != nil {
		return err
	}
	svc := fs.s3()
	if stat.IsDir() {
		// the directory is empty?
		req := svc.ListObjectsV2Request(&s3.ListObjectsV2Input{
			Bucket:  aws.String(fs.Bucket),
			Prefix:  aws.String(fs.dirkey(name)),
			MaxKeys: aws.Int64(1),
		})
		req.SetContext(ctx)
		resp, err := req.Send()
		if err != nil {
			return &os.PathError{
				Op:   "remove",
				Path: filename(name),
				Err:  err,
			}
		}
		if aws.Int64Value(resp.KeyCount) != 0 {
			return &os.PathError{
				Op:   "remove",
				Path: filename(name),
				Err:  errors.New("directory is not empty"),
			}
		}
	}

	req := svc.DeleteObjectRequest(&s3.DeleteObjectInput{
		Bucket: aws.String(fs.Bucket),
		Key:    aws.String(fs.filekey(name)),
	})
	req.SetContext(ctx)
	if _, err := req.Send(); err != nil {
		return &os.PathError{
			Op:   "remove",
			Path: filename(name),
			Err:  err,
		}
	}
	return nil
}

func (fs *FileSystem) String() string {
	return "s3vf"
}
