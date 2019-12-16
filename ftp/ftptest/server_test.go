package ftptest

import (
	"bytes"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/shogo82148/s3ftpgateway/ftp/internal"
	"github.com/shogo82148/s3ftpgateway/vfs/mapfs"
)

func TestServer_EPSV(t *testing.T) {
	curl, err := exec.LookPath("curl")
	if err != nil {
		t.Skip("curl command not found")
		return
	}

	ts := NewUnstartedServer(mapfs.New(map[string]string{
		"testfile": "Hello ftp!",
	}))
	ts.Config.Logger = testLogger{t}
	ts.Start()
	defer ts.Close()

	var stdout, stderr bytes.Buffer
	cmd := exec.Command(curl, "-s", "-v", "--ftp-pasv", ts.URL+"/testfile")
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

func TestServer_PASV(t *testing.T) {
	curl, err := exec.LookPath("curl")
	if err != nil {
		t.Skip("curl command not found")
		return
	}

	ts := NewUnstartedServer(mapfs.New(map[string]string{
		"testfile": "Hello ftp!",
	}))
	ts.Config.Logger = testLogger{t}
	ts.Start()
	defer ts.Close()

	var stdout, stderr bytes.Buffer
	cmd := exec.Command(curl, "-s", "-v", "--ftp-pasv", "--no-epsv", ts.URL+"/testfile")
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

func TestServer_EPRT(t *testing.T) {
	curl, err := exec.LookPath("curl")
	if err != nil {
		t.Skip("curl command not found")
		return
	}

	ts := NewUnstartedServer(mapfs.New(map[string]string{
		"testfile": "Hello ftp!",
	}))
	ts.Config.Logger = testLogger{t}
	ts.Start()
	defer ts.Close()

	var stdout, stderr bytes.Buffer
	cmd := exec.Command(curl, "-s", "-v", "--ftp-port", "-", ts.URL+"/testfile")
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

func TestServer_PORT(t *testing.T) {
	curl, err := exec.LookPath("curl")
	if err != nil {
		t.Skip("curl command not found")
		return
	}

	ts := NewUnstartedServer(mapfs.New(map[string]string{
		"testfile": "Hello ftp!",
	}))
	ts.Config.Logger = testLogger{t}
	ts.Start()
	defer ts.Close()

	var stdout, stderr bytes.Buffer
	cmd := exec.Command(curl, "-s", "-v", "--ftp-port", "-", "--no-eprt", ts.URL+"/testfile")
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

func TestServer_ExplicitTLS_EPSV(t *testing.T) {
	curl, err := exec.LookPath("curl")
	if err != nil {
		t.Skip("curl command not found")
		return
	}

	ts := NewUnstartedServer(mapfs.New(map[string]string{
		"testfile": "Hello ftp!",
	}))
	ts.Config.Logger = testLogger{t}
	ts.StartTLS()
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

	args := []string{ts.URL + "/testfile", "-s", "-v", "--ftp-ssl", "--ftp-pasv", "--cacert", cert}

	var stdout, stderr bytes.Buffer
	cmd := exec.Command(curl, args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	t.Logf("`%s` is finished:\n%s", strings.Join(args, " "), stderr.String())
	if stdout.String() != "Hello ftp!" {
		t.Errorf("want %s, got %s", "Hello ftp!", stdout.String())
	}
}

func TestServer_ExplicitTLS_EPRT(t *testing.T) {
	curl, err := exec.LookPath("curl")
	if err != nil {
		t.Skip("curl command not found")
		return
	}

	ts := NewUnstartedServer(mapfs.New(map[string]string{
		"testfile": "Hello ftp!",
	}))
	ts.Config.Logger = testLogger{t}
	ts.Start()
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

	args := []string{ts.URL + "/testfile", "-s", "-v", "--ftp-ssl", "--ftp-port", "-"}
	if runtime.GOOS != "windows" {
		args = append(args, "--cacert", cert)
	} else {
		// curl does not accept --cacert option in windows, why???
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

func TestServer_ImplictTLS_EPSV(t *testing.T) {
	curl, err := exec.LookPath("curl")
	if err != nil {
		t.Skip("curl command not found")
		return
	}

	ts := NewUnstartedServer(mapfs.New(map[string]string{
		"testfile": "Hello ftp!",
	}))
	ts.Config.Logger = testLogger{t}
	ts.StartTLS()
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

	args := []string{ts.URL + "/testfile", "-s", "-v", "--ftp-ssl", "--ftp-pasv"}
	if runtime.GOOS != "windows" {
		args = append(args, "--cacert", cert)
	} else {
		// curl does not accept --cacert option in windows, why???
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

func TestServer_ImplicitTLS_EPRT(t *testing.T) {
	curl, err := exec.LookPath("curl")
	if err != nil {
		t.Skip("curl command not found")
		return
	}

	ts := NewUnstartedServer(mapfs.New(map[string]string{
		"testfile": "Hello ftp!",
	}))
	ts.Config.Logger = testLogger{t}
	ts.StartTLS()
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

	args := []string{ts.URL + "/testfile", "-s", "-v", "--ftp-ssl", "--ftp-port", "-"}
	if runtime.GOOS != "windows" {
		args = append(args, "--cacert", cert)
	} else {
		// curl does not accept --cacert option in windows, why???
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
