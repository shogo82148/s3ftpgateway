package main

import (
	"flag"
	"log"
)

var config string

func init() {
	flag.StringVar(&config, "config", "", "the path to the configure file")
}

func main() {
	flag.Parse()
	if config == "" {
		log.Fatal("-config is missing.")
	}

	c, err := LoadConfig(config)
	if err != nil {
		log.Fatal("fail to load config: ", err)
	}
	Serve(c)
}
