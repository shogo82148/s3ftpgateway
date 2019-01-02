#!/bin/sh

CURRENT=$(cd "$(dirname "$0")" && pwd)
docker run --rm -it \
    -e GO111MODULES=on \
    -v "$CURRENT":/go/src/github.com/shogo82148/s3ftpgateway \
    -w /go/src/github.com/shogo82148/s3ftpgateway golang:1.11.4 "$@"
