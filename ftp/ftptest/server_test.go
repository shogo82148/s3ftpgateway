package ftptest

import (
	"bytes"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/shogo82148/s3ftpgateway/ftp/internal"
	"github.com/shogo82148/s3ftpgateway/vfs/mapfs"
)

func TestServer(t *testing.T) {
	curl, err := exec.LookPath("curl")
	if err != nil {
		t.Skip("curl command not found")
		return
	}

	ts := NewServer(mapfs.New(map[string]string{
		"testfile": "Hello ftp!",
	}))
	defer ts.Close()
	ts.Config.Logger = testLogger{t}

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

	ts := NewServer(mapfs.New(map[string]string{
		"testfile": "Hello ftp!",
	}))
	defer ts.Close()
	ts.Config.Logger = testLogger{t}

	dir, err := ioutil.TempDir("", "ftp-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	cert := filepath.Join(dir, "cert.pem")
	if err := ioutil.WriteFile(cert, internal.LocalhostCert, 0666); err != nil {
		t.Fatal(err)
	}

	args := []string{ts.URL + "/testfile", "-s", "-v", "--ftp-ssl"}
	if runtime.GOOS != "windows" {
		args = append(args, "--cacert", cert)
	} else {
		// curl does not accpect --cacert option in windows, why???
		args = append(args, "-k")
	}

	var stdout, stderr bytes.Buffer
	cmd := exec.Command(curl, args...)
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

func TestServer_ImplictTLS(t *testing.T) {
	curl, err := exec.LookPath("curl")
	if err != nil {
		t.Skip("curl command not found")
		return
	}

	ts := NewServer(mapfs.New(map[string]string{
		"testfile": "Hello ftp!",
	}))
	defer ts.Close()
	ts.Config.Logger = testLogger{t}

	dir, err := ioutil.TempDir("", "ftp-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	cert := filepath.Join(dir, "cert.pem")
	if err := ioutil.WriteFile(cert, internal.LocalhostCert, 0666); err != nil {
		t.Fatal(err)
	}

	args := []string{ts.URL + "/testfile", "-s", "-v", "--ftp-ssl"}
	if runtime.GOOS != "windows" {
		args = append(args, "--cacert", cert)
	} else {
		// curl does not accpect --cacert option in windows, why???
		args = append(args, "-k")
	}

	var stdout, stderr bytes.Buffer
	cmd := exec.Command(curl, args...)
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
