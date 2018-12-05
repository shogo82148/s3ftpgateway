package s3fs

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/external"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/shogo82148/s3ftpgateway/vfs"
)

var _ vfs.FileSystem = &FileSystem{}

func newTestFileSystem(t *testing.T) (*FileSystem, func()) {
	bucket := os.Getenv("S3FS_TEST_BUCKET")
	if bucket == "" {
		t.Skip("S3FS_TEST_BUCKET is not set, skipped")
		return nil, func() {}
	}

	cfg, err := external.LoadDefaultAWSConfig()
	if err != nil {
		t.Error(err)
		return nil, func() {}
	}

	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		t.Error(err)
		return nil, func() {}
	}
	prefix := hex.EncodeToString(buf[:])

	fs := &FileSystem{
		Config: cfg,
		Bucket: bucket,
		Prefix: prefix,
	}

	return fs, func() {}
}

func TestOpen(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fs, cleanup := newTestFileSystem(t)
	defer cleanup()

	t.Run("not-found", func(t *testing.T) {
		_, err := fs.Open(ctx, "not-fount")
		if err == nil || !os.IsNotExist(err) {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("found", func(t *testing.T) {
		req := fs.s3().PutObjectRequest(&s3.PutObjectInput{
			Bucket: aws.String(fs.Bucket),
			Key:    aws.String(fmt.Sprintf("%s/foo.txt", fs.Prefix)),
			Body:   strings.NewReader("abc123"),
		})
		req.SetContext(ctx)
		if _, err := req.Send(); err != nil {
			t.Error(err)
			return
		}
		f, err := fs.Open(ctx, "foo.txt")
		if err != nil {
			t.Error(err)
			return
		}
		defer f.Close()
		b, err := ioutil.ReadAll(f)
		if err != nil {
			t.Error(err)
			return
		}
		if string(b) != "abc123" {
			t.Errorf("want abc123, got %s", string(b))
		}
	})
}

func TestLstat(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fs, cleanup := newTestFileSystem(t)
	defer cleanup()

	t.Run("not-found", func(t *testing.T) {
		_, err := fs.Lstat(ctx, "not-fount")
		if err == nil || !os.IsNotExist(err) {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("found-file", func(t *testing.T) {
		req := fs.s3().PutObjectRequest(&s3.PutObjectInput{
			Bucket: aws.String(fs.Bucket),
			Key:    aws.String(fmt.Sprintf("%s/foo.txt", fs.Prefix)),
			Body:   strings.NewReader("abc123"),
		})
		req.SetContext(ctx)
		if _, err := req.Send(); err != nil {
			t.Error(err)
			return
		}
		info, err := fs.Lstat(ctx, "foo.txt")
		if err != nil {
			t.Error(err)
			return
		}
		if info.Name() != "foo.txt" {
			t.Errorf("want foo.txt, got %s", info.Name())
		}
		if info.Size() != 6 {
			t.Errorf("want 6, got %d", info.Size())
		}
		if info.IsDir() {
			t.Error("want not dirctory, but it is")
		}
	})

	t.Run("found-dir", func(t *testing.T) {
		req := fs.s3().PutObjectRequest(&s3.PutObjectInput{
			Bucket: aws.String(fs.Bucket),
			Key:    aws.String(fmt.Sprintf("%s/bar/foo.txt", fs.Prefix)),
			Body:   strings.NewReader("abc123"),
		})
		req.SetContext(ctx)
		if _, err := req.Send(); err != nil {
			t.Error(err)
			return
		}
		info, err := fs.Lstat(ctx, "bar")
		if err != nil {
			t.Error(err)
			return
		}
		if info.Name() != "bar" {
			t.Errorf("want foo.txt, got %s", info.Name())
		}
		if !info.IsDir() {
			t.Error("want dirctory, but it is not")
		}
	})
}
