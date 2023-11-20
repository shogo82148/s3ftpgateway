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
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

type s3client interface {
	manager.HeadBucketAPIClient
	manager.UploadAPIClient
	DeleteObject(ctx context.Context, params *s3.DeleteObjectInput, optFns ...func(*s3.Options)) (*s3.DeleteObjectOutput, error)
	GetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error)
	ListObjectsV2(ctx context.Context, params *s3.ListObjectsV2Input, optFns ...func(*s3.Options)) (*s3.ListObjectsV2Output, error)
}
type uploaderClient interface {
	Upload(ctx context.Context, input *s3.PutObjectInput, opts ...func(*manager.Uploader)) (*manager.UploadOutput, error)
}

// FileSystem implements ctxvfs.FileSystem
type FileSystem struct {
	Config aws.Config
	Bucket string
	Prefix string

	mu          sync.Mutex
	s3api       s3client
	uploaderapi uploaderClient
}

// filekey converts the name to the key value on the S3 bucket.
func (fs *FileSystem) filekey(name string) string {
	name = pathpkg.Clean("/" + name)
	return strings.TrimPrefix(pathpkg.Join(fs.Prefix, name), "/")
}

// dirkey converts the name to the key value for directories on the S3 bucket.
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

func (fs *FileSystem) s3() s3client {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	if fs.s3api == nil {
		cfg := fs.Config
		if cfg.Region == "" {
			region, err := manager.GetBucketRegion(context.TODO(), nil, fs.Bucket)
			if err != nil {
				panic(err)
			}
			cfg = cfg.Copy()
			cfg.Region = region
		}
		fs.s3api = s3.NewFromConfig(cfg)
	}
	return fs.s3api
}

func (fs *FileSystem) uploader() uploaderClient {
	s3 := fs.s3()
	fs.mu.Lock()
	defer fs.mu.Unlock()

	if fs.uploaderapi == nil {
		fs.uploaderapi = manager.NewUploader(s3)
	}
	return fs.uploaderapi
}

// Open opens the file.
func (fs *FileSystem) Open(ctx context.Context, name string) (io.ReadCloser, error) {
	svc := fs.s3()
	resp, err := svc.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(fs.Bucket),
		Key:    aws.String(fs.filekey(name)),
	})
	if err != nil {
		var respErr *awshttp.ResponseError
		if errors.As(err, &respErr) {
			switch respErr.HTTPStatusCode() {
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
		return commonPrefix{types.CommonPrefix{
			Prefix: aws.String(""),
		}}, nil
	}

	svc := fs.s3()
	file := fs.filekey(path)
	resp, err := svc.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket:    aws.String(fs.Bucket),
		Prefix:    aws.String(file),
		Delimiter: aws.String("/"),
		MaxKeys:   aws.Int32(1),
	})
	if err != nil {
		var respErr *awshttp.ResponseError
		if errors.As(err, &respErr) {
			switch respErr.HTTPStatusCode() {
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
	if len(resp.CommonPrefixes) > 0 && aws.ToString(resp.CommonPrefixes[0].Prefix) == file+"/" {
		return commonPrefix{resp.CommonPrefixes[0]}, nil
	}
	if len(resp.Contents) > 0 && aws.ToString(resp.Contents[0].Key) == file {
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
	obj types.Object
}

func (obj object) Name() string {
	return pathpkg.Base(aws.ToString(obj.obj.Key))
}

func (obj object) Size() int64 {
	return aws.ToInt64(obj.obj.Size)
}
func (obj object) Mode() os.FileMode {
	return 0644
}
func (obj object) ModTime() time.Time {
	return aws.ToTime(obj.obj.LastModified)
}

func (obj object) IsDir() bool {
	return false
}

func (obj object) Sys() interface{} {
	return obj.obj
}

type commonPrefix struct {
	prefix types.CommonPrefix
}

func (p commonPrefix) Name() string {
	return pathpkg.Base(strings.TrimSuffix(aws.ToString(p.prefix.Prefix), "/"))
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
var maxKeys = int32(1000)

// ReadDir reads the contents of the directory.
func (fs *FileSystem) ReadDir(ctx context.Context, path string) ([]os.FileInfo, error) {
	svc := fs.s3()
	paginator := s3.NewListObjectsV2Paginator(svc, &s3.ListObjectsV2Input{
		Bucket:    aws.String(fs.Bucket),
		Prefix:    aws.String(fs.dirkey(path)),
		Delimiter: aws.String("/"),
		MaxKeys:   aws.Int32(maxKeys),
	})
	res := []os.FileInfo{}
	for paginator.HasMorePages() {
		// merge Contents and CommonPrefixes
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		contents := page.Contents
		prefixes := page.CommonPrefixes
		for len(contents) > 0 && len(prefixes) > 0 {
			if aws.ToString(contents[0].Key) < aws.ToString(prefixes[0].Prefix) {
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
	// TODO error handling
	// if err := pager.Err(); err != nil {
	// 	if err, ok := err.(awserr.RequestFailure); ok {
	// 		switch err.StatusCode() {
	// 		case http.StatusNotFound:
	// 			return nil, &os.PathError{
	// 				Op:   "readdir",
	// 				Path: filename(path),
	// 				Err:  os.ErrNotExist,
	// 			}
	// 		case http.StatusForbidden:
	// 			return nil, &os.PathError{
	// 				Op:   "readdir",
	// 				Path: filename(path),
	// 				Err:  os.ErrPermission,
	// 			}
	// 		}
	// 	}
	// 	return nil, &os.PathError{
	// 		Op:   "readdir",
	// 		Path: filename(path),
	// 		Err:  err,
	// 	}
	// }
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
	_, err = svc.Upload(ctx, &s3.PutObjectInput{
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
	_, err = svc.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(fs.Bucket),
		Key:    aws.String(fs.dirkey(name)),
		Body:   strings.NewReader(""),
	})
	if err != nil {
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
		resp, err := svc.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
			Bucket:  aws.String(fs.Bucket),
			Prefix:  aws.String(fs.dirkey(name)),
			MaxKeys: aws.Int32(1),
		})
		if err != nil {
			return &os.PathError{
				Op:   "remove",
				Path: filename(name),
				Err:  err,
			}
		}
		if aws.ToInt32(resp.KeyCount) != 0 {
			return &os.PathError{
				Op:   "remove",
				Path: filename(name),
				Err:  errors.New("directory is not empty"),
			}
		}
	}

	_, err = svc.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(fs.Bucket),
		Key:    aws.String(fs.filekey(name)),
	})
	if err != nil {
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
