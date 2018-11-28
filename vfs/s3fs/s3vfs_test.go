package s3fs

import (
	"github.com/shogo82148/s3ftpgateway/vfs"
)

var _ vfs.FileSystem = &FileSystem{}
