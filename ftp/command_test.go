package ftp_test

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/url"
	"os"
	"os/exec"
	"testing"
	"time"

	tap "github.com/shogo82148/go-tap"
	"github.com/shogo82148/s3ftpgateway/ftp/ftptest"
	"github.com/shogo82148/s3ftpgateway/vfs/mapfs"
)

type perlExecutor struct {
	// Absolute path for perl
	perl string
}

func newPerlExecutor() (*perlExecutor, error) {
	// does perl exist?
	perl, err := exec.LookPath("perl")
	if err != nil {
		return nil, err
	}

	// does perl have Net::FTP module?
	cmd := exec.Command(perl, "-MNet::FTP", "-e", "")
	if err := cmd.Run(); err != nil {
		return nil, err
	}

	return &perlExecutor{
		perl: perl,
	}, nil
}

func (perl *perlExecutor) Prove(ctx context.Context, t *testing.T, script string, args ...string) {
	tmp, err := ioutil.TempFile("", "ftp_")
	if err != nil {
		t.Error(err)
		return
	}
	defer os.Remove(tmp.Name())
	if _, err := io.WriteString(tmp, script); err != nil {
		t.Error(err)
		return
	}
	if err := tmp.Close(); err != nil {
		t.Error(err)
		return
	}

	args = append([]string{tmp.Name()}, args...)
	cmd := exec.CommandContext(ctx, perl.perl, args...)

	stderr, err := cmd.StderrPipe()
	if err != nil {
		t.Error(err)
	}
	defer stderr.Close()
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			t.Log(scanner.Text())
		}
	}()

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Error(err)
	}
	defer stdout.Close()

	if err := cmd.Start(); err != nil {
		t.Error(err)
		return
	}

	p, err := tap.NewParser(stdout)
	if err != nil {
		t.Error(err)
	} else if suite, err := p.Suite(); err != nil {
		t.Error(err)
	} else {
		for _, result := range suite.Tests {
			if result.Ok {
				t.Log(result)
			} else {
				t.Error(result)
			}
		}
	}

	if err := cmd.Wait(); err != nil {
		t.Error(err)
	}
}

func TestRetr(t *testing.T) {
	perl, err := newPerlExecutor()
	if err != nil {
		t.Skipf("perl is required for this test: %v", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	ts := ftptest.NewServer(mapfs.New(map[string]string{
		"testfile": "Hello ftp!",
	}))
	defer ts.Close()
	ts.Config.Logger = testLogger{t}

	u, err := url.Parse(ts.URL)
	if err != nil {
		t.Fatal(err)
	}

	script := `use utf8;
use strict;
use warnings;
use Test::More;
use Net::FTP;

my $host = shift;
my $ftp = Net::FTP->new($host, Debug => 1) or die "fail to connect ftp server: $@";
ok $ftp->login('anonymous', 'foobar@example.com'), 'login';

my $result = "";
open my $fh, ">", \$result;
ok $ftp->get('testfile', $fh), 'get';
is $result, "Hello ftp!";
ok $ftp->quit(), 'quit';
done_testing;
`

	perl.Prove(ctx, t, script, u.Host)
}

type testLogger struct {
	t *testing.T
}

func (l testLogger) Print(sessionID string, message interface{}) {
	l.t.Logf("%s  %s", sessionID, message)
}

func (l testLogger) Printf(sessionID string, format string, v ...interface{}) {
	l.t.Log(sessionID, fmt.Sprintf(format, v...))
}

func (l testLogger) PrintCommand(sessionID string, command string, params string) {
	l.t.Logf("%s > %s %s", sessionID, command, params)
}

func (l testLogger) PrintResponse(sessionID string, code int, message string) {
	log.Printf("%s < %d %s", sessionID, code, message)
}
