package ftp

import "context"

type command interface {
	IsExtend() bool
	RequireParam() bool
	RequireAuth() bool
	Execute(ctx context.Context, c *conn, cmd, arg string) reply
}

var commands = map[string]command{
	"USER": commandUser{},
	"PASS": commandPass{},
}

// commandUser responds to the USER FTP command by asking for the password
type commandUser struct{}

func (commandUser) IsExtend() bool     { return false }
func (commandUser) RequireParam() bool { return true }
func (commandUser) RequireAuth() bool  { return false }

func (commandUser) Execute(ctx context.Context, c *conn, cmd, arg string) reply {
	c.user = arg
	return reply{Code: 331, Messages: []string{"User name ok, password required"}}
}

type commandPass struct{}

func (commandPass) IsExtend() bool     { return false }
func (commandPass) RequireParam() bool { return true }
func (commandPass) RequireAuth() bool  { return false }

func (commandPass) Execute(ctx context.Context, c *conn, cmd, arg string) reply {
	auth, err := c.server.authorize(ctx, c.user, arg)
	if err != nil {
		if err == ErrAuthorizeFailed {
			return reply{Code: 530, Messages: []string{"Not logged in"}}
		}
		return reply{Code: 500, Messages: []string{"Internal error"}}
	}
	c.auth = auth
	return reply{Code: 230, Messages: []string{"User logged in, proceed"}}
}
