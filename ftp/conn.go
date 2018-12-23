package ftp

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"sync"

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
	server    *Server
	sessionID string

	// connection for control
	mu      sync.Mutex
	rwc     net.Conn
	ctrl    *dumbTelnetConn
	scanner *bufio.Scanner

	user string
	auth *Authorization

	// TLS connection is enabled.
	tls bool

	// data channel protection level
	prot protectionLevel

	// a connector for data connection
	dt dataTransfer
}

func (c *ServerConn) serve(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	defer c.close()

	c.WriteReply(StatusReady, "Service ready")

	for c.scanner.Scan() {
		text := c.scanner.Text()
		cmd, err := ParseCommand(text)
		if err != nil {
			c.WriteReply(StatusBadCommand, "Syntax error.")
			continue
		}

		if cmd.Name != "PASS" {
			c.server.logger().PrintCommand(c.sessionID, cmd.Name, cmd.Arg)
		} else {
			c.server.logger().PrintCommand(c.sessionID, cmd.Name, "****")
		}

		if command, ok := commands[cmd.Name]; ok && command != nil {
			command.Execute(ctx, c, cmd)
		} else {
			c.WriteReply(StatusBadCommand, "Command not found.")
		}
	}
	if err := c.scanner.Err(); err != nil {
		log.Println(err)
	}
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
	return c.server.FileSystem
}

func (c *ServerConn) close() error {
	if ServerConn := c.dt; ServerConn != nil {
		ServerConn.Close()
		c.dt = nil
	}
	if ServerConn := c.rwc; ServerConn != nil {
		ServerConn.Close()
		c.rwc = nil
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
