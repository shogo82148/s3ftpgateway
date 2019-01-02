package ftp_test

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"io/ioutil"
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

func TestAppe(t *testing.T) {
	perl, err := newPerlExecutor()
	if err != nil {
		t.Skipf("perl is required for this test: %v", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	fs := mapfs.New(map[string]string{
		"foobar.txt": "Hello",
	})
	ts := ftptest.NewUnstartedServer(fs)
	ts.Config.Logger = testLogger{t}
	ts.Start()
	defer ts.Close()

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

my $content = " ftp!";
open my $fh, '<', \$content;
ok $ftp->append($fh, 'foobar.txt'), 'append';
ok $ftp->quit;

done_testing;
`

	perl.Prove(ctx, t, script, u.Host)

	r, err := fs.Open(ctx, "foobar.txt")
	if err != nil {
		t.Fatal(err)
	}
	b, err := ioutil.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "Hello ftp!" {
		t.Errorf("want Hello ftp!, got %s", b)
	}
}

func TestDele(t *testing.T) {
	perl, err := newPerlExecutor()
	if err != nil {
		t.Skipf("perl is required for this test: %v", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	ts := ftptest.NewUnstartedServer(mapfs.New(map[string]string{
		"foobar.txt": "Hello ftp!",
	}))
	ts.Config.Logger = testLogger{t}
	ts.Start()
	defer ts.Close()

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

ok $ftp->rmdir('foobar.txt'), 'remove a file';
ok !$ftp->rmdir('foobar.txt'), 'file not found';
ok $ftp->quit;

done_testing;
`

	perl.Prove(ctx, t, script, u.Host)
}

func TestPwd(t *testing.T) {
	perl, err := newPerlExecutor()
	if err != nil {
		t.Skipf("perl is required for this test: %v", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	ts := ftptest.NewUnstartedServer(mapfs.New(map[string]string{
		"foo/bar/hoge/fuga.txt": "Hello ftp!",
	}))
	ts.Config.Logger = testLogger{t}
	ts.Start()
	defer ts.Close()

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

is $ftp->pwd(), '/', 'initial currect working directory';
ok $ftp->cwd('/foo/bar'), 'cwd /foo/bar';
is $ftp->pwd(), '/foo/bar';

ok !$ftp->cwd('/not-exist'), 'not such directory';
is $ftp->pwd(), '/foo/bar';

ok !$ftp->cwd('/foo/bar/hoge/fuga.txt'), 'not directory';
is $ftp->pwd(), '/foo/bar';

ok $ftp->cdup(), 'CDUP';
is $ftp->pwd(), '/foo';

ok $ftp->cdup(), 'CDUP';
is $ftp->pwd(), '/';

ok !$ftp->cdup(), 'try to CDUP to out side of the root';
is $ftp->pwd(), '/';

done_testing;
`

	perl.Prove(ctx, t, script, u.Host)
}

func TestList(t *testing.T) {
	perl, err := newPerlExecutor()
	if err != nil {
		t.Skipf("perl is required for this test: %v", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	ts := ftptest.NewUnstartedServer(mapfs.New(map[string]string{
		"foo/bar/hoge.txt": "abc123",
		"hogehoge.txt":     "foobar",
	}))
	ts.Config.Logger = testLogger{t}
	ts.Start()
	defer ts.Close()

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
my @files = $ftp->dir();
is $files[0], 'drwxr-xr-x 1 anonymous anonymous             0  Jan  1 00:00 foo';
is $files[1], '-rw-r--r-- 1 anonymous anonymous             6  Jan  1 00:00 hogehoge.txt';
ok $ftp->quit();
done_testing;
`

	perl.Prove(ctx, t, script, u.Host)
}

func TestMkd(t *testing.T) {
	perl, err := newPerlExecutor()
	if err != nil {
		t.Skipf("perl is required for this test: %v", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	ts := ftptest.NewUnstartedServer(mapfs.New(map[string]string{}))
	ts.Config.Logger = testLogger{t}
	ts.Start()
	defer ts.Close()

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

ok $ftp->mkdir('foo'), 'make new directory';
# Net::FTP doesn't read the reply, so need to check it directly :(
like $ftp->message(), qr(^"/foo" ), 'message';
ok !$ftp->mkdir('foo'), 'the directory already exists';
ok $ftp->cwd('/foo');

ok $ftp->mkdir('bar" hoge'), 'includes quote';
like $ftp->message(), qr(^"/foo/bar"" hoge" ), 'message';
ok $ftp->cwd('/foo/bar" hoge');

done_testing;
`

	perl.Prove(ctx, t, script, u.Host)
}

func TestNlst(t *testing.T) {
	perl, err := newPerlExecutor()
	if err != nil {
		t.Skipf("perl is required for this test: %v", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	ts := ftptest.NewServer(mapfs.New(map[string]string{
		"foo/bar/hoge.txt": "abc123",
		"hogehoge.txt":     "foobar",
	}))
	ts.Config.Logger = testLogger{t}
	ts.Start()
	defer ts.Close()

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
my @files = $ftp->ls();
is $files[0], 'foo';
is $files[1], 'hogehoge.txt';
ok $ftp->quit();
done_testing;
`

	perl.Prove(ctx, t, script, u.Host)
}

func TestPortPasv(t *testing.T) {
	perl, err := newPerlExecutor()
	if err != nil {
		t.Skipf("perl is required for this test: %v", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	ts1 := ftptest.NewUnstartedServer(mapfs.New(map[string]string{
		"testfile": "Hello ftp!",
	}))
	ts1.Config.Logger = testLogger{t}
	ts1.Start()
	defer ts1.Close()
	u1, err := url.Parse(ts1.URL)
	if err != nil {
		t.Fatal(err)
	}

	fs := mapfs.New(map[string]string{})
	ts2 := ftptest.NewUnstartedServer(fs)
	ts2.Config.Logger = testLogger{t}
	ts2.Start()
	defer ts2.Close()
	u2, err := url.Parse(ts2.URL)
	if err != nil {
		t.Fatal(err)
	}

	script := `use utf8;
use strict;
use warnings;
use Test::More;
use Net::FTP;
use Net::Cmd;

my $host1 = shift;
my $host2 = shift;
my $src = Net::FTP->new($host1, Debug => 1) or die "fail to connect ftp server: $@";
ok $src->login('anonymous', 'foobar@example.com'), 'login';
my $dst = Net::FTP->new($host2, Debug => 1) or die "fail to connect ftp server: $@";
ok $dst->login('anonymous', 'foobar@example.com'), 'login';
ok my $port = $src->pasv(), 'pasv';
ok $dst->port($port), 'port';
ok $dst->stor('testfile');
ok $src->retr('testfile');
is $src->response, Net::Cmd::CMD_INFO, 'response';
ok $dst->pasv_wait($src);

ok !$dst->port(['127.0.0.1', 80]), 'well-known port is denied';
ok !$dst->port(['192.0.2.3', 8080]), 'another IP address is denied';

ok $src->quit(), 'quit';
ok $dst->quit(), 'quit';
done_testing;
`
	perl.Prove(ctx, t, script, u1.Host, u2.Host)

	r, err := fs.Open(ctx, "testfile")
	if err != nil {
		t.Fatal(err)
	}
	b, err := ioutil.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "Hello ftp!" {
		t.Errorf("want Hello ftp!, got %s", b)
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

	ts := ftptest.NewUnstartedServer(mapfs.New(map[string]string{
		"testfile": "Hello ftp!",
	}))
	ts.Config.Logger = testLogger{t}
	ts.Start()
	defer ts.Close()

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

func TestRmd(t *testing.T) {
	perl, err := newPerlExecutor()
	if err != nil {
		t.Skipf("perl is required for this test: %v", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	ts := ftptest.NewUnstartedServer(mapfs.New(map[string]string{
		"foo/bar/": "",
	}))
	ts.Config.Logger = testLogger{t}
	ts.Start()
	defer ts.Close()

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

ok $ftp->rmdir('/foo/bar'), 'remove a directory';
ok !$ftp->rmdir('/foo/bar'), 'directory not found';
ok $ftp->quit;

done_testing;
`

	perl.Prove(ctx, t, script, u.Host)
}

func TestRename(t *testing.T) {
	perl, err := newPerlExecutor()
	if err != nil {
		t.Skipf("perl is required for this test: %v", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	ts := ftptest.NewUnstartedServer(mapfs.New(map[string]string{
		"foo.txt": "hello",
	}))
	ts.Start()
	ts.Config.Logger = testLogger{t}
	defer ts.Close()

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

ok $ftp->rename('foo.txt', 'bar.txt'), 'rename';
ok !$ftp->rename('foo.txt', 'bar.txt'), 'rename';
ok $ftp->rename('bar.txt', 'foo.txt'), 'rename';

ok $ftp->quit;

done_testing;
`

	perl.Prove(ctx, t, script, u.Host)
}

func TestStat(t *testing.T) {
	perl, err := newPerlExecutor()
	if err != nil {
		t.Skipf("perl is required for this test: %v", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	ts := ftptest.NewUnstartedServer(mapfs.New(map[string]string{
		"foo.txt": "hello",
	}))
	ts.Config.Logger = testLogger{t}
	ts.Start()
	defer ts.Close()

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

# force to use STAT command to get size.
${*$ftp}{net_ftp_supported} = {
	SIZE => 0,
	STAT => 1,
};

is $ftp->size('foo.txt'), 5, "stat";

ok $ftp->quit;

done_testing;
`

	perl.Prove(ctx, t, script, u.Host)
}

func TestStor(t *testing.T) {
	perl, err := newPerlExecutor()
	if err != nil {
		t.Skipf("perl is required for this test: %v", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	fs := mapfs.New(map[string]string{})
	ts := ftptest.NewUnstartedServer(fs)
	ts.Config.Logger = testLogger{t}
	ts.Start()
	defer ts.Close()

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

my $content = "Hello ftp!";
open my $fh, "<", \$content;
ok $ftp->put($fh, 'testfile'), 'put';
ok $ftp->quit(), 'quit';
done_testing;
`

	perl.Prove(ctx, t, script, u.Host)

	r, err := fs.Open(ctx, "testfile")
	if err != nil {
		t.Error(err)
	}
	b, err := ioutil.ReadAll(r)
	if string(b) != "Hello ftp!" {
		t.Errorf("want Hello ftp!, got %s", b)
	}
}

func TestStou(t *testing.T) {
	perl, err := newPerlExecutor()
	if err != nil {
		t.Skipf("perl is required for this test: %v", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	fs := mapfs.New(map[string]string{})
	ts := ftptest.NewUnstartedServer(fs)
	ts.Config.Logger = testLogger{t}
	ts.Start()
	defer ts.Close()

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

eval {
	my $name1 = do {
		my $content = "Hello ftp!";
		open my $fh, "<", \$content;
		ok my $name = $ftp->put_unique($fh), 'put_unique';
		close $fh;
		$name;
	};
	
	my $name2 = do {
		my $content = "Hello ftp!";
		open my $fh, "<", \$content;
		ok my $name = $ftp->put_unique($fh), 'put_unique';
		close $fh;
		$name;
	};
	
	isnt $name1, $name2;
};
if ($@ =~ /Must specify remote filename with stream input/i) {
	# put_unique doesn't work correctly on old version of Net::FTP.
	plan skip_all => 'put_unique is not support';
}

ok $ftp->quit(), 'quit';
done_testing;
`

	perl.Prove(ctx, t, script, u.Host)
}

func TestFeat(t *testing.T) {
	perl, err := newPerlExecutor()
	if err != nil {
		t.Skipf("perl is required for this test: %v", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	fs := mapfs.New(map[string]string{})
	ts := ftptest.NewUnstartedServer(fs)
	ts.Config.Logger = testLogger{t}
	ts.Start()
	defer ts.Close()

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

ok $ftp->feature('EPRT'), 'EPRT is supported';
ok !$ftp->feature('HOGEFUGA'), 'HOGEFUGA is not supported';

ok $ftp->quit(), 'quit';
done_testing;
`

	perl.Prove(ctx, t, script, u.Host)
}

func TestEprt(t *testing.T) {
	perl, err := newPerlExecutor()
	if err != nil {
		t.Skipf("perl is required for this test: %v", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	ts := ftptest.NewUnstartedServer(mapfs.New(map[string]string{
		"testfile": "Hello ftp!",
	}))
	ts.Config.Logger = testLogger{t}
	ts.Start()
	defer ts.Close()

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

if (!$ftp->can('eprt')) {
	plan skip_all => 'eprt is not support';
}
ok $ftp->eprt(), 'eprt';
ok my $fh = $ftp->retr('testfile'), 'retr';
my $content = do { local $/ = ''; <$fh>};
is $content, 'Hello ftp!', 'content';
ok $ftp->quit(), 'quit';
done_testing;
`
	perl.Prove(ctx, t, script, u.Host)
}

func TestLang(t *testing.T) {
	perl, err := newPerlExecutor()
	if err != nil {
		t.Skipf("perl is required for this test: %v", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	ts := ftptest.NewUnstartedServer(mapfs.New(map[string]string{}))
	ts.Config.Logger = testLogger{t}
	ts.Start()
	defer ts.Close()

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
is $ftp->quot('LANG', 'en'), 2, 'English is supported';
is $ftp->quot('LANG', 'ja'), 5, 'Japanese is not supported';
ok $ftp->quit(), 'quit';
done_testing;
`
	perl.Prove(ctx, t, script, u.Host)
}

func TestMdtm(t *testing.T) {
	perl, err := newPerlExecutor()
	if err != nil {
		t.Skipf("perl is required for this test: %v", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	ts := ftptest.NewUnstartedServer(mapfs.New(map[string]string{
		"foobar.txt": "hello",
	}))
	ts.Config.Logger = testLogger{t}
	ts.Start()
	defer ts.Close()

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
ok my $mdtm = $ftp->mdtm('foobar.txt'), 'MDTM';
is $mdtm, -62135596800, 'January 1, year 1, 00:00:00.000000000 UTC';
ok $ftp->quit(), 'quit';
# done_testing;
`
	perl.Prove(ctx, t, script, u.Host)
}

type testLogger struct {
	t *testing.T
}

func (l testLogger) Print(sessionID string, message interface{}) {
	l.t.Helper()
	l.t.Logf("%s  %s", sessionID, message)
}

func (l testLogger) Printf(sessionID string, format string, v ...interface{}) {
	l.t.Helper()
	l.t.Log(sessionID, " ", fmt.Sprintf(format, v...))
}

func (l testLogger) PrintCommand(sessionID string, command string, params string) {
	l.t.Helper()
	l.t.Logf("%s > %s %s", sessionID, command, params)
}

func (l testLogger) PrintResponse(sessionID string, code int, message string) {
	l.t.Helper()
	l.t.Logf("%s < %d %s", sessionID, code, message)
}
