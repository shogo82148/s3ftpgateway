#!/bin/sh

CURRENT=$(cd "$(dirname "$0")" && pwd)
docker run --rm -it \
    -e GO111MODULE=on \
    -v "$CURRENT":/go/src/github.com/shogo82148/s3ftpgateway \
    -w /go/src/github.com/shogo82148/s3ftpgateway golang:1.15.6 "$@"
