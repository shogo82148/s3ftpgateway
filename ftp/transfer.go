package ftp

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"strconv"
	"sync"
	"time"
)

type dataTransfer interface {
	// Conn returns the currect data connection.
	// If the connection is closed, Conn returns new connection next time.
	Conn(ctx context.Context) (net.Conn, error)

	// Abort force closes current data connection.
	Abort() error

	// Close closes the data transfer.
	Close() error
}

type defaultDataTransfer struct{}

func (defaultDataTransfer) Conn(ctx context.Context) (net.Conn, error) {
	// TODO:
	return nil, net.UnknownNetworkError("not implemented")
}

func (defaultDataTransfer) Abort() error {
	return nil
}

func (defaultDataTransfer) Close() error {
	return nil
}

type activeDataTransfer struct {
	conn net.Conn
}

func (c *ServerConn) newActiveDataTransfer(ctx context.Context, addr string) (*activeDataTransfer, error) {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return nil, errors.New("invalid address")
	}

	if !c.server.DisableAddressCheck {
		ctrl := c.remoteIP()
		if ctrl == nil || !ip.Equal(ctrl) {
			return nil, errors.New("invalid address")
		}
	}

	dialer := c.server.dialer()
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}
	if c.tls {
		conn = tls.Server(conn, c.tlsCfg())
	}
	t := &activeDataTransfer{
		conn: conn,
	}

	// replace old data transfer.
	c.dt.Close()
	c.dt = t
	return t, nil
}

func (t *activeDataTransfer) Conn(ctx context.Context) (net.Conn, error) {
	return t.conn, nil
}

func (t *activeDataTransfer) Abort() error {
	return t.conn.Close()
}

func (t *activeDataTransfer) Close() error {
	return t.conn.Close()
}

type chConn struct {
	conn net.Conn
	err  error
}

type passiveDataTransfer struct {
	port   int
	l      net.Listener
	ch     <-chan chConn
	closed chan struct{}
	s      *Server
	c      *ServerConn

	mu   sync.Mutex
	conn net.Conn
}

func (c *ServerConn) newPassiveDataTransfer() (*passiveDataTransfer, error) {
	var port int
	var l net.Listener
	for i := 0; ; i++ {
		var err error
		port, err = c.server.choosePassivePort()
		if err != nil {
			return nil, err
		}

		l, err = net.Listen("tcp", ":"+strconv.Itoa(port))
		if err == nil {
			break
		}
		if i >= 5 {
			return nil, err
		}
	}
	l = tcpKeepAliveListener{l.(*net.TCPListener)}
	if c.tls {
		l = tls.NewListener(l, c.tlsCfg())
	}

	ch := make(chan chConn)
	t := &passiveDataTransfer{
		port:   port,
		l:      &onceCloseListener{Listener: l},
		ch:     ch,
		closed: make(chan struct{}),
		s:      c.server,
		c:      c,
	}
	go t.listen(ch)

	// replace
	c.dt.Close()
	c.dt = t

	return t, nil
}

func (t *passiveDataTransfer) listen(ch chan<- chConn) {
	var tempDelay time.Duration // how long to sleep on accept failure
	defer t.l.Close()
	for {
		rw, err := t.l.Accept()
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
			select {
			case ch <- chConn{nil, err}:
				return
			case <-t.closed:
				return
			}
		}
		tempDelay = 0

		if !t.validRemote(rw) {
			continue
		}

		select {
		case ch <- chConn{rw, nil}:
			return
		case <-t.closed:
			rw.Close()
			return
		}
	}
}

func (t *passiveDataTransfer) validRemote(conn net.Conn) bool {
	if t.s.DisableAddressCheck {
		return true
	}
	ctrl := t.c.remoteIP()
	if ctrl == nil {
		return false
	}
	data, ok := conn.RemoteAddr().(*net.TCPAddr)
	if !ok {
		return false
	}
	return data.IP.Equal(ctrl)
}

type passiveDataTransferConn struct {
	net.Conn
	transfer *passiveDataTransfer
}

func (c passiveDataTransferConn) Close() error {
	c.transfer.mu.Lock()
	c.transfer.conn = nil
	c.transfer.mu.Unlock()
	return c.Conn.Close()
}

func (t *passiveDataTransfer) Conn(ctx context.Context) (net.Conn, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.conn != nil {
		return t.conn, nil
	}

	select {
	case c := <-t.ch:
		if c.err != nil {
			return nil, c.err
		}
		conn := passiveDataTransferConn{
			Conn:     c.conn,
			transfer: t,
		}
		t.conn = conn
		return conn, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (t *passiveDataTransfer) Abort() error {
	var conn net.Conn
	t.mu.Lock()
	conn = t.conn
	t.mu.Unlock()
	if conn != nil {
		return conn.Close()
	}
	return nil
}

func (t *passiveDataTransfer) Close() error {
	var conn net.Conn
	t.mu.Lock()
	conn = t.conn
	t.mu.Unlock()
	if conn != nil {
		conn.Close()
	}
	close(t.closed)
	t.s.releasePassivePort(t.port)
	return t.l.Close()
}

// onceCloseListener wraps a net.Listener, protecting it from
// multiple Close calls.
type onceCloseListener struct {
	net.Listener
	once     sync.Once
	closeErr error
}

func (oc *onceCloseListener) Close() error {
	oc.once.Do(oc.close)
	return oc.closeErr
}

func (oc *onceCloseListener) close() { oc.closeErr = oc.Listener.Close() }
