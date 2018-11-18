package ftp

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"net"
	"strings"
)

type conn struct {
	server *Server
	rwc    net.Conn
	ctrlw  *bufio.Writer
}

func (c *conn) serve(ctx context.Context) {
	c.ctrlw = bufio.NewWriter(c.rwc)
	s := bufio.NewScanner(c.rwc)

	c.writeReply(220, "Service ready")
	for s.Scan() {
		text := s.Text()
		log.Println(text)
		cmd, _ := c.parseLine(text)
		switch cmd {
		case "USER":
			c.writeReply(331, "User name ok, need password")
		case "PASS":
			c.writeReply(230, "User logged in")
		case "PWD":
			c.writeReply(257, "dummy response")
		default:
			c.writeReply(500, "Command not found")
		}
	}
	if err := s.Err(); err != nil {
		log.Println(err)
	}
}

func (c *conn) parseLine(line string) (string, string) {
	var cmd, arg string
	if idx := strings.Index(line, " "); idx >= 0 {
		cmd = strings.ToUpper(line[:idx])
		arg = strings.TrimSpace(line[idx:])
	} else {
		cmd = strings.ToUpper(line)
	}
	return cmd, arg
}

func (c *conn) writeReply(code int, messages ...string) (int, error) {
	if len(messages) == 0 {
		n, err := fmt.Fprintf(c.ctrlw, "%03d \r\n", code)
		if err != nil {
			return n, err
		}
		if err := c.ctrlw.Flush(); err != nil {
			return n, err
		}
		return n, nil
	} else if len(messages) == 1 {
		// single line reply
		n, err := fmt.Fprintf(c.ctrlw, "%03d %s\r\n", code, messages[0])
		if err != nil {
			return n, err
		}
		if err := c.ctrlw.Flush(); err != nil {
			return n, err
		}
		return n, nil
	}

	// multiple lines reply
	m := 0
	n, err := fmt.Fprintf(c.ctrlw, "%03d-%s\r\n", code, messages[0])
	m += n
	if err != nil {
		return m, err
	}

	for _, msg := range messages[1 : len(messages)-1] {
		n, err := fmt.Fprintf(c.ctrlw, " %s\r\n", msg)
		m += n
		if err != nil {
			return m, err
		}
	}

	n, err = fmt.Fprintf(c.ctrlw, "%03d %s\r\n", code, messages[len(messages)-1])
	m += n
	if err != nil {
		return m, err
	}
	return m, nil
}
