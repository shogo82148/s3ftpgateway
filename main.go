package main

import (
	"flag"
	"fmt"
	"runtime"

	"github.com/sirupsen/logrus"
)

var config string
var showVersion bool

func init() {
	flag.StringVar(&config, "config", "", "the path to the configure file")
	flag.BoolVar(&showVersion, "version", false, "show the version")
}

func main() {
	flag.Parse()
	if showVersion {
		fmt.Printf(
			"s3ftpgateway version %s built with %s %s %s\n",
			getVersion(), runtime.Version(), runtime.GOOS, runtime.GOARCH,
		)
		return
	}
	if config == "" {
		logrus.Fatal("-config is missing.")
	}

	c, err := LoadConfig(config)
	if err != nil {
		logrus.WithError(err).Fatal("fail to load config")
	}
	Serve(c)
}
