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
	"sync/atomic"
	"time"

	"github.com/shogo82148/s3ftpgateway/vfs"
)

var defaultDialer net.Dialer

type atomicBool int32

func (b *atomicBool) isSet() bool { return atomic.LoadInt32((*int32)(b)) != 0 }
func (b *atomicBool) setTrue()    { atomic.StoreInt32((*int32)(b), 1) }

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

	shuttingDown atomicBool

	mu            sync.Mutex
	listeners     map[*net.Listener]struct{}
	ports         []int // ports are available port numbers.
	idxPorts      []int // inverted index for ports
	numEmptyPorts int
	doneChan      chan struct{}
}

func (s *Server) getDoneChan() <-chan struct{} {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.getDoneChanLocked()
}

func (s *Server) getDoneChanLocked() chan struct{} {
	if s.doneChan == nil {
		s.doneChan = make(chan struct{})
	}
	return s.doneChan
}

func (s *Server) closeDoneChanLocked() {
	ch := s.getDoneChanLocked()
	select {
	case <-ch:
		// Already closed. Don't close again.
	default:
		// Safe to close here. We're the only closer, guarded
		// by s.mu.
		close(ch)
	}
}

// ErrServerClosed is returned by the Server's Serve, ServeTLS, ListenAndServe,
// ListenAndServeTLS, ListenAndServeExplicitTLS, and ServeExplicitTLS methods after a call to Shutdown or Close.
var ErrServerClosed = errors.New("ftp: Server closed")

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

// ListenAndServeExplicitTLS listens on the TCP network address s.Addr and
// then calls ServeExplicitTLS to handle requests on incoming connections.
func (s *Server) ListenAndServeExplicitTLS(certFile, keyFile string) error {
	addr := s.Addr
	if addr == "" {
		addr = ":ftp"
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	return s.ServeExplicitTLS(tcpKeepAliveListener{ln.(*net.TCPListener)}, certFile, keyFile)
}

// ListenAndServeTLS listens on the TCP network address s.Addr and
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
	return s.ServeTLS(tcpKeepAliveListener{ln.(*net.TCPListener)}, certFile, keyFile)
}

// Serve accepts incoming connections on the Listener l, creating a new service goroutine for each.
func (s *Server) Serve(l net.Listener) error {
	return s.serve(l, nil)
}

func (s *Server) serve(l net.Listener, tlsConfig *tls.Config) error {
	l = &onceCloseListener{Listener: l}
	defer l.Close()

	if !s.trackListener(&l, true) {
		return ErrServerClosed
	}
	defer s.trackListener(&l, false)

	ctx := context.Background()
	var tempDelay time.Duration // how long to sleep on accept failure
	for {
		rw, err := l.Accept()
		if err != nil {
			select {
			case <-s.getDoneChan():
				return ErrServerClosed
			default:
			}
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
		c := s.newConn(rw, tlsConfig)
		go c.serve(ctx)
	}
}

// ServeExplicitTLS accepts incoming connections on the Listener l, creating a
// new service goroutine for each.
// The service goroutines handle FTP commands.
// An FTPS client must "explicitly request" security from the server
// and then step up to a mutually agreed encryption method.
func (s *Server) ServeExplicitTLS(l net.Listener, certFile, keyFile string) error {
	var config *tls.Config
	if s.TLSConfig != nil {
		config = s.TLSConfig.Clone()
	} else {
		config = &tls.Config{}
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

	return s.serve(l, config)
}

// ServeTLS accepts incoming connections on the Listener l, creating a
// new service goroutine for each.
// The service goroutines perform TLS setup and then handle FTP commands.
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
	return s.serve(tlsListener, config)
}

func (s *Server) newConn(rwc net.Conn, tlsConfig *tls.Config) *ServerConn {
	var sessionID string
	var buf [4]byte
	if _, err := io.ReadFull(rand.Reader, buf[:]); err != nil {
		sessionID = "????????"
	} else {
		sessionID = hex.EncodeToString(buf[:])
	}

	c := &ServerConn{
		server:    s,
		tlsConfig: tlsConfig,
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

// trackListener adds or removes a net.Listener to the set of tracked
// listeners.
func (s *Server) trackListener(ln *net.Listener, add bool) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.listeners == nil {
		s.listeners = make(map[*net.Listener]struct{})
	}
	if add {
		if s.shuttingDown.isSet() {
			return false
		}
		s.listeners[ln] = struct{}{}
	} else {
		delete(s.listeners, ln)
	}
	return true
}

func (s *Server) closeListenersLocked() error {
	var err error
	for ln := range s.listeners {
		if cerr := (*ln).Close(); cerr != nil && err == nil {
			err = cerr
		}
		delete(s.listeners, ln)
	}
	return err
}

// Close immediately closes all active net.Listeners and any connections.
func (s *Server) Close() error {
	s.shuttingDown.setTrue()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closeDoneChanLocked()
	s.closeListenersLocked()
	// TODO: close all connection.
	return nil
}

// Shutdown gracefully shuts down the server without interrupting any active data transfers.
// Shutdown works by first closing all open listeners, then waiting indefinitely for data transfers to complete,
// then send closing messages to clients, and then shut down.
// If the provided context expires before the shutdown is complete, Shutdown returns
// the context's error, otherwise it returns any error returned from closing the Server's underlying Listener(s).
//
// When Shutdown is called, Serve, ListenAndServe, and ListenAndServeTLS immediately return ErrServerClosed.
// Make sure the program doesn't exit and waits instead for Shutdown to return.
//
// Once Shutdown has been called on a server, it may not be reused;
// future calls to methods such as Serve will return ErrServerClosed.
func (s *Server) Shutdown(ctx context.Context) error {
	s.shuttingDown.setTrue()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closeDoneChanLocked()
	s.closeListenersLocked()
	// TODO: wait for all transfers are done.
	return nil
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
