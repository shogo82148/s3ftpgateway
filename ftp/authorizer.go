package ftp

import (
	"context"
	"errors"

	"github.com/shogo82148/s3ftpgateway/vfs"
)

// ErrAuthorizeFailed is an sentinel error for login failed.
var ErrAuthorizeFailed = errors.New("ftp: invalid user name or password")

// An Authorizer authorize ftp users.
type Authorizer interface {
	Authorize(ctx context.Context, conn *ServerConn, user, password string) (*Authorization, error)
}

// An Authorization is the result of Authorize.
type Authorization struct {
	User       string
	FileSystem vfs.FileSystem
}

// AnonymousAuthorizer is an Authorizer for anonymous users.
// It permits the anonymous users read only access to the virtual file system.
var AnonymousAuthorizer Authorizer = anonymousAuthorizer{}

type anonymousAuthorizer struct{}

func (anonymousAuthorizer) Authorize(ctx context.Context, conn *ServerConn, user, password string) (*Authorization, error) {
	if user != "anonymous" && user != "ftp" {
		return nil, ErrAuthorizeFailed
	}
	return &Authorization{
		User:       user,
		FileSystem: vfs.ReadOnly(conn.Server().FileSystem),
	}, nil
}

// NullAuthorizer is an Authorizer that accepts no one.
var NullAuthorizer Authorizer = nullAuthorizer{}

type nullAuthorizer struct{}

func (nullAuthorizer) Authorize(ctx context.Context, conn *ServerConn, user, password string) (*Authorization, error) {
	return nil, ErrAuthorizeFailed
}
