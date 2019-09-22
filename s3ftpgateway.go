package main

import (
	"context"
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws/external"
	"github.com/shogo82148/s3ftpgateway/ftp"
	"github.com/shogo82148/s3ftpgateway/vfs/s3fs"
	"github.com/shogo82148/server-starter/listener"
	"github.com/sirupsen/logrus"
)

// Serve serves s3ftpgateway service.
func Serve(config *Config) {
	switch config.Log.Format {
	case "", "text":
		logrus.SetFormatter(&logrus.TextFormatter{})
	case "json":
		logrus.SetFormatter(&logrus.JSONFormatter{})
	default:
		logrus.Fatalf("unknown log format: %s", config.Log.Format)
	}

	if config.GuessPublicIP {
		ip1 := make(chan string, 1)
		ip2 := make(chan string, 1)
		go func() {
			ip1 <- guessPublicIPFromMetaData(context.Background())
		}()
		go func() {
			ip2 <- guessPublicIPFromCheckIP(context.Background())
		}()
		if ip := <-ip1; ip != "" {
			config.PublicIPs = []string{ip}
		} else if ip := <-ip2; ip != "" {
			config.PublicIPs = []string{ip}
		}
	}

	ls, err := listeners(config)
	if err != nil {
		logrus.WithError(err).Fatal("fail to listen")
	}

	cfg, err := external.LoadDefaultAWSConfig()
	if err != nil {
		logrus.WithError(err).Fatal("fail to get AWS config")
	}

	fs := &s3fs.FileSystem{
		Config: cfg,
		Bucket: config.Bucket,
		Prefix: config.Prefix,
	}

	auth, err := NewAuthorizer(config.Authorizer)
	if err != nil {
		logrus.WithError(err).Fatal("fail to parse s3ftpgateway config")
	}

	cert, err := loadCertificate(config)
	if err != nil {
		logrus.WithError(err).Fatal("fail to load certificate")
	}

	s := &ftp.Server{
		FileSystem: fs,
		Authorizer: auth,
		TLSConfig: &tls.Config{
			Certificates: []tls.Certificate{cert},
		},
		MinPassivePort:      config.MinPassivePort,
		MaxPassivePort:      config.MaxPassivePort,
		PublicIPs:           config.PublicIPs,
		EnableActiveMode:    config.EnableActiveMode,
		DisableAddressCheck: !config.EnableAddressCheck,
		Logger:              logger{},
	}

	// start to serve
	cherr := make(chan error, len(ls))
	for _, l := range ls {
		l := l
		go func() {
			cherr <- serve(s, l)
		}()
	}

	// handle signals
	chsig := make(chan os.Signal, 1)
	signal.Notify(
		chsig,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT,
	)

	var exitCode int
	var chshutdown chan struct{}
	for {
		select {
		case err := <-cherr:
			if err == ftp.ErrServerClosed {
				break
			}
			logrus.WithError(err).Error("fail to start the server")
			exitCode = 1
			if chshutdown != nil {
				break
			}
			chshutdown = make(chan struct{})
			go func() {
				defer close(chshutdown)
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				if err := s.Shutdown(ctx); err != nil {
					s.Close()
				}
			}()
		case sig := <-chsig:
			if chshutdown != nil {
				logrus.Infof("received %s, force close...", sig)
				s.Close()
				break
			}
			logrus.Infof("received %s, shutting down...", sig)
			chshutdown = make(chan struct{})
			go func() {
				defer close(chshutdown)
				s.Shutdown(context.Background())
			}()
		case <-chshutdown:
			logrus.Infof("exit with %d", exitCode)
			os.Exit(exitCode)
		}
	}
}

func serve(s *ftp.Server, l listenerConfig) error {
	if l.tls {
		logrus.WithFields(logrus.Fields{
			"address": l.listener.Addr().String(),
			"mode":    "implicit tls",
		}).Info("start to listen")
		return s.ServeTLS(l.listener, "", "")
	}
	logrus.WithFields(logrus.Fields{
		"address": l.listener.Addr().String(),
		"mode":    "plain",
	}).Info("start to listen")
	return s.Serve(l.listener)
}

type listenerConfig struct {
	listener net.Listener
	tls      bool
}

func listeners(config *Config) ([]listenerConfig, error) {
	lc, err := listener.PortsFallback()
	if err != nil {
		return nil, err
	}

	var lastErr error
	ls := make([]listenerConfig, 0, len(config.Listenrs))
	for _, listener := range config.Listenrs {
		addr := listener.Address
		if addr == "" {
			if listener.TLS {
				addr = ":ftps"
			} else {
				addr = ":ftp"
			}
		}
		l, err := lc.Listen(context.Background(), "tcp", addr)
		if err != nil {
			lastErr = err
			continue
		}
		l = newTCPKeepAliveListener(l)

		ls = append(ls, listenerConfig{
			listener: l,
			tls:      listener.TLS,
		})
	}

	if lastErr != nil {
		for _, l := range ls {
			l.listener.Close()
		}
		return nil, lastErr
	}
	return ls, nil
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

func loadCertificate(config *Config) (tls.Certificate, error) {
	return tls.LoadX509KeyPair(config.Certificate, config.CertificateKey)
}

// guessPublicIPFromMetaData guesses Public IP address from EC2 instance meta data.
func guessPublicIPFromMetaData(ctx context.Context) string {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	req, err := http.NewRequest(http.MethodGet, "http://169.254.169.254/latest/meta-data/public-ipv4", nil)
	if err != nil {
		return ""
	}
	req = req.WithContext(ctx)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return ""
	}
	var builer strings.Builder
	if _, err := io.Copy(&builer, resp.Body); err != nil {
		return ""
	}
	return strings.TrimSpace(builer.String())
}

// guessPublicIPFromCheckIP guesses Public IP address from checkip.amazonaws.com
func guessPublicIPFromCheckIP(ctx context.Context) string {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	req, err := http.NewRequest(http.MethodGet, "https://checkip.amazonaws.com", nil)
	if err != nil {
		return ""
	}
	req = req.WithContext(ctx)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return ""
	}
	var builer strings.Builder
	if _, err := io.Copy(&builer, resp.Body); err != nil {
		return ""
	}
	return strings.TrimSpace(builer.String())
}
