package ftp

import "context"

type command interface {
	IsExtend() bool
	RequireParam() bool
	RequireAuth() bool
	Execute(ctx context.Context, c *conn, cmd, arg string) (int, error)
}

var commands = map[string]command{
	"USER": commandUser{},
}

// commandUser responds to the USER FTP command by asking for the password
type commandUser struct{}

func (commandUser) IsExtend() bool     { return false }
func (commandUser) RequireParam() bool { return true }
func (commandUser) RequireAuth() bool  { return false }

func (commandUser) Execute(ctx context.Context, c *conn, cmd, arg string) (int, error) {
	c.User = arg
	return c.writeReply(331, "User name ok, password required")
}
