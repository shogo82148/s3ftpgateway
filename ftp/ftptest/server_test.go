package ftptest

import (
	"bytes"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/shogo82148/s3ftpgateway/ftp/internal"
	"github.com/sourcegraph/ctxvfs"
)

func TestServer(t *testing.T) {
	curl, err := exec.LookPath("curl")
	if err != nil {
		t.Skip("curl command not found")
		return
	}

	ts := NewServer(ctxvfs.Map(map[string][]byte{
		"testfile": []byte("Hello ftp!"),
	}))
	defer ts.Close()

	var stdout, stderr bytes.Buffer
	cmd := exec.Command(curl, "-s", "-v", ts.URL+"/testfile")
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Log(stderr.String())
		t.Fatal(err)
	}
	if stdout.String() != "Hello ftp!" {
		t.Log(stderr.String())
		t.Errorf("want %s, got %s", "Hello ftp!", stdout.String())
	}
}

func TestServer_ExplicitTLS(t *testing.T) {
	curl, err := exec.LookPath("curl")
	if err != nil {
		t.Skip("curl command not found")
		return
	}

	ts := NewServer(ctxvfs.Map(map[string][]byte{
		"testfile": []byte("Hello ftp!"),
	}))
	defer ts.Close()

	dir, err := ioutil.TempDir("", "ftp-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	cert := filepath.Join(dir, "cert.pem")
	if err := ioutil.WriteFile(cert, internal.LocalhostCert, 0666); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	cmd := exec.Command(curl, "-s", "-v", "--cacert", cert, "--ftp-ssl", ts.URL+"/testfile")
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Log(stderr.String())
		t.Fatal(err)
	}
	if stdout.String() != "Hello ftp!" {
		t.Log(stderr.String())
		t.Errorf("want %s, got %s", "Hello ftp!", stdout.String())
	}
}
