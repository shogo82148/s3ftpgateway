package main

import (
	"runtime/debug"
)

// the version of s3ftpgateway. it is is set by goreleaser.
var version string

func getVersion() string {
	if version != "" {
		return version
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown"
	}
	return info.Main.Version
}
