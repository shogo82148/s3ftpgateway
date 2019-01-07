package main

import (
	"flag"

	"github.com/sirupsen/logrus"
)

var config string

func init() {
	flag.StringVar(&config, "config", "", "the path to the configure file")
}

func main() {
	flag.Parse()
	if config == "" {
		logrus.Fatal("-config is missing.")
	}

	c, err := LoadConfig(config)
	if err != nil {
		logrus.WithError(err).Fatal("fail to load config")
	}
	Serve(c)
}
