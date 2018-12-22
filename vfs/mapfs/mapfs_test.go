package mapfs

import (
	"context"
	"io"
	"io/ioutil"
	"os"
	"reflect"
	"testing"
)

func TestOpen(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fs := New(map[string]string{
		"foo/bar/three.txt": "a",
		"foo/bar.txt":       "b",
		"top.txt":           "c",
		"other-top.txt":     "d",
	})
	tests := []struct {
		path string
		want string
	}{
		{"/foo/bar/three.txt", "a"},
		{"foo/bar/three.txt", "a"},
		{"foo/bar.txt", "b"},
		{"top.txt", "c"},
		{"/top.txt", "c"},
		{"other-top.txt", "d"},
		{"/other-top.txt", "d"},
		{"foo/bar/../bar.txt", "b"},
	}
	for _, tt := range tests {
		rsc, err := fs.Open(ctx, tt.path)
		if err != nil {
			t.Errorf("Open(%q) = %v", tt.path, err)
			continue
		}
		slurp, err := ioutil.ReadAll(rsc)
		if err != nil {
			t.Error(err)
		}
		if string(slurp) != tt.want {
			t.Errorf("Read(%q) = %q; want %q", tt.path, tt.want, slurp)
		}
		rsc.Close()
	}

	_, err := fs.Open(ctx, "/xxxx")
	if !os.IsNotExist(err) {
		t.Errorf("ReadDir /xxxx = %v; want os.IsNotExist error", err)
	}
}

func TestLstat(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fs := New(map[string]string{
		"foo/bar/three.txt": "333",
		"foo/bar.txt":       "22",
		"top.txt":           "top.txt file",
		"other-top.txt":     "other-top.txt file",
	})
	tests := []struct {
		path string
		want os.FileInfo
	}{
		{
			path: "foo",
			want: mapFI{name: "foo", dir: true},
		},
		{
			path: "foo/bar.txt",
			want: mapFI{name: "bar.txt", size: 2},
		},
	}
	for _, tt := range tests {
		fis, err := fs.Lstat(ctx, tt.path)
		if err != nil {
			t.Errorf("Lstat(%q) = %v", tt.path, err)
			continue
		}
		if !reflect.DeepEqual(fis, tt.want) {
			t.Errorf("Lstat(%q) = %#v; want %#v", tt.path, fis, tt.want)
			continue
		}
	}

	_, err := fs.ReadDir(ctx, "/xxxx")
	if !os.IsNotExist(err) {
		t.Errorf("Lstat /xxxx = %v; want os.IsNotExist error", err)
	}
}

func TestReaddir(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fs := New(map[string]string{
		"foo/bar/three.txt": "333",
		"foo/bar.txt":       "22",
		"top.txt":           "top.txt file",
		"other-top.txt":     "other-top.txt file",
	})
	tests := []struct {
		dir  string
		want []os.FileInfo
	}{
		{
			dir: "/",
			want: []os.FileInfo{
				mapFI{name: "foo", dir: true},
				mapFI{name: "other-top.txt", size: len("other-top.txt file")},
				mapFI{name: "top.txt", size: len("top.txt file")},
			},
		},
		{
			dir: "/foo",
			want: []os.FileInfo{
				mapFI{name: "bar", dir: true},
				mapFI{name: "bar.txt", size: 2},
			},
		},
		{
			dir: "/foo/",
			want: []os.FileInfo{
				mapFI{name: "bar", dir: true},
				mapFI{name: "bar.txt", size: 2},
			},
		},
		{
			dir: "/foo/bar",
			want: []os.FileInfo{
				mapFI{name: "three.txt", size: 3},
			},
		},
	}
	for _, tt := range tests {
		fis, err := fs.ReadDir(ctx, tt.dir)
		if err != nil {
			t.Errorf("ReadDir(%q) = %v", tt.dir, err)
			continue
		}
		if !reflect.DeepEqual(fis, tt.want) {
			t.Errorf("ReadDir(%q) = %#v; want %#v", tt.dir, fis, tt.want)
			continue
		}
	}

	_, err := fs.ReadDir(ctx, "/xxxx")
	if !os.IsNotExist(err) {
		t.Errorf("ReadDir /xxxx = %v; want os.IsNotExist error", err)
	}
}

func TestCreate(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create new file
	fs := New(map[string]string{})
	w, err := fs.Create(ctx, "foobar.txt")
	if err != nil {
		t.Fatal("unexpected error: ", err)
	}
	n, err := io.WriteString(w, "hello")
	if err != nil {
		t.Fatal("unexpected error: ", err)
	}
	if n != len("hello") {
		t.Errorf("got %d, want %d", n, len("hello"))
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	// Check the content of th file.
	r, err := fs.Open(ctx, "foobar.txt")
	slurp, err := ioutil.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	if string(slurp) != "hello" {
		t.Errorf("got %s, want Hello World", string(slurp))
	}
}

func TestMkdir(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	t.Run("success", func(t *testing.T) {
		fs := New(map[string]string{})
		if err := fs.Mkdir(ctx, "foo"); err != nil {
			t.Error("unexpected error: ", err)
		}
	})

	t.Run("dir-already-exists", func(t *testing.T) {
		fs := New(map[string]string{
			"foo/": "",
		})
		if err := fs.Mkdir(ctx, "foo"); err == nil || !os.IsExist(err) {
			t.Errorf("want exist, got %v", err)
		}
	})

	t.Run("file-already-exists", func(t *testing.T) {
		fs := New(map[string]string{
			"foo": "a",
		})
		if err := fs.Mkdir(ctx, "foo"); err == nil || !os.IsExist(err) {
			t.Errorf("want exist, got %v", err)
		}
	})
}

func TestRemove(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fs := New(map[string]string{
		"foo/bar/":          "",
		"foo/bar/three.txt": "a",
		"foo/bar.txt":       "b",
		"top.txt":           "c",
		"other-top.txt":     "d",
	})

	if err := fs.Remove(ctx, "foo/bar.txt"); err != nil {
		t.Error("unexpected error: ", err)
	}

	// it will fail because directory is not empty
	err := fs.Remove(ctx, "foo/bar")
	if err == nil || err.(*os.PathError).Err.Error() != "directory is not empty" {
		t.Error("unexpected error: ", err)
	}

	// remove all
	if err := fs.Remove(ctx, "foo/bar/three.txt"); err != nil {
		t.Error("unexpected error: ", err)
	}
	if stat, err := fs.Lstat(ctx, "foo/bar"); err != nil || !stat.IsDir() {
		t.Error("want path `foo/bar` is directory, but not")
	}
	if err := fs.Remove(ctx, "foo/bar"); err != nil {
		t.Error("unexpected error: ", err)
	}

	// "foo/bar" is already moved, so it fails
	if err := fs.Remove(ctx, "foo/bar"); !os.IsNotExist(err) {
		t.Error("unexpected error: ", err)
	}
}
