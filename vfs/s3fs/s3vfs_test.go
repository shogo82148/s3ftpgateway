package s3fs

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws/external"
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
}
