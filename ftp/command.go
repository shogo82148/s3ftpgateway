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
}

// commandUser responds to the USER FTP command by asking for the password
type commandUser struct{}

func (commandUser) IsExtend() bool     { return false }
func (commandUser) RequireParam() bool { return true }
func (commandUser) RequireAuth() bool  { return false }

func (commandUser) Execute(ctx context.Context, c *conn, cmd, arg string) reply {
	c.User = arg
	return reply{Code: 331, Messages: []string{"User name ok, password required"}}
}
