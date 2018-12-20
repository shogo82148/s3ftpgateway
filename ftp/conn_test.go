package ftp

import (
	"io/ioutil"
	"net"
	"testing"
)

func TestWriteReply(t *testing.T) {
	t.Run("no-message", func(t *testing.T) {
		client, server := net.Pipe()
		s := &Server{
			Logger: testLogger{t},
		}
		c := s.newConn(server)
		go func() {
			c.WriteReply(200)
			server.Close()
		}()
		ret, err := ioutil.ReadAll(client)
		if err != nil {
			t.Fatal(err)
		}
		if string(ret) != "200 \r\n" {
			t.Errorf(`want "200 \r\n", got %#v`, string(ret))
		}
		client.Close()
	})

	t.Run("one-line", func(t *testing.T) {
		client, server := net.Pipe()
		s := &Server{
			Logger: testLogger{t},
		}
		c := s.newConn(server)
		go func() {
			c.WriteReply(200, "Okay.")
			server.Close()
		}()
		ret, err := ioutil.ReadAll(client)
		if err != nil {
			t.Fatal(err)
		}
		if string(ret) != "200 Okay.\r\n" {
			t.Errorf(`want "200 Okay.\r\n", got %#v`, string(ret))
		}
		client.Close()
	})

	t.Run("multiple-lines", func(t *testing.T) {
		client, server := net.Pipe()
		s := &Server{
			Logger: testLogger{t},
		}
		c := s.newConn(server)
		go func() {
			c.WriteReply(200, "First line.", "Second line.", "Last line.")
			server.Close()
		}()
		ret, err := ioutil.ReadAll(client)
		if err != nil {
			t.Fatal(err)
		}
		if string(ret) != "200-First line.\r\n200-Second line.\r\n200 Last line.\r\n" {
			t.Errorf(`want "200-First line.\r\n200-Second line.\r\n200 Last line.\r\n", got %#v`, string(ret))
		}
		client.Close()
	})
}
