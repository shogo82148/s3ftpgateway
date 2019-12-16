package ftptest

import (
	"bytes"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
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
	args := []string{"-s", "-v", "--ftp-pasv", ts.URL + "/testfile"}
	cmd := exec.Command(curl, args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Error(err)
	}
	t.Logf("`curl %s` is finished:\n%s", strings.Join(args, " "), stderr.String())
	if stdout.String() != "Hello ftp!" {
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
	args := []string{"-s", "-v", "--ftp-pasv", "--no-epsv", ts.URL + "/testfile"}
	cmd := exec.Command(curl, args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Error(err)
	}
	t.Logf("`curl %s` is finished:\n%s", strings.Join(args, " "), stderr.String())
	if stdout.String() != "Hello ftp!" {
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
	args := []string{"-s", "-v", "--ftp-port", "-", ts.URL + "/testfile"}
	cmd := exec.Command(curl, args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Error(err)
	}
	t.Logf("`curl %s` is finished:\n%s", strings.Join(args, " "), stderr.String())
	if stdout.String() != "Hello ftp!" {
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
	args := []string{"-s", "-v", "--ftp-port", "-", "--no-eprt", ts.URL + "/testfile"}
	cmd := exec.Command(curl, args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Error(err)
	}
	t.Logf("`curl %s` is finished:\n%s", strings.Join(args, " "), stderr.String())
	if stdout.String() != "Hello ftp!" {
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

	args := []string{ts.URL + "/testfile", "-s", "-v", "--ftp-ssl", "--ftp-pasv"}

	var stdout, stderr bytes.Buffer
	cmd := exec.Command(curl, append(args, "--cacert", cert)...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if strings.Contains(stderr.String(), "WARNING: using IP address, SNI is being disabled by the OS.") {
			t.Log("certificate verification failed, skip it")
			stderr.Reset()
			stdout.Reset()
			cmd = exec.Command(curl, append(args, "-k")...)
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			if err := cmd.Run(); err != nil {
				t.Error(err)
			}
		} else {
			t.Error(err)
		}
	}
	t.Logf("`curl %s` is finished:\n%s", strings.Join(cmd.Args, " "), stderr.String())
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

	var stdout, stderr bytes.Buffer
	cmd := exec.Command(curl, append(args, "--cacert", cert)...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if strings.Contains(stderr.String(), "WARNING: using IP address, SNI is being disabled by the OS.") {
			t.Log("certificate verification failed, skip it")
			stderr.Reset()
			stdout.Reset()
			cmd = exec.Command(curl, append(args, "-k")...)
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			if err := cmd.Run(); err != nil {
				t.Error(err)
			}
		} else {
			t.Error(err)
		}
	}
	t.Logf("`curl %s` is finished:\n%s", strings.Join(cmd.Args, " "), stderr.String())
	if stdout.String() != "Hello ftp!" {
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

	var stdout, stderr bytes.Buffer
	cmd := exec.Command(curl, append(args, "--cacert", cert)...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if strings.Contains(stderr.String(), "WARNING: using IP address, SNI is being disabled by the OS.") {
			t.Log("certificate verification failed, skip it")
			stderr.Reset()
			stdout.Reset()
			cmd = exec.Command(curl, append(args, "-k")...)
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			if err := cmd.Run(); err != nil {
				t.Error(err)
			}
		} else {
			t.Error(err)
		}
	}
	t.Logf("`curl %s` is finished:\n%s", strings.Join(cmd.Args, " "), stderr.String())
	if stdout.String() != "Hello ftp!" {
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

	var stdout, stderr bytes.Buffer
	cmd := exec.Command(curl, append(args, "--cacert", cert)...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if strings.Contains(stderr.String(), "WARNING: using IP address, SNI is being disabled by the OS.") {
			t.Log("certificate verification failed, skip it")
			stderr.Reset()
			stdout.Reset()
			cmd = exec.Command(curl, append(args, "-k")...)
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			if err := cmd.Run(); err != nil {
				t.Error(err)
			}
		} else {
			t.Error(err)
		}
	}
	t.Logf("`curl %s` is finished:\n%s", strings.Join(cmd.Args, " "), stderr.String())
	if stdout.String() != "Hello ftp!" {
		t.Errorf("want %s, got %s", "Hello ftp!", stdout.String())
	}
}
