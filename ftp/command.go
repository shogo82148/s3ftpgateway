package ftp

import (
	"context"
	"fmt"
	"io"
	"net"
	"strconv"
)

type command interface {
	IsExtend() bool
	RequireParam() bool
	RequireAuth() bool
	Execute(ctx context.Context, c *conn, cmd, arg string) reply
}

var commands = map[string]command{
	// FILE TRANSFER PROTOCOL (FTP)
	// https://tools.ietf.org/html/rfc959
	"ABOR": nil,
	"ACCT": nil,
	"ALLO": nil,
	"APPE": nil,
	"CDUP": nil,
	"CWD":  nil,
	"DELE": nil,
	"HELP": nil,
	"LIST": nil,
	"MKD":  nil,
	"NLST": nil,
	"NOOP": nil,
	"MODE": nil,
	"PASS": commandPass{},
	"PASV": nil,
	"PORT": nil,
	"PWD":  commandPwd{},
	"QUIT": commandQuit{},
	"REIN": nil,
	"RETR": commandRetr{},
	"RMD":  nil,
	"RNFR": nil,
	"RNTO": nil,
	"SITE": nil,
	"SMNT": nil,
	"STAT": nil,
	"STOR": nil,
	"STOU": nil,
	"STRU": nil,
	"SYST": nil,
	"TYPE": commandType{},
	"USER": commandUser{},

	// FTP Operation Over Big Address Records (FOOBAR)
	// https://tools.ietf.org/html/rfc1639
	"LPRT": nil,
	"LPSV": nil,

	// FTP Security Extensions
	// https://tools.ietf.org/html/rfc2228
	"ADAT": nil,
	"AUTH": nil,
	"CCC":  nil,
	"CONF": nil,
	"ENC":  nil,
	"MIC":  nil,
	"PBSZ": nil,

	// Feature negotiation mechanism for the File Transfer Protocol
	// https://tools.ietf.org/html/rfc2389
	"FEAT": nil,
	"OPTS": nil,

	// FTP Extensions for IPv6 and NATs
	// https://tools.ietf.org/html/rfc2428
	"EPRT": commandEprt{},
	"EPSV": commandEpsv{},

	// Internationalization of the File Transfer Protocol
	// https://tools.ietf.org/html/rfc2640
	"LANG": nil,

	// Extensions to FTP
	// https://tools.ietf.org/html/rfc3659
	"MDTM": nil,
	"MLSD": nil,
	"MLST": nil,
	"REST": nil,
	"SIZE": commandSize{},
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

type commandPwd struct{}

func (commandPwd) IsExtend() bool     { return false }
func (commandPwd) RequireParam() bool { return false }
func (commandPwd) RequireAuth() bool  { return true }

func (commandPwd) Execute(ctx context.Context, c *conn, cmd, arg string) reply {
	// TODO: It's dummy response. fix me plz.
	return reply{Code: 257, Messages: []string{`"/"`}}
}

// commandQuit closes the control connection
type commandQuit struct{}

func (commandQuit) IsExtend() bool     { return false }
func (commandQuit) RequireParam() bool { return false }
func (commandQuit) RequireAuth() bool  { return false }

func (commandQuit) Execute(ctx context.Context, c *conn, cmd, arg string) reply {
	c.writeReply(reply{Code: 221, Messages: []string{"Good bye"}})
	c.rwc.Close()
	return reply{}
}

// commandRetr causes the server-DTP to transfer a copy of the
// file, specified in the pathname, to the server- or user-DTP
// at the other end of the data connection.  The status and
// contents of the file at the server site shall be unaffected.
type commandRetr struct{}

func (commandRetr) IsExtend() bool     { return false }
func (commandRetr) RequireParam() bool { return true }
func (commandRetr) RequireAuth() bool  { return true }

func (commandRetr) Execute(ctx context.Context, c *conn, cmd, arg string) reply {
	f, err := c.fileSystem().Open(ctx, arg)
	if err != nil {
		return reply{Code: 553, Messages: []string{"Requested action not taken"}}
	}
	defer f.Close()

	c.writeReply(reply{Code: 150, Messages: []string{"File status okay; about to open data connection."}})
	conn, err := c.dt.Conn(ctx)
	if err != nil {
		return reply{Code: 552, Messages: []string{"Requested file action aborted."}}
	}
	n, err := io.Copy(conn, f)
	if err != nil {
		return reply{Code: 552, Messages: []string{"Requested file action aborted."}}
	}
	if dt := c.dt; dt != nil {
		c.dt = nil
		dt.Close()
	}

	return reply{Code: 226, Messages: []string{fmt.Sprintf("Data transfer starting %d bytes", n)}}
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

// commandUser responds to the USER FTP command by asking for the password
type commandUser struct{}

func (commandUser) IsExtend() bool     { return false }
func (commandUser) RequireParam() bool { return true }
func (commandUser) RequireAuth() bool  { return false }

func (commandUser) Execute(ctx context.Context, c *conn, cmd, arg string) reply {
	c.user = arg
	return reply{Code: 331, Messages: []string{"User name ok, password required"}}
}

// FTP Extensions for IPv6 and NATs
// https://tools.ietf.org/html/rfc2428

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
	if dt := c.dt; dt != nil {
		c.dt = nil
		dt.Close()
	}
	dt, err := newPassiveDataTransfer()
	if err != nil {
		return reply{Code: 425, Messages: []string{"Data connection failed"}}
	}
	_, port, err := net.SplitHostPort(dt.l.Addr().String())
	if err != nil {
		dt.Close()
		return reply{Code: 425, Messages: []string{"Data connection failed"}}
	}
	c.dt = dt
	return reply{Code: 229, Messages: []string{fmt.Sprintf("Entering extended passive mode (|||%s|)", port)}}
}

// Extensions to FTP
// https://tools.ietf.org/html/rfc3659

// commandSize return the file size.
type commandSize struct{}

func (commandSize) IsExtend() bool     { return true }
func (commandSize) RequireParam() bool { return true }
func (commandSize) RequireAuth() bool  { return true }

func (commandSize) Execute(ctx context.Context, c *conn, cmd, arg string) reply {
	fs := c.fileSystem()
	stat, err := fs.Stat(ctx, arg)
	if err != nil {
		return reply{Code: 550, Messages: []string{"File system error"}}
	}
	return reply{Code: 229, Messages: []string{strconv.FormatInt(stat.Size(), 10)}}
}
