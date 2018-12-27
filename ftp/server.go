package ftp

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/shogo82148/s3ftpgateway/vfs"
)

var defaultDialer net.Dialer

// A Server defines patameters for running a FTP server.
type Server struct {
	// TCP address for the control connection to listen on, ":ftp" if empty.
	Addr string

	// Authorizer is an authorize method.
	// If it is nil, NullAuthorizer is used.
	Authorizer Authorizer

	// FileSystem is a virtual file system.
	// If it nil, vfs.Null is used.
	FileSystem vfs.FileSystem

	// TLSConfig optionally provides a TLS configuration for use
	// by ServeTLS and ListenAndServeTLS. Note that this value is
	// cloned by ServeTLS and ListenAndServeTLS, so it's not
	// possible to modify the configuration with methods like
	// tls.Config.SetSessionTicketKeys.
	TLSConfig *tls.Config

	// Logger specifies an optional logger.
	// If nil, logging is done via the log package's standard logger.
	Logger Logger

	// MinPassivePort is minimum port number for passive data connections.
	// If MinPassivePort is more than MaxPassivePort, passive more is disabled.
	MinPassivePort int

	// MaxPassivePort is maximum port number for passive data connections.
	// If MaxPassivePort is zero, a port number is automatically chosen.
	MaxPassivePort int

	// PublicIPs are public IPs.
	PublicIPs []string

	// Dialer is used for creating active data connections.
	// If it is nil, the zero value is used.
	Dialer *net.Dialer

	// EnableActiveMode enables active transfer mode.
	// PORT and EPRT commands are disabled by default,
	// because it has some security risk, and most clients use passive mode.
	EnableActiveMode bool

	// DisableAddressCheck disables checking address of data connection peer.
	// The checking is enabled by default to avoid the bounce attack.
	DisableAddressCheck bool

	listener net.Listener

	mu            sync.Mutex
	ports         []int // ports are available port numbers.
	idxPorts      []int // inverted index for ports
	numEmptyPorts int
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
	var config *tls.Config
	if s.TLSConfig != nil {
		config = s.TLSConfig.Clone()
	} else {
		config = &tls.Config{}
	}

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
	var sessionID string
	var buf [4]byte
	if _, err := io.ReadFull(rand.Reader, buf[:]); err != nil {
		sessionID = "????????"
	} else {
		sessionID = hex.EncodeToString(buf[:])
	}

	c := &ServerConn{
		server:    s,
		sessionID: sessionID,
		rwc:       rwc,
		dt:        defaultDataTransfer{},
	}

	// setup control channel
	c.ctrl = newDumbTelnetConn(c.rwc, c.rwc)
	c.scanner = bufio.NewScanner(c.ctrl)
	return c
}

func (s *Server) authorizer() Authorizer {
	auth := s.Authorizer
	if auth == nil {
		auth = NullAuthorizer
	}
	return auth
}

// Close immediately closes all active net.Listeners
func (s *Server) Close() error {
	return s.listener.Close()
}

func (s *Server) logger() Logger {
	if s.Logger == nil {
		return StdLogger
	}
	return s.Logger
}

func (s *Server) dialer() *net.Dialer {
	if s.Dialer == nil {
		return &defaultDialer
	}
	return s.Dialer
}

var errPassiveModeIsDisabled = errors.New("ftp: passive mode is disable")
var errEmptyPortNotFound = errors.New("ftp: empty port not found")

func (s *Server) choosePassivePort() (int, error) {
	min, max := s.MinPassivePort, s.MaxPassivePort
	if min > max {
		return 0, errPassiveModeIsDisabled
	}
	if max == 0 {
		return 0, nil
	}
	if min < 0 {
		min = 0
	}
	if max > 65535 {
		max = 65535
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ports == nil {
		s.ports = make([]int, max-min+1)
		s.idxPorts = make([]int, max-min+1)
		for i := range s.ports {
			s.ports[i] = i + min
			s.idxPorts[i] = i
		}
		s.numEmptyPorts = max - min + 1
	}
	if s.numEmptyPorts == 0 {
		return 0, errEmptyPortNotFound
	}

	idx := cryptorand.Intn(s.numEmptyPorts)
	s.numEmptyPorts--
	port1, port2 := s.ports[idx], s.ports[s.numEmptyPorts]
	s.ports[idx], s.ports[s.numEmptyPorts] = port2, port1
	s.idxPorts[port1-min], s.idxPorts[port2-min] = s.numEmptyPorts, idx
	return port1, nil
}

func (s *Server) releasePassivePort(port int) {
	if port == 0 {
		return
	}
	min, max := s.MinPassivePort, s.MaxPassivePort
	if port < min || port > max {
		panic(fmt.Sprintf("invalid port number %d", port))
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	idx1 := s.idxPorts[port-min]
	idx2 := s.numEmptyPorts
	port2 := s.ports[idx2]
	s.idxPorts[port-min], s.idxPorts[port2-min] = s.idxPorts[port2-min], s.idxPorts[port-min]
	s.ports[idx1], s.ports[idx2] = s.ports[idx2], s.ports[idx1]
	s.numEmptyPorts++
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
