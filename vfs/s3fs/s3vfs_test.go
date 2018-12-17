package s3fs

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
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

func TestReadDir(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fs, cleanup := newTestFileSystem(t)
	defer cleanup()

	maxKeys = 1
	t.Run("simple", func(t *testing.T) {
		// add test objects
		req := fs.s3().PutObjectRequest(&s3.PutObjectInput{
			Bucket: aws.String(fs.Bucket),
			Key:    aws.String(fmt.Sprintf("%s/bar/foo1.txt", fs.Prefix)),
			Body:   strings.NewReader("abc123"),
		})
		req.SetContext(ctx)
		if _, err := req.Send(); err != nil {
			t.Error(err)
			return
		}
		req = fs.s3().PutObjectRequest(&s3.PutObjectInput{
			Bucket: aws.String(fs.Bucket),
			Key:    aws.String(fmt.Sprintf("%s/bar/foo2.txt", fs.Prefix)),
			Body:   strings.NewReader("abc123"),
		})
		req.SetContext(ctx)
		if _, err := req.Send(); err != nil {
			t.Error(err)
			return
		}
		req = fs.s3().PutObjectRequest(&s3.PutObjectInput{
			Bucket: aws.String(fs.Bucket),
			Key:    aws.String(fmt.Sprintf("%s/foobar.txt", fs.Prefix)),
			Body:   strings.NewReader("abc123"),
		})
		req.SetContext(ctx)
		if _, err := req.Send(); err != nil {
			t.Error(err)
			return
		}

		list, err := fs.ReadDir(ctx, "")
		if err != nil {
			t.Error(err)
			return
		}
		if len(list) != 2 {
			t.Fatalf("want 2, got %d", len(list))
		}
		if list[0].Name() != "bar" {
			t.Errorf("want bar, got %s", list[0].Name())
		}
		if list[1].Name() != "foobar.txt" {
			t.Errorf("want foobar.txt, got %s", list[1].Name())
		}
	})
}

func TestCreate(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fs, cleanup := newTestFileSystem(t)
	defer cleanup()

	w, err := fs.Create(ctx, "foobar.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(w, "abc123"); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	req := fs.s3().GetObjectRequest(&s3.GetObjectInput{
		Bucket: aws.String(fs.Bucket),
		Key:    aws.String(fmt.Sprintf("%s/foobar.txt", fs.Prefix)),
	})
	req.SetContext(ctx)
	resp, err := req.Send()
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	ret, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(ret) != "abc123" {
		t.Errorf("want abc123, got %s", string(ret))
	}
}
