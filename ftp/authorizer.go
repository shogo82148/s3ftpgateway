package ftp

import (
	"context"
	"errors"
)

// ErrAuthorizeFailed is an sentinel error for login failed.
var ErrAuthorizeFailed = errors.New("ftp: invalid user name or password")

// An Authorizer authorize ftp users.
type Authorizer interface {
	Authorize(ctx context.Context, user, passord string) (*Authorization, error)
}

// An Authorization is the result of Authorize
type Authorization struct {
	User string
}

// AnonymousAuthorizer is an Authorizer for anonymous users.
var AnonymousAuthorizer Authorizer = anonymousAuthorizer{}

type anonymousAuthorizer struct{}

func (anonymousAuthorizer) Authorize(ctx context.Context, user, passord string) (*Authorization, error) {
	if user != "anonymous" && user != "ftp" {
		return nil, ErrAuthorizeFailed
	}
	return &Authorization{
		User: user,
	}, nil
}
