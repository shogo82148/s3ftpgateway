package ftp

import (
	"context"
	"io"
	"io/ioutil"
	"net"
	"strconv"
	"testing"
)

// choose two adjacent port.
func choosePort() (int, int, error) {
	ln1, err := net.Listen("tcp", ":0")
	if err != nil {
		return 0, 0, err
	}
	defer ln1.Close()
	_, port, _ := net.SplitHostPort(ln1.Addr().String())
	port1, _ := strconv.Atoi(port)

	port2 := port1 - 1
	ln2, err := net.Listen("tcp", ":"+strconv.Itoa(port2))
	if err != nil {
		return 0, 0, err
	}
	defer ln2.Close()

	return port2, port1, nil
}

func TestServerConn_newPassiveDataTransfer(t *testing.T) {
	var min, max int
	for i := 0; ; i++ {
		var err error
		min, max, err = choosePort()
		if err == nil {
			break
		}
		if i >= 5 {
			t.Errorf("cannot assign test port: %s", err)
			return
		}
	}

	s := &Server{
		MinPassivePort:      min,
		MaxPassivePort:      max,
		DisableAddressCheck: true,
	}

	t.Run("connect", func(t *testing.T) {
		conn := s.newConn(nil, nil)
		dt, err := conn.newPassiveDataTransfer()
		if err != nil {
			t.Error(err)
		}
		defer dt.Close()

		go func() {
			client, err := net.Dial("tcp", dt.l.Addr().String())
			if err != nil {
				t.Error(err)
			}
			defer client.Close()
			io.WriteString(client, "hello")
		}()

		c, err := dt.Conn(context.Background())
		if err != nil {
			t.Error(err)
		}
		data, err := ioutil.ReadAll(c)
		if err != nil {
			t.Error(err)
		}
		if string(data) != "hello" {
			t.Errorf("want hello, got %s", string(data))
		}
	})

	t.Run("already use", func(t *testing.T) {
		ln, err := net.Listen("tcp", ":"+strconv.Itoa(min))
		if err != nil {
			t.Fatal(err)
		}
		defer ln.Close()

		conn1 := s.newConn(nil, nil)
		dt1, err := conn1.newPassiveDataTransfer()
		if err != nil {
			t.Fatal(err)
		}
		defer dt1.Close()

		// all ports that the ftp server can use are all in used.
		// so newPassiveDataTransfer will return errEmptyPortNotFound.
		conn2 := s.newConn(nil, nil)
		_, err = conn2.newPassiveDataTransfer()
		if err != errEmptyPortNotFound {
			t.Errorf("want errEmptyPortNotFound, got %v", err)
		}
	})
}
