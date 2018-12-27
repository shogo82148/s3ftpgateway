package main

import (
	"os"

	yaml "gopkg.in/yaml.v2"
)

// Config is a configure of s3ftpgateway.
type Config struct {
	Bucket   string          `yaml:"bucket"`
	Prefix   string          `yaml:"prefix"`
	Listenrs []ListenrConfig `yaml:"listeners"`
}

// ListenrConfig is a configure of listener.
type ListenrConfig struct {
	// Address is used for listening ftp control connections.
	Address string `yaml:"address"`

	// TLS enables implicit tls mode.
	TLS bool `yaml:"tls"`

	// Certificate is a file path for certificate public key.
	// The file must contain PEM encoded data.
	Certificate string `yaml:"certificate"`

	// CertificateKey is a file path for certificate private key.
	// The file must contain PEM encoded data.
	CertificateKey string `yaml:"certificate_key"`
}

// LoadConfig loads a configure file.
func LoadConfig(path string) (*Config, error) {
	// TODO: load confirue via http and s3

	var conf Config
	r, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	dec := yaml.NewDecoder(r)
	if err := dec.Decode(&conf); err != nil {
		return nil, err
	}
	return &conf, nil
}
