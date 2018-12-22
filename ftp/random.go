package ftp

import (
	crand "crypto/rand"
	"encoding/binary"
	"math"
	"math/rand"
)

var cryptorand = rand.New(cryptosrc{})

type cryptosrc struct{}

func (cryptosrc) Int63() int64 {
	var buf [8]byte
	if _, err := crand.Read(buf[:]); err != nil {
		panic(err)
	}
	return int64(binary.BigEndian.Uint64(buf[:]) & math.MaxInt64)
}

func (cryptosrc) Seed(seed int64) {
}

func (cryptosrc) Uint64() uint64 {
	var buf [8]byte
	if _, err := crand.Read(buf[:]); err != nil {
		panic(err)
	}
	return binary.BigEndian.Uint64(buf[:])
}
