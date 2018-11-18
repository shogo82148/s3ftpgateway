package ftp

import (
	"bufio"
	"context"
	"io"
	"log"
	"net"
	"strings"
)

type conn struct {
	server *Server
	rwc    net.Conn
}

func (c *conn) serve(ctx context.Context) {
	io.WriteString(c.rwc, "220 Service ready\r\n")
	s := bufio.NewScanner(c.rwc)
	for s.Scan() {
		text := s.Text()
		log.Println(text)
		var cmd, arg string
		if idx := strings.Index(text, " "); idx >= 0 {
			cmd = strings.ToUpper(text[:idx])
			arg = strings.TrimSpace(text[idx:])
		} else {
			cmd = strings.ToUpper(text)
		}
		_ = arg
		switch cmd {
		case "USER":
			io.WriteString(c.rwc, "331 User name ok, need password\r\n")
		case "PASS":
			io.WriteString(c.rwc, "230 User logged in\r\n")
		case "PWD":
			io.WriteString(c.rwc, "257 dummy response\r\n")
		default:
			io.WriteString(c.rwc, "500 Command not found\r\n")
		}
	}
	if err := s.Err(); err != nil {
		log.Println(err)
	}
}
