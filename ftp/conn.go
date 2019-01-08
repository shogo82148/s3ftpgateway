package ftp

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"os"
	pkgpath "path"
	"sync"
	"time"

	"github.com/shogo82148/s3ftpgateway/vfs"
)

type protectionLevel byte

const (
	protectionLevelClear        protectionLevel = 'C'
	protectionLevelSafe                         = 'S'
	protectionLevelConfidential                 = 'E'
	protectionLevelPrivate                      = 'P'
)

// ServerConn is a connection of the ftp server.
type ServerConn struct {
	ctx       context.Context
	cancel    context.CancelFunc
	server    *Server
	tlsConfig *tls.Config
	sessionID string

	// connection for control
	mu      sync.Mutex
	rwc     net.Conn
	ctrl    *dumbTelnetConn
	scanner *bufio.Scanner

	executing    atomicBool
	shuttingDown atomicBool
	closeOnce    sync.Once
	closeErr     error

	// the authorization info
	user    string
	auth    *Authorization
	failCnt int // the count of failing authorization

	// pwd is current working directory.
	pwd string

	// TLS connection is enabled.
	tls bool

	// data channel protection level
	prot protectionLevel

	// for RNFR command.
	rmfr       string
	rmfrReader io.ReadCloser

	// a connector for data connection
	mudt sync.Mutex // guard dt
	dt   dataTransfer

	// use EPSV command for starting data connection.
	// if it is true, reject all data connection
	// setup commands other than EPSV (i.e., EPRT, PORT, PASV, et al.)
	epsvAll bool
}

// Server returns a ftp server of the connection.
func (c *ServerConn) Server() *Server {
	return c.server
}

func (c *ServerConn) tlsCfg() *tls.Config {
	if c.tlsConfig != nil {
		return c.tlsConfig
	}
	if c.server.TLSConfig != nil {
		return c.server.TLSConfig
	}
	return &tls.Config{}
}

func (c *ServerConn) serve() {
	c.server.logger().Printf(c.sessionID, "a new connection from %s", c.rwc.RemoteAddr().String())

	defer func() {
		if c.rmfrReader != nil {
			c.rmfrReader.Close()
		}
	}()

	c.WriteReply(StatusReady, "Service ready")

	for !c.shuttingDown.isSet() && c.scanner.Scan() {
		c.executing.setTrue()
		text := c.scanner.Text()
		if c.shuttingDown.isSet() {
			c.WriteReply(StatusNotAvailable, "Service not available, closing control connection.")
			break
		}
		cmd, err := ParseCommand(text)
		if err != nil {
			c.WriteReply(StatusBadCommand, "Syntax error.")
			continue
		}
		c.execCommand(cmd)
		c.executing.setFalse()
	}
	if err := c.scanner.Err(); err != nil {
		c.server.logger().Printf(c.sessionID, "error reading the control connection: %v", err)
	}
	c.server.logger().Print(c.sessionID, "closing the connection")
}

func (c *ServerConn) execCommand(cmd *Command) {
	ctx, cancel := context.WithTimeout(c.ctx, time.Minute)
	defer cancel()

	if cmd.Name != "PASS" {
		c.server.logger().PrintCommand(c.sessionID, cmd.Name, cmd.Arg)
	} else {
		c.server.logger().PrintCommand(c.sessionID, cmd.Name, "****")
	}

	command, ok := commands[cmd.Name]
	if !ok || command == nil {
		c.WriteReply(StatusBadCommand, "Command not found.")
		return
	}
	if command.RequireParam() && cmd.Arg == "" {
		c.WriteReply(StatusBadArguments, "Action aborted, required param missing.")
		return
	}
	if command.RequireAuth() && c.auth == nil {
		c.WriteReply(StatusNotLoggedIn, "Not logged in")
		return
	}
	command.Execute(ctx, c, cmd)
}

// WriteReply writes a ftp reply.
func (c *ServerConn) WriteReply(code int, messages ...string) {
	if len(messages) > 0 {
		c.server.logger().PrintResponse(c.sessionID, code, messages[0])
	} else {
		c.server.logger().PrintResponse(c.sessionID, code, "")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, err := c.writeReply(code, messages...); err != nil {
		c.server.logger().Printf(c.sessionID, "error: %v", err)
	}
}

func (c *ServerConn) writeReply(code int, messages ...string) (int, error) {
	if len(messages) == 0 {
		n, err := fmt.Fprintf(c.ctrl, "%03d \r\n", code)
		if err != nil {
			return n, err
		}
		if err := c.ctrl.Flush(); err != nil {
			return n, err
		}
		return n, nil
	} else if len(messages) == 1 {
		// single line Reply
		n, err := fmt.Fprintf(c.ctrl, "%03d %s\r\n", code, messages[0])
		if err != nil {
			return n, err
		}
		if err := c.ctrl.Flush(); err != nil {
			return n, err
		}
		return n, nil
	}

	// multiple lines Reply
	m := 0
	for _, msg := range messages[:len(messages)-1] {
		n, err := fmt.Fprintf(c.ctrl, "%03d-%s\r\n", code, msg)
		m += n
		if err != nil {
			return m, err
		}
	}

	n, err := fmt.Fprintf(c.ctrl, "%03d %s\r\n", code, messages[len(messages)-1])
	m += n
	if err != nil {
		return m, err
	}
	if err := c.ctrl.Flush(); err != nil {
		return m, err
	}
	return m, nil
}

func (c *ServerConn) upgradeToTLS() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	tlsConn := tls.Server(c.rwc, c.server.TLSConfig)
	if err := tlsConn.Handshake(); err != nil {
		return err
	}

	c.ctrl = newDumbTelnetConn(tlsConn, tlsConn)
	c.scanner = bufio.NewScanner(c.ctrl)
	c.tls = true

	return nil
}

func (c *ServerConn) fileSystem() vfs.FileSystem {
	fs := c.auth.FileSystem
	if fs == nil {
		fs = c.server.FileSystem
	}
	if fs == nil {
		fs = vfs.Null
	}
	return fs
}

// Close closes all connections inluding the data transfer connection.
func (c *ServerConn) Close() error {
	c.cancel()
	c.closeOnce.Do(c.close)
	return c.closeErr
}

func (c *ServerConn) close() {
	c.shuttingDown.setTrue()
	if err := c.closeDataTransfer(); err != nil && c.closeErr == nil {
		c.closeErr = err
	}
	if err := c.rwc.Close(); err != nil && c.closeErr == nil {
		c.closeErr = err
	}
}

// Shutdown wait for transfer and closes the connection.
func (c *ServerConn) Shutdown(ctx context.Context) error {
	if err := c.shutdown(); err != nil {
		return err
	}
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	for {
		if err := c.shutdown(); err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func (c *ServerConn) shutdown() error {
	c.shuttingDown.setTrue()
	c.mudt.Lock()
	defer c.mudt.Unlock()
	if _, ok := c.dt.(emptyDataTransfer); ok && !c.executing.isSet() {
		return c.rwc.Close()
	}
	return nil
}

func (c *ServerConn) publicIPv4() net.IP {
	for _, s := range c.server.PublicIPs {
		ip := net.ParseIP(s)
		if ip != nil {
			ip = ip.To4()
		}
		if ip != nil {
			return ip
		}
	}
	if addr, ok := c.rwc.LocalAddr().(*net.TCPAddr); ok {
		return addr.IP.To4()
	}
	return nil
}

func (c *ServerConn) remoteIP() net.IP {
	addr, ok := c.rwc.RemoteAddr().(*net.TCPAddr)
	if !ok {
		return nil
	}
	return addr.IP
}

func (c *ServerConn) buildPath(path string) string {
	if pkgpath.IsAbs(path) {
		return pkgpath.Clean(path)
	}
	return pkgpath.Clean(pkgpath.Join(c.pwd, path))
}

func (c *ServerConn) formatFileInfo(fi os.FileInfo) string {
	return fmt.Sprintf(
		"%s 1 %s %s %13d %s %s",
		fi.Mode(),
		c.auth.User, c.auth.User,
		fi.Size(),
		fi.ModTime().Format(" Jan _2 15:04"),
		fi.Name(),
	)
}
