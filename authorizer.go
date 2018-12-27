package main

import (
	"context"

	"github.com/shogo82148/s3ftpgateway/ftp"
)

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
