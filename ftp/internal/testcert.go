package internal

// LocalhostCert is a PEM-encoded TLS cert with "*.loopback.shogo82148.com"
// (its IPs are "127.0.0.1" and "[::1]"), expiring at Jan 29 16:00:00 2084 GMT.
// generated from src/crypto/tls:
// go run generate_cert.go  --rsa-bits 1024 --host "*.loopback.shogo82148.com" --ca --start-date "Jan 1 00:00:00 1970" --duration=1000000h
var LocalhostCert = []byte(`-----BEGIN CERTIFICATE-----
MIICCjCCAXOgAwIBAgIRAMwQp+dGO4nNqNbf/k5sf5swDQYJKoZIhvcNAQELBQAw
EjEQMA4GA1UEChMHQWNtZSBDbzAgFw03MDAxMDEwMDAwMDBaGA8yMDg0MDEyOTE2
MDAwMFowEjEQMA4GA1UEChMHQWNtZSBDbzCBnzANBgkqhkiG9w0BAQEFAAOBjQAw
gYkCgYEA5aaXdzEsNEa8Zzsl2UPqI+gSb22qWAfkpOolThvZjbtJ5y6zuppfISGA
0IytnAAg+0La5tLYl4hvjPvIoA1DryvhVDTlKyS8X/PFWDskrEJm1RefOjypcnRC
Pre8Yc9toMn52svoHcxMXqkzSolORbx3B6JLYIT39APpj26GYaUCAwEAAaNeMFww
DgYDVR0PAQH/BAQDAgKkMBMGA1UdJQQMMAoGCCsGAQUFBwMBMA8GA1UdEwEB/wQF
MAMBAf8wJAYDVR0RBB0wG4IZKi5sb29wYmFjay5zaG9nbzgyMTQ4LmNvbTANBgkq
hkiG9w0BAQsFAAOBgQCEgPTag9+hahm5g+1+KSdQAIB4QsxH8mB7by4jLOiCh2np
vN+7SC3D131YFJlpJR2D4s0KxjbcCfKyMXEyGAR5v4MxXr3YYhbDwSHRvYK7Qn7p
D9Gn2dAbmmy+0HlpaY3zap0yvUu4fbVpr5zwwf2QDtx0PGkzqz2modOULeXt9Q==
-----END CERTIFICATE-----
`)

// LocalhostKey is the private key for localhostCert.
var LocalhostKey = []byte(`-----BEGIN PRIVATE KEY-----
MIICdQIBADANBgkqhkiG9w0BAQEFAASCAl8wggJbAgEAAoGBAOWml3cxLDRGvGc7
JdlD6iPoEm9tqlgH5KTqJU4b2Y27Secus7qaXyEhgNCMrZwAIPtC2ubS2JeIb4z7
yKANQ68r4VQ05SskvF/zxVg7JKxCZtUXnzo8qXJ0Qj63vGHPbaDJ+drL6B3MTF6p
M0qJTkW8dweiS2CE9/QD6Y9uhmGlAgMBAAECgYAwuZzvdCZt3QhCWuFX7Lnz7lxi
+gCndt1DRE6v+Oa61J8EhvspP3GppOMg3IhFTh2xUekCCoBb/l20qwNROh8+14JG
zdzC0anzJt+5tdwESmc9pXp0kIz8HrQPsv+WuAKxvnGVV416kpp47Xi8pqDju0Bp
Ajcdsu436wClMoL0DQJBAOwYC+olzEN5q92c/jKDFKe01lRGCw4R7auMXDtrEYMf
I2qcqtkOJ5dkWQ6+EkrVB2g1W1+hyXoIESr0Cx8g9xsCQQD5A3kBBVr4E1rMVUy6
/6GJzY4ToqRY47CIkIDx5q82Wn98uRQqGuTOnbA3P8V3IbG5dQ1dcplX27X6CFjl
G9Y/AkAQffWHG7DTHdK1nlvbZ3Cv7l/ybxoil3oEu79Nn0MP58LvlZYRp314g9f8
waZBd/QWgXOqkICkd5/LYlTMjd71AkBlUtdq5e31IZMBr/fP43KsqvqT3Ms47DUJ
7Jq7U52Z5UsYygp9c4IE3L82S/milxBFIW71xkrFKD6s5baeSyxrAkAevO4BFSiq
yQQsYIrnOmez2WGSMqFY5bK381IusNQJnOhW7MXJxAAct1Pcoj3VUV1jIy55SvX2
q0IiRxYcrlte
-----END PRIVATE KEY-----
`)
