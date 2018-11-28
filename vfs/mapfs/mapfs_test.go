package mapfs

import (
	"context"
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
