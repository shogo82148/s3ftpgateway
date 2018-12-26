package main

import (
	"os"

	yaml "gopkg.in/yaml.v2"
)

// Config is a configure of s3ftpgateway.
type Config struct {
	Bucket string `yaml:"bucket"`
	Prefix string `yaml:"prefix"`
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
