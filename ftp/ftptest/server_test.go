package ftptest

import (
	"bytes"
	"os/exec"
	"testing"

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
