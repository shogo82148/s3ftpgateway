package ftp

import (
	"context"
	"crypto/tls"
	"net"
	"time"

	"github.com/sourcegraph/ctxvfs"
)

// A Server defines patameters for running a FTP server.
type Server struct {
	Addr string // TCP address to listen on, ":ftp" if empty

	Authorizer Authorizer // Authorize method

	FileSystem ctxvfs.FileSystem // Virtual File System

	TLSConfig *tls.Config

	listener net.Listener
}

// ListenAndServe listens on the TCP network address srv.Addr and then calls Serve to handle requests on incoming connections.
func (s *Server) ListenAndServe() error {
	addr := s.Addr
	if addr == "" {
		addr = ":ftp"
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	return s.Serve(tcpKeepAliveListener{ln.(*net.TCPListener)})
}

// ListenAndServeTLS listens on the TCP network address srv.Addr and
// then calls ServeTLS to handle requests on incoming TLS connections.
func (s *Server) ListenAndServeTLS(certFile, keyFile string) error {
	addr := s.Addr
	if addr == "" {
		addr = ":ftps"
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	defer ln.Close()
	return s.ServeTLS(tcpKeepAliveListener{ln.(*net.TCPListener)}, certFile, keyFile)
}

// Serve accepts incoming connections on the Listener l, creating a new service goroutine for each.
func (s *Server) Serve(l net.Listener) error {
	ctx := context.Background()
	var tempDelay time.Duration // how long to sleep on accept failure
	for {
		rw, err := l.Accept()
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Temporary() {
				if tempDelay == 0 {
					tempDelay = 5 * time.Millisecond
				} else {
					tempDelay *= 2
				}
				if max := 1 * time.Second; tempDelay > max {
					tempDelay = max
				}
				time.Sleep(tempDelay)
				continue
			}
			return err
		}
		tempDelay = 0
		c := s.newConn(rw)
		go c.serve(ctx)
	}
}

// ServeTLS accepts incoming connections on the Listener l, creating a
// new service goroutine for each.
func (s *Server) ServeTLS(l net.Listener, certFile, keyFile string) error {
	config := s.TLSConfig.Clone()
	if !strSliceContains(config.NextProtos, "ftp") {
		config.NextProtos = append(config.NextProtos, "ftp")
	}
	configHasCert := len(config.Certificates) > 0 || config.GetCertificate != nil
	if !configHasCert || certFile != "" || keyFile != "" {
		var err error
		config.Certificates = make([]tls.Certificate, 1)
		config.Certificates[0], err = tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			return err
		}
	}

	tlsListener := tls.NewListener(l, config)
	return s.Serve(tlsListener)
}

func (s *Server) newConn(rwc net.Conn) *ServerConn {
	c := &ServerConn{
		server: s,
		rwc:    rwc,
	}
	return c
}

func (s *Server) authorize(ctx context.Context, user, passord string) (*Authorization, error) {
	auth := s.Authorizer
	if auth == nil {
		auth = AnonymousAuthorizer
	}
	return auth.Authorize(ctx, user, passord)
}

// Close immediately closes all active net.Listeners
func (s *Server) Close() error {
	return s.listener.Close()
}

// tcpKeepAliveListener sets TCP keep-alive timeouts on accepted
// connections. It's used by ListenAndServe and ListenAndServeTLS so
// dead TCP connections (e.g. closing laptop mid-download) eventually
// go away.
type tcpKeepAliveListener struct {
	*net.TCPListener
}

func (ln tcpKeepAliveListener) Accept() (net.Conn, error) {
	tc, err := ln.AcceptTCP()
	if err != nil {
		return nil, err
	}
	tc.SetKeepAlive(true)
	tc.SetKeepAlivePeriod(3 * time.Minute)
	return tc, nil
}

func strSliceContains(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}
