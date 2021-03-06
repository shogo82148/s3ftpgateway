package main

import (
	"os"

	yaml "gopkg.in/yaml.v2"
)

// Config is a configure of s3ftpgateway.
type Config struct {
	Bucket    string           `yaml:"bucket"`
	Prefix    string           `yaml:"prefix"`
	Listeners []ListenerConfig `yaml:"listeners"`

	Log LogConfig `yaml:"log"`

	Authorizer AuthorizerConfig `yaml:"authorizer"`

	// MinPassivePort is minimum port number for passive data connections.
	// If MinPassivePort is more than MaxPassivePort, passive more is disabled.
	MinPassivePort int `yaml:"min_passive_port"`

	// MaxPassivePort is maximum port number for passive data connections.
	// If MaxPassivePort is zero, a port number is automatically chosen.
	MaxPassivePort int `yaml:"max_passive_port"`

	// PublicIPs are public IPs.
	PublicIPs []string `yaml:"public_ips"`

	// GuessPublicIP enables guessing public IP address.
	// It is for severs behind NAT, such as EC2.
	// s3ftpgateway uses http://169.254.169.254/latest/meta-data/public-ipv4 and,
	// https://checkip.amazonaws.com for guessing IP address.
	GuessPublicIP bool `yaml:"guess_public_ip"`

	// EnableActiveMode enables active transfer mode.
	// PORT and EPRT commands are disabled by default,
	// because it has some security risk, and most clients use passive mode.
	EnableActiveMode bool `yaml:"enable_active_mode"`

	// EnableAddressCheck enables checking address of data connection peer.
	// The checking is enabled by default to avoid the bounce attack.
	EnableAddressCheck bool `yaml:"enable_address_check"`

	// Certificate is a file path for certificate public key.
	// The file must contain PEM encoded data.
	Certificate string `yaml:"certificate"`

	// CertificateKey is a file path for certificate private key.
	// The file must contain PEM encoded data.
	CertificateKey string `yaml:"certificate_key"`
}

// ListenerConfig is a configure of listener.
type ListenerConfig struct {
	// Address is used for listening ftp control connections.
	Address string `yaml:"address"`

	// TLS enables implicit tls mode.
	TLS bool `yaml:"tls"`
}

// LogConfig is the config for log.
type LogConfig struct {
	// Format is the format of the log.
	// "json" or "text" are valid.
	Format string `yaml:"format"`
}

// AuthorizerConfig is config for authorize.
type AuthorizerConfig struct {
	Method string                 `yaml:"method"`
	Config map[string]interface{} `yaml:"config"`
}

// LoadConfig loads a configure file.
func LoadConfig(path string) (*Config, error) {
	// TODO: load configure via http and s3

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
