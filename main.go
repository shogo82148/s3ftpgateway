package main

import (
	"log"

	"github.com/shogo82148/s3ftpgateway/ftp"
	"github.com/sourcegraph/ctxvfs"
)

func main() {
	s := &ftp.Server{
		Addr: ":8000",
		FileSystem: ctxvfs.Map(map[string][]byte{
			"hoge": []byte("Hello ftp!"),
		}),
	}
	if err := s.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
