package ftp

import (
	"context"
	"fmt"
	"net"
)

type command interface {
	IsExtend() bool
	RequireParam() bool
	RequireAuth() bool
	Execute(ctx context.Context, c *conn, cmd, arg string) reply
}

var commands = map[string]command{
	"USER": commandUser{},
	"PASS": commandPass{},
	"TYPE": commandType{},

	// FTP Extensions for IPv6 and NATs https://tools.ietf.org/html/rfc2428
	"EPRT": commandEprt{},
	"EPSV": commandEpsv{},
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

// commandType
type commandType struct{}

func (commandType) IsExtend() bool     { return false }
func (commandType) RequireParam() bool { return false }
func (commandType) RequireAuth() bool  { return true }

func (commandType) Execute(ctx context.Context, c *conn, cmd, arg string) reply {
	// TODO: Support other types
	return reply{Code: 200, Messages: []string{"Type set to ASCII"}}
}

// commandEprt allows for the specification of an extended address for the data connection
type commandEprt struct{}

func (commandEprt) IsExtend() bool     { return true }
func (commandEprt) RequireParam() bool { return true }
func (commandEprt) RequireAuth() bool  { return false }

func (commandEprt) Execute(ctx context.Context, c *conn, cmd, arg string) reply {
	return reply{Code: 502, Messages: []string{"Command not implemented"}}
}

// commandEpsv requests that a server listen on a data port and wait for a connection
type commandEpsv struct{}

func (commandEpsv) IsExtend() bool     { return true }
func (commandEpsv) RequireParam() bool { return false }
func (commandEpsv) RequireAuth() bool  { return true }

func (commandEpsv) Execute(ctx context.Context, c *conn, cmd, arg string) reply {
	if ln := c.pasvListener; ln != nil {
		c.pasvListener = nil
		ln.Close()
	}
	if conn := c.dtp; conn != nil {
		conn.Close()
		c.dtp = nil
	}
	ln, err := net.Listen("tcp", ":0")
	if err != nil {
		return reply{Code: 425, Messages: []string{"Data connection failed"}}
	}
	_, port, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		return reply{Code: 425, Messages: []string{"Data connection failed"}}
	}
	c.pasvListener = ln
	return reply{Code: 229, Messages: []string{fmt.Sprintf("Entering extended passive mode (|||%s|)", port)}}
}
