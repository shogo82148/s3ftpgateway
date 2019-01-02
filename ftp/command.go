package ftp

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	pkgpath "path"
	"sort"
	"strconv"
	"strings"
	"time"
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

var escapeQuote = strings.NewReplacer(
	`"`, `""`,
)

// helper function for handling file error (e.g. Open, Stat)
func handleFileError(c *ServerConn, err error) {
	if os.IsNotExist(err) {
		c.WriteReply(StatusFileUnavailable, "No such file.")
		return
	} else if os.IsPermission(err) {
		c.WriteReply(StatusFileUnavailable, "Permission is denied.")
		return
	}
	c.server.logger().Printf(c.sessionID, "fail to open file: %v", err)
	c.WriteReply(StatusBadCommand, "Internal error.")
}

type command interface {
	IsExtend() bool
	RequireParam() bool
	RequireAuth() bool
	Execute(ctx context.Context, c *ServerConn, cmd *Command)
}

type featureParam interface {
	FeatureParam() string
}

var commands = map[string]command{
	// FILE TRANSFER PROTOCOL (FTP)
	// https://tools.ietf.org/html/rfc959
	"ABOR": commandAbor{},
	"ACCT": commandAcct{},
	"ALLO": commandAllo{},
	"APPE": commandAppe{},
	"CDUP": commandCdup{},
	"CWD":  commandCwd{},
	"DELE": commandDele{},
	"HELP": commandHelp{},
	"LIST": commandList{},
	"MKD":  commandMkd{},
	"NLST": commandNlst{},
	"NOOP": commandNoop{},
	"MODE": commandMode{},
	"PASS": commandPass{},
	"PASV": commandPasv{},
	"PORT": commandPort{},
	"PWD":  commandPwd{},
	"QUIT": commandQuit{},
	// "REIN": nil, // a number of FTP servers do not implement it.
	"RETR": commandRetr{},
	"RMD":  commandRmd{},
	"RNFR": commandRnfr{},
	"RNTO": commandRnto{},
	"SITE": nil,
	// "SMNT": nil, // mount is not permitted.
	"STAT": commandStat{},
	"STOR": commandStor{},
	"STOU": commandStou{},
	"STRU": commandStru{},
	"SYST": commandSyst{},
	"TYPE": commandType{},
	"USER": commandUser{},

	// FTP Operation Over Big Address Records (FOOBAR)
	// https://tools.ietf.org/html/rfc1639
	// These commands are obsoleted by https://tools.ietf.org/html/rfc5797
	// "LPRT": commandLprt{},
	// "LPSV": commandLpsv{},

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
	"FEAT": commandFeat{},
	"OPTS": commandOpts{},

	// FTP Extensions for IPv6 and NATs
	// https://tools.ietf.org/html/rfc2428
	"EPRT": commandEprt{},
	"EPSV": commandEpsv{},

	// Internationalization of the File Transfer Protocol
	// https://tools.ietf.org/html/rfc2640
	"LANG": commandLang{},

	// Extensions to FTP
	// https://tools.ietf.org/html/rfc3659
	"MDTM": commandMdtm{},
	"MLSD": commandMlsd{},
	"MLST": commandMlst{},
	// "REST": nil, // I gave up, see https://github.com/shogo82148/s3ftpgateway/pull/34
	"SIZE": commandSize{},
}

type commandAbor struct{}

func (commandAbor) IsExtend() bool     { return false }
func (commandAbor) RequireParam() bool { return true }
func (commandAbor) RequireAuth() bool  { return true }

func (commandAbor) Execute(ctx context.Context, c *ServerConn, cmd *Command) {
	c.dt.Abort()
}

// ACCOUNT (ACCT)
// The argument field is a Telnet string identifying the user's account.
type commandAcct struct{}

func (commandAcct) IsExtend() bool     { return false }
func (commandAcct) RequireParam() bool { return true }
func (commandAcct) RequireAuth() bool  { return true }

func (commandAcct) Execute(ctx context.Context, c *ServerConn, cmd *Command) {
	// permission was already granted in response to USER or PASS.
	// no need to use ACCT.
	c.WriteReply(StatusCommandNotImplemented, "Permission was already granted.")
}

// ALLOCATE (ALLO)
// This command may be required by some servers to reserve
// sufficient storage to accommodate the new file to be
// transferred.
type commandAllo struct{}

func (commandAllo) IsExtend() bool     { return false }
func (commandAllo) RequireParam() bool { return true }
func (commandAllo) RequireAuth() bool  { return true }

func (commandAllo) Execute(ctx context.Context, c *ServerConn, cmd *Command) {
	// ALLO is obsolete.
	c.WriteReply(StatusCommandNotImplemented, "Obsolete.")
}

// APPEND (with create) (APPE)
// This command causes the server-DTP to accept the data
// transferred via the data connection and to store the data in
// a file at the server site.
type commandAppe struct{}

func (commandAppe) IsExtend() bool     { return false }
func (commandAppe) RequireParam() bool { return true }
func (commandAppe) RequireAuth() bool  { return true }

func (commandAppe) Execute(ctx context.Context, c *ServerConn, cmd *Command) {
	tctx, cancel := context.WithCancel(context.Background())

	name := c.buildPath(cmd.Arg)
	fs := c.fileSystem()
	chSuccess := make(chan bool, 1)
	go func() {
		defer cancel()
		r, err := fs.Open(tctx, name)
		if err != nil {
			if os.IsNotExist(err) {
				r = ioutil.NopCloser(strings.NewReader(""))
			} else if os.IsPermission(err) {
				chSuccess <- false
				c.WriteReply(StatusFileUnavailable, "Permission is denied.")
				return
			} else {
				chSuccess <- false
				c.server.logger().Printf(c.sessionID, "fail to open file: %v", err)
				c.WriteReply(StatusBadCommand, "Internal error.")
				return
			}
		}
		defer r.Close()

		c.WriteReply(StatusAboutToSend, "Data transfer starting")
		conn, err := c.dt.Conn(tctx)
		if err != nil {
			chSuccess <- false
			c.server.logger().Printf(c.sessionID, "fail to start data connection: %v", err)
			c.WriteReply(StatusTransfertAborted, "Requested file action aborted.")
			return
		}
		defer conn.Close()

		chSuccess <- true
		cr := &countReader{Reader: conn}
		reader := io.MultiReader(r, cr)
		err = c.fileSystem().Create(tctx, name, reader)
		if err != nil {
			c.server.logger().Printf(c.sessionID, "fail to store file: %v", err)
			c.WriteReply(StatusActionAborted, "Requested file action aborted.")
			return
		}
		c.WriteReply(StatusClosingDataConnection, fmt.Sprintf("OK, received %d bytes.", cr.count))
	}()
	select {
	case success := <-chSuccess:
		if !success {
			cancel()
		}
	case <-ctx.Done():
		cancel()
	}
}

// CHANGE TO PARENT DIRECTORY (CDUP)
// This command is a special case of CWD, and is included to
// simplify the implementation of programs for transferring
// directory trees between operating systems having different
// syntaxes for naming the parent directory.
type commandCdup struct{}

func (commandCdup) IsExtend() bool     { return false }
func (commandCdup) RequireParam() bool { return false }
func (commandCdup) RequireAuth() bool  { return true }

func (commandCdup) Execute(ctx context.Context, c *ServerConn, cmd *Command) {
	if c.pwd == "" || c.pwd == "/" {
		c.WriteReply(StatusNeedSomeUnavailableResource, "No such directory.")
		return
	}
	c.pwd = pkgpath.Dir(c.pwd)
	c.WriteReply(StatusCommandOK, fmt.Sprintf("Directory changed to %s.", c.pwd))
}

type commandCwd struct{}

func (commandCwd) IsExtend() bool     { return false }
func (commandCwd) RequireParam() bool { return true }
func (commandCwd) RequireAuth() bool  { return true }

func (commandCwd) Execute(ctx context.Context, c *ServerConn, cmd *Command) {
	path := c.buildPath(cmd.Arg)
	stat, err := c.fileSystem().Stat(ctx, path)
	if err != nil || !stat.IsDir() {
		c.WriteReply(StatusNeedSomeUnavailableResource, "No such directory.")
		return
	}
	c.pwd = path
	c.WriteReply(StatusCommandOK, fmt.Sprintf("Directory changed to %s.", path))
}

// DELETE (DELE)
// This command causes the file specified in the pathname to be
// deleted at the server site.
type commandDele struct{}

func (commandDele) IsExtend() bool     { return false }
func (commandDele) RequireParam() bool { return true }
func (commandDele) RequireAuth() bool  { return true }

func (commandDele) Execute(ctx context.Context, c *ServerConn, cmd *Command) {
	path := c.buildPath(cmd.Arg)
	if err := c.fileSystem().Remove(ctx, path); err != nil {
		if os.IsNotExist(err) {
			c.WriteReply(StatusNeedSomeUnavailableResource, "No such file.")
			return
		}
		c.WriteReply(StatusBadCommand, "Internal error.")
		return
	}
	c.WriteReply(StatusCommandOK, "Removed directory "+path)
}

// HELP (HELP)
// This command shall cause the server to send helpful
// information regarding its implementation status over the
// control connection to the user.
type commandHelp struct{}

func (commandHelp) IsExtend() bool     { return false }
func (commandHelp) RequireParam() bool { return false }
func (commandHelp) RequireAuth() bool  { return true }

func (commandHelp) Execute(ctx context.Context, c *ServerConn, cmd *Command) {
	if cmd.Arg == "" {
		c.WriteReply(StatusHelp, "s3ftpgateway - https://github.com/shogo82148/s3ftpgateway")
		return
	}
	name := strings.ToUpper(cmd.Arg)
	command, ok := commands[name]
	if !ok || command == nil {
		c.WriteReply(StatusNotImplemented, fmt.Sprintf("Unknown command %s.", name))
		return
	}
	c.WriteReply(StatusHelp, fmt.Sprintf("%s.", name))
}

// LIST (LIST)
type commandList struct{}

func (commandList) IsExtend() bool     { return false }
func (commandList) RequireParam() bool { return false }
func (commandList) RequireAuth() bool  { return true }

func (commandList) Execute(ctx context.Context, c *ServerConn, cmd *Command) {
	path := c.pwd

	// RFC959 says the argument is a path name, but some ftp clients send options(e.g. '-al').
	// it is hard to parse them, so we just ignore them.
	if cmd.Arg != "" && cmd.Arg[0] != '-' {
		path = c.buildPath(cmd.Arg)
	}

	info, err := c.fileSystem().ReadDir(ctx, path)
	if err != nil {
		if os.IsNotExist(err) {
			c.WriteReply(StatusNeedSomeUnavailableResource, "No such directory.")
			return
		}
		c.WriteReply(StatusBadCommand, "Internal error.")
		return
	}
	c.WriteReply(StatusAboutToSend, "File status okay; about to open data connection.")

	conn, err := c.dt.Conn(ctx)
	if err != nil {
		c.server.logger().Printf(c.sessionID, "fail to start data connection: %v", err)
		c.WriteReply(StatusTransfertAborted, "Requested file action aborted.")
		return
	}

	go func() {
		defer conn.Close()
		w := bufio.NewWriter(conn)
		bytes := int64(0)
		for _, fi := range info {
			n, _ := io.WriteString(w, c.formatFileInfo(fi))
			bytes += int64(n)
			n, _ = io.WriteString(w, "\r\n")
			bytes += int64(n)
		}
		if err := w.Flush(); err != nil {
			c.server.logger().Printf(c.sessionID, "fail to list directory: %v", err)
			c.WriteReply(StatusActionAborted, "Requested file action aborted.")
			return
		}
		c.WriteReply(StatusClosingDataConnection, fmt.Sprintf("Data transfer starting %d bytes", bytes))
	}()
}

type commandMkd struct{}

func (commandMkd) IsExtend() bool     { return false }
func (commandMkd) RequireParam() bool { return true }
func (commandMkd) RequireAuth() bool  { return true }

func (commandMkd) Execute(ctx context.Context, c *ServerConn, cmd *Command) {
	path := c.buildPath(cmd.Arg)
	if err := c.fileSystem().Mkdir(ctx, path); err != nil {
		if os.IsExist(err) {
			c.WriteReply(
				StatusDirectoryAlreadyExists,
				fmt.Sprintf(`"%s" directory already exists; taking no action.`, escapeQuote.Replace(path)),
			)
			return
		}
		c.WriteReply(StatusBadCommand, "Internal error.")
		return
	}
	c.WriteReply(StatusPathCreated, fmt.Sprintf(`"%s" directory created.`, escapeQuote.Replace(path)))
}

// NAME LIST (NLST)
// This command causes a directory listing to be sent from
// server to user site.
type commandNlst struct{}

func (commandNlst) IsExtend() bool     { return false }
func (commandNlst) RequireParam() bool { return false }
func (commandNlst) RequireAuth() bool  { return true }

func (commandNlst) Execute(ctx context.Context, c *ServerConn, cmd *Command) {
	path := c.pwd
	if cmd.Arg != "" {
		path = c.buildPath(cmd.Arg)
	}
	info, err := c.fileSystem().ReadDir(ctx, path)
	if err != nil {
		if os.IsNotExist(err) {
			c.WriteReply(StatusNeedSomeUnavailableResource, "No such directory.")
			return
		}
		c.WriteReply(StatusBadCommand, "Internal error.")
		return
	}
	c.WriteReply(StatusAboutToSend, "File status okay; about to open data connection.")

	conn, err := c.dt.Conn(ctx)
	if err != nil {
		c.server.logger().Printf(c.sessionID, "fail to start data connection: %v", err)
		c.WriteReply(StatusTransfertAborted, "Requested file action aborted.")
		return
	}

	go func() {
		defer conn.Close()
		w := bufio.NewWriter(conn)
		bytes := int64(0)
		for _, fi := range info {
			n, _ := fmt.Fprintf(w, "%s\r\n", fi.Name())
			bytes += int64(n)
		}
		if err := w.Flush(); err != nil {
			c.server.logger().Printf(c.sessionID, "fail to list directory: %v", err)
			c.WriteReply(StatusActionAborted, "Requested file action aborted.")
			return
		}
		c.WriteReply(StatusClosingDataConnection, fmt.Sprintf("Data transfer starting %d bytes", bytes))
	}()

}

// NOOP (NOOP)
// This command does not affect any parameters or previously
// entered commands.
type commandNoop struct{}

func (commandNoop) IsExtend() bool     { return false }
func (commandNoop) RequireParam() bool { return false }
func (commandNoop) RequireAuth() bool  { return false }

func (commandNoop) Execute(ctx context.Context, c *ServerConn, cmd *Command) {
	c.WriteReply(StatusCommandOK, "Okay.")
}

// TRANSFER MODE (MODE)
// The argument is a single Telnet character code specifying
// the data transfer modes described in the Section on
// Transmission Modes.
type commandMode struct{}

func (commandMode) IsExtend() bool     { return false }
func (commandMode) RequireParam() bool { return true }
func (commandMode) RequireAuth() bool  { return true }

func (commandMode) Execute(ctx context.Context, c *ServerConn, cmd *Command) {
	switch cmd.Arg {
	case "S", "s": // Stream Mode
		c.WriteReply(StatusCommandOK, "Change transfer mode to stream.")
		return
		// RFC 959 assigns the following modes, but they are obsolete.
		// case "B", "b": // Block Mode
		// case "C", "c": // Compressed Mode
	}
	c.WriteReply(StatusNotImplementedParameter, "Unknown transfer mode.")
}

type commandPass struct{}

func (commandPass) IsExtend() bool     { return false }
func (commandPass) RequireParam() bool { return true }
func (commandPass) RequireAuth() bool  { return false }

func (commandPass) Execute(ctx context.Context, c *ServerConn, cmd *Command) {
	auth, err := c.server.authorizer().Authorize(ctx, c, c.user, cmd.Arg)
	if err != nil {
		c.failCnt++
		if c.failCnt > 1 || !isAnonymous(c.user) {
			if err := sleepWithContext(ctx, 5*time.Second); err != nil {
				c.server.logger().Printf(c.sessionID, "fail to execute PASS command: %v", err)
				c.closing = true
				return
			}
		}
		if err == ErrAuthorizeFailed {
			c.WriteReply(StatusNotLoggedIn, "Not logged in.")
			c.closing = c.failCnt > 5
			return
		}
		c.WriteReply(StatusBadCommand, "Internal error.")
		c.closing = c.failCnt > 5
		return
	}
	c.failCnt = 0
	c.auth = auth
	c.pwd = "/"
	c.WriteReply(StatusLoggedIn, "User logged in, proceed.")
}

func isAnonymous(user string) bool {
	return user == "anonymous" || user == "ftp"
}

func sleepWithContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
	case <-ctx.Done():
		return ctx.Err()
	}
	return nil
}

type commandPasv struct{}

func (commandPasv) IsExtend() bool     { return false }
func (commandPasv) RequireParam() bool { return false }
func (commandPasv) RequireAuth() bool  { return true }

func (commandPasv) Execute(ctx context.Context, c *ServerConn, cmd *Command) {
	if c.epsvAll {
		c.WriteReply(StatusBadArguments, "PASV command is disabled.")
		return
	}
	ipv4 := c.publicIPv4()
	if ipv4 == nil {
		c.WriteReply(StatusNotImplemented, "PASV command is disabled.")
	}
	dt, err := c.newPassiveDataTransfer()
	if err != nil {
		if err == errPassiveModeIsDisabled {
			c.WriteReply(StatusNotImplemented, "Passive mode is disabled.")
			return
		}
		c.server.logger().Printf(c.sessionID, "fail to enter passive mode: %v", err)
		c.WriteReply(StatusCanNotOpenDataConnection, "Data connection failed.")
		return
	}
	addr := dt.l.Addr().(*net.TCPAddr)
	c.WriteReply(StatusPassiveMode, fmt.Sprintf("Entering Passive Mode (%d,%d,%d,%d,%d,%d)", ipv4[0], ipv4[1], ipv4[2], ipv4[3], addr.Port>>8, addr.Port&0xFF))
}

type commandPort struct{}

func (commandPort) IsExtend() bool     { return false }
func (commandPort) RequireParam() bool { return true }
func (commandPort) RequireAuth() bool  { return true }

func (commandPort) Execute(ctx context.Context, c *ServerConn, cmd *Command) {
	if c.epsvAll || !c.server.EnableActiveMode {
		c.WriteReply(StatusBadArguments, "PORT command is disabled.")
		return
	}

	args := strings.Split(cmd.Arg, ",")
	if len(args) != 6 {
		c.WriteReply(StatusBadArguments, "Syntax error.")
		return
	}
	nums := make([]int, 0, 6)
	for _, s := range args {
		n, err := strconv.Atoi(strings.TrimSpace(s))
		if err != nil {
			c.WriteReply(StatusBadArguments, "Syntax error.")
			return
		}
		nums = append(nums, n)
	}
	port := (nums[4] << 8) + nums[5]

	// https://tools.ietf.org/html/rfc2577
	// Protecting Against the Bounce Attack
	if port < 1024 || port > 65535 {
		c.WriteReply(StatusNotImplemented, "Command not implemented for that parameter.")
		return
	}

	_, err := c.newActiveDataTransfer(ctx, fmt.Sprintf("%d.%d.%d.%d:%d", nums[0], nums[1], nums[2], nums[3], port))
	if err != nil {
		c.server.logger().Printf(c.sessionID, "fail to enter active mode: %v", err)
		c.WriteReply(StatusCanNotOpenDataConnection, "Data connection failed.")
		return
	}
	c.WriteReply(StatusCommandOK, "Okay.")
}

type commandPwd struct{}

func (commandPwd) IsExtend() bool     { return false }
func (commandPwd) RequireParam() bool { return false }
func (commandPwd) RequireAuth() bool  { return true }

func (commandPwd) Execute(ctx context.Context, c *ServerConn, cmd *Command) {
	pwd := escapeQuote.Replace(c.pwd)
	c.WriteReply(StatusPathCreated, fmt.Sprintf(`"%s"`, pwd))
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
	// tctx is a context for transfering data
	tctx, cancel := context.WithCancel(context.Background())

	cherr := make(chan error, 1)
	go func() {
		defer cancel()

		f, err := c.fileSystem().Open(tctx, cmd.Arg)
		if err != nil {
			c.server.logger().Printf(c.sessionID, "fail to retrieve file: %v", err)
			cherr <- err
			return
		}
		defer f.Close()

		c.WriteReply(StatusAboutToSend, "File status okay; about to open data connection.")
		conn, err := c.dt.Conn(tctx)
		if err != nil {
			c.server.logger().Printf(c.sessionID, "fail to start data connection: %v", err)
			cherr <- err
			return
		}
		defer conn.Close()

		// starting to transfer succeed.
		cherr <- nil

		// transfering continues in the background.
		n, err := io.Copy(conn, f)
		if err != nil {
			c.server.logger().Printf(c.sessionID, "fail to retrieve file: %v", err)
			c.WriteReply(StatusActionAborted, "Requested file action aborted.")
			return
		}

		c.WriteReply(StatusClosingDataConnection, fmt.Sprintf("Data transfer starting %d bytes", n))
	}()

	// wait for starting to transfer.
	var err error
	select {
	case err = <-cherr:
	case <-ctx.Done():
		cancel()
		err = ctx.Err()
	}
	if err != nil {
		c.WriteReply(StatusBadFileName, "Requested action not taken.")
		return
	}
}

// RMD: Remove the directory with the name "pathname".
type commandRmd struct{}

func (commandRmd) IsExtend() bool     { return false }
func (commandRmd) RequireParam() bool { return true }
func (commandRmd) RequireAuth() bool  { return true }

func (commandRmd) Execute(ctx context.Context, c *ServerConn, cmd *Command) {
	path := c.buildPath(cmd.Arg)
	if err := c.fileSystem().Remove(ctx, path); err != nil {
		if os.IsNotExist(err) {
			c.WriteReply(StatusNeedSomeUnavailableResource, "No such directory.")
			return
		}
		c.WriteReply(StatusBadCommand, "Internal error.")
		return
	}
	c.WriteReply(StatusCommandOK, "Removed directory "+path)
}

// RRENAME FROM (RNFR)
// This command specifies the old pathname of the file which is
// to be renamed.
type commandRnfr struct{}

func (commandRnfr) IsExtend() bool     { return false }
func (commandRnfr) RequireParam() bool { return true }
func (commandRnfr) RequireAuth() bool  { return true }

func (commandRnfr) Execute(ctx context.Context, c *ServerConn, cmd *Command) {
	if c.rmfr != "" || c.rmfrReader != nil {
		c.WriteReply(StatusBadSequence, "RNTO must be call after RNFR.")
		return
	}

	path := c.buildPath(cmd.Arg)
	r, err := c.fileSystem().Open(ctx, path)
	if err != nil {
		if os.IsNotExist(err) {
			c.WriteReply(StatusNeedSomeUnavailableResource, "No such directory.")
			return
		}
		c.server.logger().Printf(c.sessionID, "fail to open file: %v", err)
		c.WriteReply(StatusBadCommand, "Internal error.")
		return
	}
	c.rmfr = path
	c.rmfrReader = r
	c.WriteReply(StatusRequestFilePending, "Requested file action pending further information.")
}

// RRENAME TO (RNTO)
// This command specifies the new pathname of the file
// specified in the immediately preceding "rename from"
// command.
type commandRnto struct{}

func (commandRnto) IsExtend() bool     { return false }
func (commandRnto) RequireParam() bool { return true }
func (commandRnto) RequireAuth() bool  { return true }

func (commandRnto) Execute(ctx context.Context, c *ServerConn, cmd *Command) {
	fs := c.fileSystem()
	if c.rmfr == "" || c.rmfrReader == nil {
		c.WriteReply(StatusBadSequence, "RNTO must be call after RNFR.")
		return
	}
	from := c.rmfr
	to := c.buildPath(cmd.Arg)
	r := c.rmfrReader
	c.rmfr = ""
	c.rmfrReader = nil

	go func() {
		defer r.Close()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		err := fs.Create(ctx, to, r)
		if err != nil {
			c.server.logger().Printf(c.sessionID, "fail to create file: %v", err)
			c.WriteReply(StatusBadCommand, "Internal error.")
			return
		}
		fs.Remove(ctx, from)
		c.WriteReply(StatusRequestedFileActionOK, "Requested file action okay, completed.")
	}()
}

// STATUS (STAT)
// This command shall cause a status response to be sent over
// the control connection in the form of a reply.
type commandStat struct{}

func (commandStat) IsExtend() bool     { return false }
func (commandStat) RequireParam() bool { return false }
func (commandStat) RequireAuth() bool  { return true }

func (commandStat) Execute(ctx context.Context, c *ServerConn, cmd *Command) {
	if cmd.Arg == "" {
		c.WriteReply(StatusSystem, "https://github.com/shogo82148/s3ftpgateway")
		return
	}

	fs := c.fileSystem()
	path := c.buildPath(cmd.Arg)
	stat, err := fs.Stat(ctx, path)
	if err != nil {
		handleFileError(c, err)
		return
	}
	status := StatusFile
	if stat.IsDir() {
		status = StatusDirectory
	}
	c.WriteReply(status, "STAT", c.formatFileInfo(stat), "End.")
}

// commandStor
type commandStor struct{}

func (commandStor) IsExtend() bool     { return false }
func (commandStor) RequireParam() bool { return true }
func (commandStor) RequireAuth() bool  { return true }

func (commandStor) Execute(ctx context.Context, c *ServerConn, cmd *Command) {
	c.WriteReply(StatusAboutToSend, "Data transfer starting")

	name := cmd.Arg
	conn, err := c.dt.Conn(ctx)
	if err != nil {
		c.server.logger().Printf(c.sessionID, "fail to start data connection: %v", err)
		c.WriteReply(StatusTransfertAborted, "Requested file action aborted.")
		return
	}

	go func() {
		defer conn.Close()
		r := &countReader{Reader: conn}
		err = c.fileSystem().Create(context.Background(), name, r)
		if err != nil {
			c.server.logger().Printf(c.sessionID, "fail to store file: %v", err)
			c.WriteReply(StatusActionAborted, "Requested file action aborted.")
			return
		}
		c.WriteReply(StatusClosingDataConnection, fmt.Sprintf("OK, received %d bytes.", r.count))
	}()
}

type countReader struct {
	io.Reader
	count int64
}

func (r *countReader) Read(b []byte) (int, error) {
	n, err := r.Reader.Read(b)
	r.count += int64(n)
	return n, err
}

// STORE UNIQUE (STOU)
// This command behaves like STOR except that the resultant
// file is to be created in the current directory under a name
// unique to that directory.
type commandStou struct{}

func (commandStou) IsExtend() bool     { return false }
func (commandStou) RequireParam() bool { return false }
func (commandStou) RequireAuth() bool  { return true }

func (commandStou) Execute(ctx context.Context, c *ServerConn, cmd *Command) {
	// generate unique file name.
	var name string
	var buf [16]byte
	if _, err := io.ReadFull(rand.Reader, buf[:]); err != nil {
		c.WriteReply(StatusBadCommand, "Internal error.")
		return
	}
	name = c.buildPath(hex.EncodeToString(buf[:]))

	c.WriteReply(StatusAboutToSend, fmt.Sprintf("Data transfer starting: %s", name))

	conn, err := c.dt.Conn(ctx)
	if err != nil {
		c.server.logger().Printf(c.sessionID, "fail to start data connection: %v", err)
		c.WriteReply(StatusTransfertAborted, "Requested file action aborted.")
		return
	}

	go func() {
		defer conn.Close()
		r := &countReader{Reader: conn}
		err = c.fileSystem().Create(context.Background(), name, r)
		if err != nil {
			c.server.logger().Printf(c.sessionID, "fail to store file: %v", err)
			c.WriteReply(StatusActionAborted, "Requested file action aborted.")
			return
		}
		c.WriteReply(StatusClosingDataConnection, fmt.Sprintf("OK, received %d bytes. unique file name: %s", r.count, name))
	}()
}

// FILE STRUCTURE (STRU)
// The argument is a single Telnet character code specifying
// file structure described in the Section on Data
// Representation and Storage.
type commandStru struct{}

func (commandStru) IsExtend() bool     { return false }
func (commandStru) RequireParam() bool { return true }
func (commandStru) RequireAuth() bool  { return true }

func (commandStru) Execute(ctx context.Context, c *ServerConn, cmd *Command) {
	switch cmd.Arg {
	case "F", "f": // File (no record structure)
		c.WriteReply(StatusCommandOK, "Set file structure to file.")
		return
		// RFC 959 assigns the following modes, but they are obsolete.
		// case "R", "r": // Record structure
		// case "P", "p": // Page structure
	}
	c.WriteReply(StatusNotImplementedParameter, "Unknown file structure.")
}

// SYSTEM (SYST)
// This command is used to find out the type of operating
// system at the server.
type commandSyst struct{}

func (commandSyst) IsExtend() bool     { return false }
func (commandSyst) RequireParam() bool { return false }
func (commandSyst) RequireAuth() bool  { return true }

func (commandSyst) Execute(ctx context.Context, c *ServerConn, cmd *Command) {
	c.WriteReply(StatusName, "UNIX Type: L8")
}

// commandType responds to the TYPE FTP command.
//
//  like the MODE and STRU commands, TYPE dates back to a time when the FTP
//  protocol was more aware of the content of the files it was transferring, and
//  would sometimes be expected to translate things like EOL markers on the fly.
//
//  Valid options were A(SCII), I(mage), E(BCDIC) or LN (for local type). Since
//  we plan to just accept bytes from the client unchanged, I think Image mode is
//  adequate. The RFC requires we accept ASCII mode however, so accept it, but
//  ignore it.
type commandType struct{}

func (commandType) IsExtend() bool     { return false }
func (commandType) RequireParam() bool { return false }
func (commandType) RequireAuth() bool  { return true }

func (commandType) Execute(ctx context.Context, c *ServerConn, cmd *Command) {
	switch cmd.Arg {
	case "A", "a":
		c.WriteReply(StatusCommandOK, "Type set to ASCII.")
	case "I", "i":
		c.WriteReply(StatusCommandOK, "Type set to binary.")
	default:
		c.WriteReply(StatusBadArguments, "Unknown type.")
	}
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

func (commandAuth) IsExtend() bool       { return true }
func (commandAuth) RequireParam() bool   { return true }
func (commandAuth) RequireAuth() bool    { return false }
func (commandAuth) FeatureParam() string { return "TLS" }

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

// Feature negotiation mechanism for the File Transfer Protocol
// https://tools.ietf.org/html/rfc2389

type commandFeat struct{}

// The FEAT command is extended command,
// while it is not included in the list of features supported.
func (commandFeat) IsExtend() bool { return false }

func (commandFeat) RequireParam() bool { return false }
func (commandFeat) RequireAuth() bool  { return false }

func (commandFeat) Execute(ctx context.Context, c *ServerConn, cmd *Command) {
	cmds := []string{}
	for k, v := range commands {
		if v == nil || !v.IsExtend() {
			continue
		}
		if f, ok := v.(featureParam); ok {
			k += " " + f.FeatureParam()
		}
		cmds = append(cmds, " "+k)
	}
	sort.Strings(cmds)
	cmds = append([]string{"Extensions supported:", " UTF8"}, cmds...)
	cmds = append(cmds, "End.")
	c.WriteReply(StatusSystem, cmds...)
}

type commandOpts struct{}

// The OPTS command is extended command,
// while it is not included in the list of features supported.
func (commandOpts) IsExtend() bool { return false }

func (commandOpts) RequireParam() bool { return false }
func (commandOpts) RequireAuth() bool  { return false }

func (commandOpts) Execute(ctx context.Context, c *ServerConn, cmd *Command) {
	parts := strings.Fields(cmd.Arg)
	if len(parts) != 2 {
		c.WriteReply(StatusBadArguments, "Invalid option.")
		return
	}
	if !strings.EqualFold(parts[0], "utf8") {
		c.WriteReply(StatusBadArguments, "Invalid option.")
		return
	}
	if strings.EqualFold(parts[1], "on") {
		c.WriteReply(StatusCommandOK, "UTF8 mode enabled.")
	} else {
		c.WriteReply(StatusBadArguments, "Unsupported non-utf8 mode.")
	}
}

// FTP Extensions for IPv6 and NATs
// https://tools.ietf.org/html/rfc2428

// commandEprt allows for the specification of an extended address for the data connection
type commandEprt struct{}

func (commandEprt) IsExtend() bool     { return true }
func (commandEprt) RequireParam() bool { return true }
func (commandEprt) RequireAuth() bool  { return true }

func (commandEprt) Execute(ctx context.Context, c *ServerConn, cmd *Command) {
	if c.epsvAll || !c.server.EnableActiveMode {
		c.WriteReply(StatusBadArguments, "EPRT command is disabled.")
		return
	}

	delem := cmd.Arg[:1]
	params := strings.Split(cmd.Arg, delem)
	if len(params) < 5 {
		c.WriteReply(StatusBadArguments, "Syntax error.")
		return
	}
	proto := params[1]
	addr := params[2]
	port := params[3]

	var ip net.IP
	switch proto {
	case "1": // IP v4
		ip = net.ParseIP(addr)
		if ip != nil {
			ip = ip.To4()
		}
	case "2": // IP v6
		ip = net.ParseIP(addr)
		if ip != nil {
			ip = ip.To16()
		}
	default:
		c.WriteReply(StatusNetworkProtoNotSupported, "Network protocol not supported, use (1,2)")
		return
	}
	if ip == nil {
		c.WriteReply(StatusBadArguments, "Invalid address.")
		return
	}
	portNum, err := strconv.Atoi(port)
	if err != nil {
		c.WriteReply(StatusBadArguments, "Invalid port number.")
		return
	}

	// https://tools.ietf.org/html/rfc2577
	// Protecting Against the Bounce Attack
	if portNum < 1024 || portNum > 65535 {
		c.WriteReply(StatusNotImplemented, "Command not implemented for that parameter.")
		return
	}

	_, err = c.newActiveDataTransfer(ctx, fmt.Sprintf("%s:%d", ip.String(), portNum))
	if err != nil {
		c.server.logger().Printf(c.sessionID, "fail to enter active mode: %v", err)
		c.WriteReply(StatusCanNotOpenDataConnection, "Data connection failed.")
		return
	}
	c.WriteReply(StatusCommandOK, "Okay.")
}

// commandEpsv requests that a server listen on a data port and wait for a connection
type commandEpsv struct{}

func (commandEpsv) IsExtend() bool     { return true }
func (commandEpsv) RequireParam() bool { return false }
func (commandEpsv) RequireAuth() bool  { return true }

func (commandEpsv) Execute(ctx context.Context, c *ServerConn, cmd *Command) {
	if strings.EqualFold(cmd.Arg, "all") {
		c.epsvAll = true
		c.WriteReply(StatusReady, "all data connection setup commands other than EPSV is disabled.")
		return
	}
	switch cmd.Arg {
	case "":
	case "1": // IPv4 Address
	case "2": // IPv6 Address
	default:
		c.WriteReply(StatusBadArguments, "Invalid arguments.")
		return
	}
	dt, err := c.newPassiveDataTransfer()
	if err != nil {
		if err == errPassiveModeIsDisabled {
			c.WriteReply(StatusNotImplemented, "Passive mode is disabled.")
			return
		}
		c.server.logger().Printf(c.sessionID, "fail to enter passive mode: %v", err)
		c.WriteReply(StatusCanNotOpenDataConnection, "Data connection failed.")
		return
	}
	addr := dt.l.Addr().(*net.TCPAddr)
	c.WriteReply(StatusExtendedPassiveMode, fmt.Sprintf("Entering extended passive mode (|||%d|)", addr.Port))
}

// Internationalization of the File Transfer Protocol
// https://tools.ietf.org/html/rfc2640
type commandLang struct{}

func (commandLang) IsExtend() bool       { return true }
func (commandLang) RequireParam() bool   { return true }
func (commandLang) RequireAuth() bool    { return false }
func (commandLang) FeatureParam() string { return "EN*" }

func (commandLang) Execute(ctx context.Context, c *ServerConn, cmd *Command) {
	args := strings.Split(cmd.Arg, "-")
	if len(args) < 1 {
		c.WriteReply(StatusBadArguments, "Syntax error in patameter.")
		return
	}
	if !strings.EqualFold(args[0], "en") {
		c.WriteReply(StatusNotImplemented, fmt.Sprintf("Language %s is not supported.", cmd.Arg))
		return
	}
	c.WriteReply(StatusCommandOK, fmt.Sprintf("Language change to %s", cmd.Arg))
}

// Extensions to FTP
// https://tools.ietf.org/html/rfc3659

// File Modification Time (MDTM)
type commandMdtm struct{}

func (commandMdtm) IsExtend() bool     { return true }
func (commandMdtm) RequireParam() bool { return true }
func (commandMdtm) RequireAuth() bool  { return true }

func (commandMdtm) Execute(ctx context.Context, c *ServerConn, cmd *Command) {
	fs := c.fileSystem()
	path := c.buildPath(cmd.Arg)
	stat, err := fs.Stat(ctx, path)
	if err != nil {
		handleFileError(c, err)
		return
	}
	c.WriteReply(StatusFile, stat.ModTime().UTC().Format("20060102150405.999"))
}

// Listings for Machine Processing (MLST and MLSD)
type commandMlst struct{}

func (commandMlst) IsExtend() bool       { return true }
func (commandMlst) RequireParam() bool   { return true }
func (commandMlst) RequireAuth() bool    { return true }
func (commandMlst) FeatureParam() string { return "Type*,Modify*,Size*,Perm*" }

func (commandMlst) Execute(ctx context.Context, c *ServerConn, cmd *Command) {
	path := c.pwd
	if cmd.Arg != "" {
		path = c.buildPath(cmd.Arg)
	}
	stat, err := c.fileSystem().Stat(ctx, path)
	if err != nil {
		handleFileError(c, err)
		return
	}
	c.WriteReply(
		StatusFile,
		"Listing "+path,
		formatMachineListings(stat),
		"End.",
	)
}

type commandMlsd struct{}

func (commandMlsd) IsExtend() bool     { return true }
func (commandMlsd) RequireParam() bool { return true }
func (commandMlsd) RequireAuth() bool  { return true }

func (commandMlsd) Execute(ctx context.Context, c *ServerConn, cmd *Command) {
	path := c.pwd
	if cmd.Arg != "" {
		path = c.buildPath(cmd.Arg)
	}
	info, err := c.fileSystem().ReadDir(ctx, path)
	if err != nil {
		handleFileError(c, err)
		return
	}
	c.WriteReply(StatusAboutToSend, "File status okay; about to open data connection.")

	conn, err := c.dt.Conn(ctx)
	if err != nil {
		c.server.logger().Printf(c.sessionID, "fail to start data connection: %v", err)
		c.WriteReply(StatusTransfertAborted, "Requested file action aborted.")
		return
	}

	go func() {
		defer conn.Close()
		w := bufio.NewWriter(conn)
		bytes := int64(0)
		for _, fi := range info {
			n, _ := fmt.Print(w, formatMachineListings(fi), "\r\n")
			bytes += int64(n)
		}
		if err := w.Flush(); err != nil {
			c.server.logger().Printf(c.sessionID, "fail to list directory: %v", err)
			c.WriteReply(StatusActionAborted, "Requested file action aborted.")
			return
		}
		c.WriteReply(StatusClosingDataConnection, fmt.Sprintf("Data transfer starting %d bytes", bytes))
	}()
}

func formatMachineListings(stat os.FileInfo) string {
	var builder strings.Builder
	if stat.IsDir() {
		builder.WriteString("Type=dir;")
	} else {
		builder.WriteString("Type=file;")
	}

	builder.WriteString("Modify=")
	builder.WriteString(stat.ModTime().UTC().Format("20060102150405.999"))
	builder.WriteString(";")

	fmt.Fprintf(&builder, "Size=%d;", stat.Size())

	builder.WriteString("Perm=")
	isdir := stat.IsDir()
	mode := stat.Mode()
	if !isdir && (mode&0600) == 0600 {
		builder.WriteRune('a') //  the APPE (append) command may be applied
	}
	if isdir && (mode&0200) == 0200 {
		// files may be created in the directory
		// the MKD command may be used to create a new directory
		builder.WriteString("cmp")
	}
	if (mode & 0200) == 0200 {
		// the object named may be deleted
		// the object named may be renamed
		builder.WriteString("df")
	}
	if isdir && (mode&0100) == 0100 {
		// that a CWD command naming the object should succeed
		builder.WriteString("e")
	}
	if isdir && (mode&0400) == 0400 {
		// he listing commands may be applied to the directory
		builder.WriteString("l")
	}
	if !isdir && (mode&0400) == 0400 {
		// the RETR command may be applied to that object
		builder.WriteString("r")
	}
	if !isdir && (mode&0200) == 0200 {
		// the STOR command may be applied to that object
		builder.WriteString("w")
	}
	builder.WriteString(";")

	builder.WriteString(" ")
	builder.WriteString(stat.Name())

	return builder.String()
}

// commandSize return the file size.
type commandSize struct{}

func (commandSize) IsExtend() bool     { return true }
func (commandSize) RequireParam() bool { return true }
func (commandSize) RequireAuth() bool  { return true }

func (commandSize) Execute(ctx context.Context, c *ServerConn, cmd *Command) {
	fs := c.fileSystem()
	path := c.buildPath(cmd.Arg)
	stat, err := fs.Stat(ctx, path)
	if err != nil {
		handleFileError(c, err)
		return
	}
	c.WriteReply(StatusFile, strconv.FormatInt(stat.Size(), 10))
}
