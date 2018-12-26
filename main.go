package main

import (
	"context"
	"flag"
	"log"

	"github.com/aws/aws-sdk-go-v2/aws/external"
	"github.com/shogo82148/s3ftpgateway/ftp"
	"github.com/shogo82148/s3ftpgateway/vfs/s3fs"
)

var config string

func init() {
	flag.StringVar(&config, "config", "", "the path to the configure file")
}

func main() {
	flag.Parse()
	if config == "" {
		log.Fatal("-config is missing.")
	}

	c, err := LoadConfig(config)
	if err != nil {
		log.Fatal("fail to load config: ", err)
	}
	cfg, err := external.LoadDefaultAWSConfig()
	if err != nil {
		log.Fatal(err)
	}

	fs := &s3fs.FileSystem{
		Config: cfg,
		Bucket: c.Bucket,
		Prefix: c.Prefix,
	}

	s := &ftp.Server{
		Addr:       ":8000",
		FileSystem: fs,
		Authorizer: authorizer{},
	}
	if err := s.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}

type authorizer struct {
}

func (authorizer) Authorize(ctx context.Context, conn *ftp.ServerConn, user, password string) (*ftp.Authorization, error) {
	if user != "anonymous" && user != "ftp" {
		return nil, ftp.ErrAuthorizeFailed
	}
	return &ftp.Authorization{
		User:       user,
		FileSystem: conn.Server().FileSystem,
	}, nil
}
