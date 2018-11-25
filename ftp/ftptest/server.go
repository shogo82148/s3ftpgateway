package ftptest

import (
	"flag"
	"fmt"
	"net"

	"github.com/shogo82148/s3ftpgateway/ftp"
	"github.com/sourcegraph/ctxvfs"
)

func newLocalListener() net.Listener {
	if *serve != "" {
		l, err := net.Listen("tcp", *serve)
		if err != nil {
			panic(fmt.Sprintf("httptest: failed to listen on %v: %v", *serve, err))
		}
		return l
	}
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		if l, err = net.Listen("tcp6", "[::1]:0"); err != nil {
			panic(fmt.Sprintf("ftptest: failed to listen on a port: %v", err))
		}
	}
	return l
}

// When debugging a particular ftp server-based test,
// this flag lets you run
//	go test -run=BrokenTest -ftptest.serve=127.0.0.1:8000
// to start the broken server so you can interact with it manually.
var serve = flag.String("ftptest.serve", "", "if non-empty, ftptest.NewServer serves on this address and blocks")

// A Server is an FTP server listening on a system-chosen port on the local loopback interface,
// for use in end-to-end FTP tests.
type Server struct {
	URL      string // base URL of form ftp://ipaddr:port with no trailing slash
	Listener net.Listener

	// Config may be changed after calling NewUnstartedServer and
	// before Start or StartTLS.
	Config *ftp.Server
}

// NewServer starts and returns a new Server.
// The caller should call Close when finished, to shut it down.
func NewServer(vfs ctxvfs.FileSystem) *Server {
	ts := NewUnstartedServer(vfs)
	ts.Start()
	return ts
}

// NewUnstartedServer returns a new Server but doesn't start it.
func NewUnstartedServer(vfs ctxvfs.FileSystem) *Server {
	return &Server{
		Listener: newLocalListener(),
		Config: &ftp.Server{
			FileSystem: vfs,
		},
	}
}

// Start starts a server from NewUnstartedServer.
func (s *Server) Start() {
	if s.URL != "" {
		panic("Server already started")
	}
	s.URL = "ftp://" + s.Listener.Addr().String()
	go s.Config.Serve(s.Listener)
}

// Close shuts down the server.
func (s *Server) Close() {
	// TODO: shut down the server.
}
