package ftp

import (
	"context"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
)

// Command is a ftp command.
type Command struct {
	Name string
	Arg  string
}

func (c Command) String() string {
	if len(c.Arg) == 0 {
		return c.Name
	}
	return c.Name + " " + c.Arg
}

// ParseCommand parses ftp commands.
func ParseCommand(s string) (*Command, error) {
	var c Command
	s = strings.TrimSpace(s)
	if idx := strings.Index(s, " "); idx >= 0 {
		c.Name = strings.ToUpper(s[:idx])
		c.Arg = strings.TrimSpace(s[idx:])
	} else {
		c.Name = strings.ToUpper(s)
	}
	return &c, nil
}

type command interface {
	IsExtend() bool
	RequireParam() bool
	RequireAuth() bool
	Execute(ctx context.Context, c *ServerConn, cmd *Command)
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
	"STOR": commandStor{},
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
	"AUTH": commandAuth{},
	"CCC":  nil,
	"CONF": nil,
	"ENC":  nil,
	"MIC":  nil,
	"PBSZ": commandPbsz{},
	"PROT": commandProt{},

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

func (commandPass) Execute(ctx context.Context, c *ServerConn, cmd *Command) {
	auth, err := c.server.authorize(ctx, c.user, cmd.Arg)
	if err != nil {
		if err == ErrAuthorizeFailed {
			c.WriteReply(StatusNotLoggedIn, "Not logged in.")
		}
		c.WriteReply(StatusBadCommand, "Internal error.")
	}
	c.auth = auth
	c.WriteReply(StatusLoggedIn, "User logged in, proceed.")
}

type commandPwd struct{}

func (commandPwd) IsExtend() bool     { return false }
func (commandPwd) RequireParam() bool { return false }
func (commandPwd) RequireAuth() bool  { return true }

func (commandPwd) Execute(ctx context.Context, c *ServerConn, cmd *Command) {
	// TODO: It's dummy response. fix me plz.
	c.WriteReply(StatusPathCreated, `"/"`)
}

// commandQuit closes the control connection
type commandQuit struct{}

func (commandQuit) IsExtend() bool     { return false }
func (commandQuit) RequireParam() bool { return false }
func (commandQuit) RequireAuth() bool  { return false }

func (commandQuit) Execute(ctx context.Context, c *ServerConn, cmd *Command) {
	c.WriteReply(StatusClosing, "Good bye.")
}

// commandRetr causes the server-DTP to transfer a copy of the
// file, specified in the pathname, to the server- or user-DTP
// at the other end of the data connection.  The status and
// contents of the file at the server site shall be unaffected.
type commandRetr struct{}

func (commandRetr) IsExtend() bool     { return false }
func (commandRetr) RequireParam() bool { return true }
func (commandRetr) RequireAuth() bool  { return true }

func (commandRetr) Execute(ctx context.Context, c *ServerConn, cmd *Command) {
	f, err := c.fileSystem().Open(ctx, cmd.Arg)
	if err != nil {
		c.WriteReply(StatusBadFileName, "Requested action not taken.")
		return
	}
	defer f.Close()

	c.WriteReply(StatusAboutToSend, "File status okay; about to open data connection.")
	conn, err := c.dt.Conn(ctx)
	if err != nil {
		c.WriteReply(StatusTransfertAborted, "Requested file action aborted.")
		return
	}
	n, err := io.Copy(conn, f)
	if err != nil {
		c.WriteReply(StatusActionAborted, "Requested file action aborted.")
		return
	}

	c.WriteReply(StatusClosingDataConnection, fmt.Sprintf("Data transfer starting %d bytes", n))
	if dt := c.dt; dt != nil {
		c.dt = nil
		dt.Close()
	}
}

// commandStor
type commandStor struct{}

func (commandStor) IsExtend() bool     { return false }
func (commandStor) RequireParam() bool { return false }
func (commandStor) RequireAuth() bool  { return true }

func (commandStor) Execute(ctx context.Context, c *ServerConn, cmd *Command) {
	f, err := c.fileSystem().Create(ctx, cmd.Arg)
	if err != nil {
		c.WriteReply(StatusBadFileName, "Requested action not taken.")
		return
	}
	c.WriteReply(StatusAboutToSend, "Data transfer starting")

	conn, err := c.dt.Conn(ctx)
	if err != nil {
		c.WriteReply(StatusTransfertAborted, "Requested file action aborted.")
		return
	}
	n, err := io.Copy(f, conn)
	if err != nil {
		c.WriteReply(StatusActionAborted, "Requested file action aborted.")
		return
	}
	if err := f.Close(); err != nil {
		c.WriteReply(StatusActionAborted, "Requested file action aborted.")
		return
	}
	c.WriteReply(StatusClosingDataConnection, fmt.Sprintf("OK, received %d bytes.", n))
	if dt := c.dt; dt != nil {
		c.dt = nil
		dt.Close()
	}
}

// commandType
type commandType struct{}

func (commandType) IsExtend() bool     { return false }
func (commandType) RequireParam() bool { return false }
func (commandType) RequireAuth() bool  { return true }

func (commandType) Execute(ctx context.Context, c *ServerConn, cmd *Command) {
	// TODO: Support other types
	c.WriteReply(StatusCommandOK, "Type set to ASCII")
}

// commandUser responds to the USER FTP command by asking for the password
type commandUser struct{}

func (commandUser) IsExtend() bool     { return false }
func (commandUser) RequireParam() bool { return true }
func (commandUser) RequireAuth() bool  { return false }

func (commandUser) Execute(ctx context.Context, c *ServerConn, cmd *Command) {
	c.user = cmd.Arg
	c.WriteReply(StatusUserOK, "User name ok, password required.")
}

// FTP Security Extensions
// https://tools.ietf.org/html/rfc2228
type commandAuth struct{}

func (commandAuth) IsExtend() bool     { return true }
func (commandAuth) RequireParam() bool { return true }
func (commandAuth) RequireAuth() bool  { return false }

func (commandAuth) Execute(ctx context.Context, c *ServerConn, cmd *Command) {
	if !strings.EqualFold(cmd.Arg, "TLS") {
		c.WriteReply(StatusNotImplementedParameter, "Action not taken.")
		return
	}
	c.WriteReply(StatusSecurityDataExchangeComplete, "AUTH command OK.")
	if err := c.upgradeToTLS(); err != nil {
		c.server.logger().Printf(c.sessionID, "fail to upgrade to tls: %v", err)
	}
}

type commandPbsz struct{}

func (commandPbsz) IsExtend() bool     { return true }
func (commandPbsz) RequireParam() bool { return true }
func (commandPbsz) RequireAuth() bool  { return false }

func (commandPbsz) Execute(ctx context.Context, c *ServerConn, cmd *Command) {
	if c.tls && cmd.Arg == "0" {
		c.WriteReply(StatusCommandOK, "OK.")
	} else {
		c.WriteReply(StatusFileUnavailable, "Action not taken.")
	}
}

// commandProt specify the data channel protection level.
type commandProt struct{}

func (commandProt) IsExtend() bool     { return true }
func (commandProt) RequireParam() bool { return true }
func (commandProt) RequireAuth() bool  { return false }

func (commandProt) Execute(ctx context.Context, c *ServerConn, cmd *Command) {
	switch cmd.Arg {
	case "C": // Clear
		c.prot = protectionLevelClear
		c.WriteReply(StatusCommandOK, "OK.")
	case "S": // Safe
		c.WriteReply(StatusProtLevelNotSupported, "Safe level is not supported.")
	case "E": // Confidential
		c.WriteReply(StatusProtLevelNotSupported, "Confidential level is not supported.")
	case "P": // Private
		if c.tls {
			c.WriteReply(StatusCommandOK, "OK.")
		} else {
			c.WriteReply(StatusProtLevelNotSupported, "Private level is only supported in TLS.")
		}
	}
}

// FTP Extensions for IPv6 and NATs
// https://tools.ietf.org/html/rfc2428

// commandEprt allows for the specification of an extended address for the data connection
type commandEprt struct{}

func (commandEprt) IsExtend() bool     { return true }
func (commandEprt) RequireParam() bool { return true }
func (commandEprt) RequireAuth() bool  { return false }

func (commandEprt) Execute(ctx context.Context, c *ServerConn, cmd *Command) {
	c.WriteReply(StatusNotImplemented, "Command not implemented.")
}

// commandEpsv requests that a server listen on a data port and wait for a connection
type commandEpsv struct{}

func (commandEpsv) IsExtend() bool     { return true }
func (commandEpsv) RequireParam() bool { return false }
func (commandEpsv) RequireAuth() bool  { return true }

func (commandEpsv) Execute(ctx context.Context, c *ServerConn, cmd *Command) {
	if dt := c.dt; dt != nil {
		c.dt = nil
		dt.Close()
	}
	dt, err := c.newPassiveDataTransfer()
	if err != nil {
		c.WriteReply(StatusCanNotOpenDataConnection, "Data connection failed.")
		return
	}
	_, port, err := net.SplitHostPort(dt.l.Addr().String())
	if err != nil {
		dt.Close()
		c.WriteReply(StatusCanNotOpenDataConnection, "Data connection failed.")
		return
	}
	c.dt = dt
	c.WriteReply(StatusExtendedPassiveMode, fmt.Sprintf("Entering extended passive mode (|||%s|)", port))
}

// Extensions to FTP
// https://tools.ietf.org/html/rfc3659

// commandSize return the file size.
type commandSize struct{}

func (commandSize) IsExtend() bool     { return true }
func (commandSize) RequireParam() bool { return true }
func (commandSize) RequireAuth() bool  { return true }

func (commandSize) Execute(ctx context.Context, c *ServerConn, cmd *Command) {
	fs := c.fileSystem()
	stat, err := fs.Stat(ctx, cmd.Arg)
	if err != nil {
		c.WriteReply(StatusFileUnavailable, "File system error.")
		return
	}
	c.WriteReply(StatusFile, strconv.FormatInt(stat.Size(), 10))
}
