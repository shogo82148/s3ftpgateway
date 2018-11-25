package ftp

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"log"
	"net"

	"github.com/sourcegraph/ctxvfs"
)

// ServerConn is a connection of the ftp server.
type ServerConn struct {
	server *Server

	// connection for control
	rwc  net.Conn
	ctrl *dumbTelnetConn

	chCmd   <-chan *Command
	chReply chan<- *Reply

	user string
	auth *Authorization

	dt dataTransfer
}

func (c *ServerConn) serve(ctx context.Context) {
	c.ctrl = newDumbTelnetConn(c.rwc, c.rwc)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	chCmd := make(chan *Command, 1)
	c.chCmd = chCmd
	chReply := make(chan *Reply, 1)
	c.chReply = chReply

	go c.handleCommand(chCmd)
	go c.handleReply(chReply)

	defer c.close()

	c.WriteReply(&Reply{Code: 220, Messages: []string{"Service ready"}})

	for cmd := range c.chCmd {
		if command, ok := commands[cmd.Name]; ok && command != nil {
			command.Execute(ctx, c, cmd)
		} else {
			c.WriteReply(&Reply{Code: 500, Messages: []string{"Command not found"}})
		}
	}
}

func (c *ServerConn) handleCommand(chCmd chan<- *Command) {
	s := bufio.NewScanner(c.ctrl)
	for s.Scan() {
		text := s.Text()
		log.Println(text)
		cmd, err := ParseCommand(text)
		if err != nil {
			log.Println(err)
			return
		}
		chCmd <- cmd
	}
	if err := s.Err(); err != nil {
		log.Println(err)
	}
}

func (c *ServerConn) handleReply(chReply <-chan *Reply) {
	for r := range chReply {
		c.writeReply(r)
	}
}

// Reply is a ftp reply.
type Reply struct {
	Code     int
	Messages []string
}

func (r Reply) String() string {
	var buf bytes.Buffer
	if len(r.Messages) == 0 {
		fmt.Fprintf(&buf, "%03d \n", r.Code)
	} else if len(r.Messages) == 1 {
		fmt.Fprintf(&buf, "%03d %s\n", r.Code, r.Messages[0])
	} else {
		fmt.Fprintf(&buf, "%03d-%s\n", r.Code, r.Messages[0])
		for _, msg := range r.Messages[1 : len(r.Messages)-1] {
			buf.WriteString(msg)
			buf.WriteByte('\n')
		}
		fmt.Fprintf(&buf, "%03d %s\n", r.Code, r.Messages[len(r.Messages)-1])
	}
	return buf.String()
}

// WriteReply writes a ftp reply.
func (c *ServerConn) WriteReply(r *Reply) {
	c.chReply <- r
}

func (c *ServerConn) writeReply(r *Reply) (int, error) {
	code := r.Code
	messages := r.Messages
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
	n, err := fmt.Fprintf(c.ctrl, "%03d-%s\r\n", code, messages[0])
	m += n
	if err != nil {
		return m, err
	}

	for _, msg := range messages[1 : len(messages)-1] {
		n, err := fmt.Fprintf(c.ctrl, "%s\r\n", msg)
		m += n
		if err != nil {
			return m, err
		}
	}

	n, err = fmt.Fprintf(c.ctrl, "%03d %s\r\n", code, messages[len(messages)-1])
	m += n
	if err != nil {
		return m, err
	}
	return m, nil
}

func (c *ServerConn) fileSystem() ctxvfs.FileSystem {
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
