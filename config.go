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

	// MinPassivePort is minimum port number for passive data connections.
	// If MinPassivePort is more than MaxPassivePort, passive more is disabled.
	MinPassivePort int `yaml:"min_passive_port"`

	// MaxPassivePort is maximum port number for passive data connections.
	// If MaxPassivePort is zero, a port number is automatically chosen.
	MaxPassivePort int `yaml:"max_passive_port"`

	// PublicIPs are public IPs.
	PublicIPs []string `yaml:"public_ips"`

	// EnableActiveMode enables active transfer mode.
	// PORT and EPRT commands are disabled by default,
	// because it has some security risk, and most clients use passive mode.
	EnableActiveMode bool `yaml:"enable_active_mode"`

	// EnableAddressCheck enables checking address of data connection peer.
	// The checking is enabled by default to avoid the bounce attack.
	EnableAddressCheck bool `yaml:"enable_address_check"`
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

	conf := Config{
		EnableAddressCheck: true,
	}
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
