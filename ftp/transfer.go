package ftp

import (
	"context"
	"net"
	"sync"
)

type dataTransfer interface {
	Conn(ctx context.Context) (net.Conn, error)
	Close() error
}

type passiveDataTransfer struct {
	l    net.Listener
	conn net.Conn
	ch   <-chan net.Conn
}

func newPassiveDataTransfer() (*passiveDataTransfer, error) {
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		return nil, err
	}
	l = &onceCloseListener{Listener: l}
	ch := make(chan net.Conn, 1)
	go func() {
		defer l.Close()
		conn, err := l.Accept()
		if err != nil {
			// TODO: error handling
			return
		}
		ch <- conn
		close(ch)
	}()

	return &passiveDataTransfer{
		l:  l,
		ch: ch,
	}, nil
}

func (t *passiveDataTransfer) Conn(ctx context.Context) (net.Conn, error) {
	if t.conn != nil {
		return t.conn, nil
	}
	select {
	case conn := <-t.ch:
		t.conn = conn
		return conn, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (t *passiveDataTransfer) Close() error {
	if conn := t.conn; conn != nil {
		t.conn = nil
		conn.Close()
	}
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
