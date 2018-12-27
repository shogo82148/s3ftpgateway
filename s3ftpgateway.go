package main

import (
	"log"
	"net"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws/external"
	"github.com/shogo82148/s3ftpgateway/ftp"
	"github.com/shogo82148/s3ftpgateway/vfs/s3fs"
)

// Serve serves s3ftpgateway service.
func Serve(config *Config) {
	ls, err := listeners(config)
	if err != nil {
		log.Fatal(err)
	}

	cfg, err := external.LoadDefaultAWSConfig()
	if err != nil {
		log.Fatal(err)
	}

	fs := &s3fs.FileSystem{
		Config: cfg,
		Bucket: config.Bucket,
		Prefix: config.Prefix,
	}

	s := &ftp.Server{
		FileSystem: fs,
		Authorizer: authorizer{},
	}

	var wg sync.WaitGroup
	for _, l := range ls {
		l := l
		wg.Add(1)
		go func() {
			defer wg.Done()
			if l.tls {
				if err := s.ServeTLS(l.listener, l.certFile, l.keyFile); err != nil {
					log.Fatal(err)
				}
			} else {
				if err := s.Serve(l.listener); err != nil {
					log.Fatal(err)
				}
			}
		}()
	}
	wg.Wait()
}

type listenerConfig struct {
	listener net.Listener
	tls      bool
	certFile string
	keyFile  string
}

func listeners(config *Config) (ls []listenerConfig, err error) {
	defer func() {
		if err != nil {
			for _, l := range ls {
				l.listener.Close()
			}
		}
	}()

	for _, listener := range config.Listenrs {
		var l net.Listener
		l, err = net.Listen("tcp", listener.Address)
		if err != nil {
			return
		}
		l = newTCPKeepAliveListener(l)

		ls = append(ls, listenerConfig{
			listener: l,
			tls:      listener.TLS,
			certFile: listener.Certificate,
			keyFile:  listener.CertificateKey,
		})
	}
	return
}

func newTCPKeepAliveListener(l net.Listener) net.Listener {
	if l, ok := l.(*net.TCPListener); ok {
		return tcpKeepAliveListener{l}
	}
	return l
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
