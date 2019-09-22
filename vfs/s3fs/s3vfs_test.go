package s3fs

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"sync"
	"testing"
	"testing/iotest"

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
	svc := s3.New(cfg)

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

	var wg sync.WaitGroup
	chDel := make(chan string, 5)
	wg.Add(1)
	go func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		for key := range chDel {
			req := svc.DeleteObjectRequest(&s3.DeleteObjectInput{
				Bucket: aws.String(bucket),
				Key:    aws.String(key),
			})
			req.Send(ctx)
		}
		wg.Done()
	}()

	return fs, func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		req := svc.ListObjectsV2Request(&s3.ListObjectsV2Input{
			Bucket: aws.String(bucket),
			Prefix: aws.String(prefix + "/"),
		})
		pager := s3.NewListObjectsV2Paginator(req)
		for pager.Next(ctx) {
			resp := pager.CurrentPage()
			for _, v := range resp.Contents {
				chDel <- aws.StringValue(v.Key)
			}
		}
		close(chDel)
		wg.Wait()
	}
}

func TestKey(t *testing.T) {
	cases := []struct {
		prefix string
		in     string
		file   string
		dir    string
	}{
		{"", "foobar.txt", "foobar.txt", "foobar.txt/"},
		{"", "/foobar.txt", "foobar.txt", "foobar.txt/"},
		{"", "", "", ""},
		{"", "../foobar.txt", "foobar.txt", "foobar.txt/"},

		{"abc123", "foobar.txt", "abc123/foobar.txt", "abc123/foobar.txt/"},
		{"abc123", "/foobar.txt", "abc123/foobar.txt", "abc123/foobar.txt/"},
		{"abc123", "", "abc123", "abc123/"},
		{"abc123", "../foobar.txt", "abc123/foobar.txt", "abc123/foobar.txt/"},
	}

	for i, c := range cases {
		fs := &FileSystem{
			Prefix: c.prefix,
		}
		file := fs.filekey(c.in)
		if file != c.file {
			t.Errorf("file %d: want %s, got %s", i, c.file, file)
		}
		dir := fs.dirkey(c.in)
		if dir != c.dir {
			t.Errorf("dir %d: want %s, got %s", i, c.dir, dir)
		}
	}
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
		if _, err := req.Send(ctx); err != nil {
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
		req := fs.s3().PutObjectRequest(&s3.PutObjectInput{
			Bucket: aws.String(fs.Bucket),
			Key:    aws.String(fmt.Sprintf("%s/not-found ", fs.Prefix)),
			Body:   strings.NewReader("abc123"),
		})
		if _, err := req.Send(ctx); err != nil {
			t.Error(err)
			return
		}
		req = fs.s3().PutObjectRequest(&s3.PutObjectInput{
			Bucket: aws.String(fs.Bucket),
			Key:    aws.String(fmt.Sprintf("%s/not-found  /", fs.Prefix)),
			Body:   strings.NewReader("abc123"),
		})
		if _, err := req.Send(ctx); err != nil {
			t.Error(err)
			return
		}

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
		if _, err := req.Send(ctx); err != nil {
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
			t.Error("want not directory, but it is")
		}
	})

	t.Run("found-dir", func(t *testing.T) {
		req := fs.s3().PutObjectRequest(&s3.PutObjectInput{
			Bucket: aws.String(fs.Bucket),
			Key:    aws.String(fmt.Sprintf("%s/bar/foo.txt", fs.Prefix)),
			Body:   strings.NewReader("abc123"),
		})
		if _, err := req.Send(ctx); err != nil {
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
			t.Error("want directory, but it is not")
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
		if _, err := req.Send(ctx); err != nil {
			t.Error(err)
			return
		}
		req = fs.s3().PutObjectRequest(&s3.PutObjectInput{
			Bucket: aws.String(fs.Bucket),
			Key:    aws.String(fmt.Sprintf("%s/bar/foo2.txt", fs.Prefix)),
			Body:   strings.NewReader("abc123"),
		})
		if _, err := req.Send(ctx); err != nil {
			t.Error(err)
			return
		}
		req = fs.s3().PutObjectRequest(&s3.PutObjectInput{
			Bucket: aws.String(fs.Bucket),
			Key:    aws.String(fmt.Sprintf("%s/foobar.txt", fs.Prefix)),
			Body:   strings.NewReader("abc123"),
		})
		if _, err := req.Send(ctx); err != nil {
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

	t.Run("success", func(t *testing.T) {
		fs, cleanup := newTestFileSystem(t)
		defer cleanup()

		err := fs.Create(ctx, "foobar.txt", strings.NewReader("abc123"))
		if err != nil {
			t.Fatal(err)
		}

		req := fs.s3().GetObjectRequest(&s3.GetObjectInput{
			Bucket: aws.String(fs.Bucket),
			Key:    aws.String(fmt.Sprintf("%s/foobar.txt", fs.Prefix)),
		})
		resp, err := req.Send(ctx)
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

		if aws.StringValue(resp.ContentType) != "text/plain; charset=utf-8" {
			t.Errorf("want text/plain; charset=utf-8, got %s", aws.StringValue(resp.ContentType))
		}
	})

	t.Run("timeout", func(t *testing.T) {
		fs, cleanup := newTestFileSystem(t)
		defer cleanup()

		body := iotest.TimeoutReader(strings.NewReader("foobar"))
		err := fs.Create(ctx, "foobar.txt", body)
		if err == nil {
			t.Errorf("want error, got %v", err)
		}

		if _, err := fs.Lstat(ctx, "foobar.txt"); err == nil || !os.IsNotExist(err) {
			t.Errorf("want NotExist, got %v", err)
		}
	})

	t.Run("exist", func(t *testing.T) {
		fs, cleanup := newTestFileSystem(t)
		defer cleanup()

		req := fs.s3().PutObjectRequest(&s3.PutObjectInput{
			Bucket: aws.String(fs.Bucket),
			Key:    aws.String(fmt.Sprintf("%s/foobar/hoge.txt", fs.Prefix)),
			Body:   strings.NewReader("abc123"),
		})
		if _, err := req.Send(ctx); err != nil {
			t.Error(err)
			return
		}

		body := strings.NewReader("abc123")
		if err := fs.Create(ctx, "foobar", body); err == nil || !os.IsExist(err) {
			t.Errorf("want ErrExist, got %s", err)
		}
	})
}

func TestMkdir(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	t.Run("success", func(t *testing.T) {
		fs, cleanup := newTestFileSystem(t)
		defer cleanup()

		if err := fs.Mkdir(ctx, "foobar"); err != nil {
			t.Fatal(err)
		}

		req := fs.s3().GetObjectRequest(&s3.GetObjectInput{
			Bucket: aws.String(fs.Bucket),
			Key:    aws.String(fmt.Sprintf("%s/foobar/", fs.Prefix)),
		})
		resp, err := req.Send(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
	})

	t.Run("exist", func(t *testing.T) {
		fs, cleanup := newTestFileSystem(t)
		defer cleanup()

		req := fs.s3().PutObjectRequest(&s3.PutObjectInput{
			Bucket: aws.String(fs.Bucket),
			Key:    aws.String(fmt.Sprintf("%s/foobar/hoge.txt", fs.Prefix)),
			Body:   strings.NewReader("abc123"),
		})
		if _, err := req.Send(ctx); err != nil {
			t.Error(err)
			return
		}

		if err := fs.Mkdir(ctx, "foobar"); err == nil || !os.IsExist(err) {
			t.Errorf("want ErrExist, got %s", err)
		}
	})
}

func TestRemove(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	t.Run("success", func(t *testing.T) {
		fs, cleanup := newTestFileSystem(t)
		defer cleanup()

		req := fs.s3().PutObjectRequest(&s3.PutObjectInput{
			Bucket: aws.String(fs.Bucket),
			Key:    aws.String(fmt.Sprintf("%s/foobar.txt", fs.Prefix)),
			Body:   strings.NewReader("abc123"),
		})
		if _, err := req.Send(ctx); err != nil {
			t.Error(err)
			return
		}

		if err := fs.Remove(ctx, "foobar.txt"); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("no file", func(t *testing.T) {
		fs, cleanup := newTestFileSystem(t)
		defer cleanup()

		if err := fs.Remove(ctx, "foobar.txt"); err == nil {
			t.Errorf("want error, got %s", err)
		}
	})

	t.Run("non empty dir", func(t *testing.T) {
		fs, cleanup := newTestFileSystem(t)
		defer cleanup()

		req := fs.s3().PutObjectRequest(&s3.PutObjectInput{
			Bucket: aws.String(fs.Bucket),
			Key:    aws.String(fmt.Sprintf("%s/foobar/hoge.txt", fs.Prefix)),
			Body:   strings.NewReader("abc123"),
		})
		if _, err := req.Send(ctx); err != nil {
			t.Error(err)
			return
		}

		if err := fs.Remove(ctx, "foobar"); err == nil {
			t.Errorf("want error, got %s", err)
		}
	})
}
