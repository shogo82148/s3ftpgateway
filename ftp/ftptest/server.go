package ftptest

import (
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"net"

	"github.com/shogo82148/s3ftpgateway/ftp"
	"github.com/shogo82148/s3ftpgateway/ftp/internal"
	"github.com/shogo82148/s3ftpgateway/vfs"
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

	certificate *x509.Certificate
}

// NewServer starts and returns a new Server.
// The caller should call Close when finished, to shut it down.
func NewServer(vfs vfs.FileSystem) *Server {
	ts := NewUnstartedServer(vfs)
	ts.Start()
	return ts
}

// NewTLSServer starts and returns a new Server using TLS.
// The caller should call Close when finished, to shut it down.
func NewTLSServer(vfs vfs.FileSystem) *Server {
	ts := NewUnstartedServer(vfs)
	ts.StartTLS()
	return ts
}

// NewUnstartedServer returns a new Server but doesn't start it.
func NewUnstartedServer(vfs vfs.FileSystem) *Server {
	return &Server{
		Listener: newLocalListener(),
		Config: &ftp.Server{
			FileSystem:       vfs,
			EnableActiveMode: true,
		},
	}
}

// Start starts a server from NewUnstartedServer.
func (s *Server) Start() {
	if s.URL != "" {
		panic("Server already started")
	}
	s.URL = "ftp://" + s.Listener.Addr().String()
	s.setupTLS()
	go s.Config.Serve(s.Listener)
}

// StartTLS starts TLS on a server from NewUnstartedServer.
func (s *Server) StartTLS() {
	if s.URL != "" {
		panic("Server already started")
	}
	s.setupTLS()
	s.Listener = tls.NewListener(s.Listener, s.Config.TLSConfig)
	s.URL = "ftps://" + s.Listener.Addr().String()
	go s.Config.Serve(s.Listener)
}

func (s *Server) setupTLS() {
	cert, err := tls.X509KeyPair(internal.LocalhostCert, internal.LocalhostKey)
	if err != nil {
		panic(fmt.Sprintf("ftptest: NewTLSServer: %v", err))
	}

	if existingConfig := s.Config.TLSConfig; existingConfig != nil {
		s.Config.TLSConfig = existingConfig.Clone()
	} else {
		s.Config.TLSConfig = new(tls.Config)
	}
	if s.Config.TLSConfig.NextProtos == nil {
		s.Config.TLSConfig.NextProtos = []string{"ftp"}
	}
	if len(s.Config.TLSConfig.Certificates) == 0 {
		s.Config.TLSConfig.Certificates = []tls.Certificate{cert}
	}
	s.certificate, err = x509.ParseCertificate(s.Config.TLSConfig.Certificates[0].Certificate[0])
	if err != nil {
		panic(fmt.Sprintf("ftptest: NewTLSServer: %v", err))
	}
}

// Certificate returns the certificate used by the server.
func (s *Server) Certificate() *x509.Certificate {
	return s.certificate
}

// Close shuts down the server.
func (s *Server) Close() {
	// TODO: shut down the server.
}
