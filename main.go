package main

import (
	"log"

	"github.com/shogo82148/s3ftpgateway/ftp"
)

func main() {
	s := &ftp.Server{
		Addr: ":8000",
	}
	if err := s.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
