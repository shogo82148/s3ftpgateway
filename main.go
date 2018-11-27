package main

import (
	"crypto/tls"
	"log"

	"github.com/shogo82148/s3ftpgateway/ftp"
	"github.com/sourcegraph/ctxvfs"
)

// LocalhostCert is a PEM-encoded TLS cert with SAN IPs
// "127.0.0.1" and "[::1]", expiring at Jan 29 16:00:00 2084 GMT.
// generated from src/crypto/tls:
// go run generate_cert.go  --rsa-bits 1024 --host 127.0.0.1,::1,example.com --ca --start-date "Jan 1 00:00:00 1970" --duration=1000000h
var LocalhostCert = []byte(`-----BEGIN CERTIFICATE-----
MIICFDCCAX2gAwIBAgIRAM5RafBpq4/SbyTSw9q35FswDQYJKoZIhvcNAQELBQAw
EjEQMA4GA1UEChMHQWNtZSBDbzAgFw03MDAxMDEwMDAwMDBaGA8yMDg0MDEyOTE2
MDAwMFowEjEQMA4GA1UEChMHQWNtZSBDbzCBnzANBgkqhkiG9w0BAQEFAAOBjQAw
gYkCgYEAq9rd/wk/xgYqXR/Uq89l1haEw8kysYDhShf1KSUslQpduQX0dxNuo4DW
QP1gPz7zv+EEWIY0VQfK3vaxE4vkoGysUUBKOAGm8sa+RO0DmwyAdKHvy9OnHaCw
5fcTqtd11+ixo1wvfbP9EFkAb5e5y3q0vWKVevl+g4CdsNIikGsCAwEAAaNoMGYw
DgYDVR0PAQH/BAQDAgKkMBMGA1UdJQQMMAoGCCsGAQUFBwMBMA8GA1UdEwEB/wQF
MAMBAf8wLgYDVR0RBCcwJYILZXhhbXBsZS5jb22HBH8AAAGHEAAAAAAAAAAAAAAA
AAAAAAEwDQYJKoZIhvcNAQELBQADgYEAeyTY6bOV/5CDdTAgHHaOLrrlq6G1Jkst
oRrZGP1qIYlKElBSCh4WPVANbbIjTaVJe4a4hZJQKq5XpbT/yiWnGpdUYlBKrLJ0
PCr0F+zdSYiI2ECfjBTI9790E0Vp9s583im1bz/G4PHvTXQ4RiYTRvSEVFJVgqCT
30Vnhxep3wo=
-----END CERTIFICATE-----`)

// LocalhostKey is the private key for localhostCert.
var LocalhostKey = []byte(`-----BEGIN RSA PRIVATE KEY-----
MIICXAIBAAKBgQCr2t3/CT/GBipdH9Srz2XWFoTDyTKxgOFKF/UpJSyVCl25BfR3
E26jgNZA/WA/PvO/4QRYhjRVB8re9rETi+SgbKxRQEo4Aabyxr5E7QObDIB0oe/L
06cdoLDl9xOq13XX6LGjXC99s/0QWQBvl7nLerS9YpV6+X6DgJ2w0iKQawIDAQAB
AoGAdUyCUd1CRitXJxDe3BZHcAUWwXvGuhk5rJUFpPvWnjPhHLTP06bT0Y3Sr7FB
zGlvffxcNwADIDadZeoDm0/Uz/w7k3XFKlsw1w182ScASmJ5uP8yUECa1Pwr7etl
KHoLnQx9ngi9E4AcK5IMFnS6kSbgKU/1KRqr+BTRq6kD6aECQQDbjRT03NpVE5Zw
XLf+GeLxDU2wkoBr+lninKt+sNS7yzMPVGDB9M1TzAGU5DLTL3br/OZPDuudNkpO
SRQigcofAkEAyGKzHmUxWneOnhoW/r7y/gKeXVOXb2UJBODes1aFcQ2E6g9j8UTx
c1qhlPpU5CAUCjZcz1zA2owyUxQrLDVINQJAJCIqCsq2XD4nCkMYPQfBo+6OlLrn
y92eIX+rceRkfqvIsYMvkXxatqnisMCF5N/w8JHkzaok+PDQdeXtHGjD/QJAdS3i
eL/MII8RgzrWf5nCFvAJE6IySB3ZLFUjZdQOrJGvTAA7/XbHiyFQpAHPaqenkGFB
3LDsxeB9/T8qD+wIkQJBAJGo1I8gu2kacq7FW+C6fQq/G7c5m8msks7ddZ87B8Qn
6NpYnBnfJPWMExNKe9aDWkY9Lrtwj+KsKu6EVDmrCrc=
-----END RSA PRIVATE KEY-----`)

func main() {
	cert, err := tls.X509KeyPair(LocalhostCert, LocalhostKey)
	if err != nil {
		log.Fatal(err)
	}
	s := &ftp.Server{
		Addr: ":8000",
		FileSystem: ctxvfs.Map(map[string][]byte{
			"hoge": []byte("Hello ftp!"),
		}),
		TLSConfig: &tls.Config{
			NextProtos:   []string{"ftp"},
			Certificates: []tls.Certificate{cert},
		},
	}
	if err := s.ListenAndServeTLS("", ""); err != nil {
		log.Fatal(err)
	}
}
