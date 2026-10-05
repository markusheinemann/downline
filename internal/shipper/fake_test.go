package shipper

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"os"
	"path"
	"sync"
	"testing"
)

var _ remoteFS = (*fakeRemoteFS)(nil)

type fakeRemoteFS struct {
	mu     sync.Mutex
	files  map[string][]byte
	dirs   map[string]bool
	failOn map[string]error
}

var errInjected = errors.New("injected error")

type fakeWriter struct {
	remote *fakeRemoteFS
	path   string
	buf    bytes.Buffer
}

func newFakeRemoteFS() *fakeRemoteFS {
	return &fakeRemoteFS{
		files:  make(map[string][]byte),
		dirs:   map[string]bool{".": true},
		failOn: make(map[string]error),
	}
}

func (f *fakeRemoteFS) failAt(op string, p string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failOn[op+":"+path.Clean(p)] = errInjected
}

func (f *fakeRemoteFS) injected(op string, p string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.failOn[op+":"+p]
}

func (f *fakeRemoteFS) MkdirAll(p string) error {
	p = path.Clean(p)

	if err := f.injected("mkdir", p); err != nil {
		return err
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	for p != "." && p != "/" {
		f.dirs[p] = true
		p = path.Dir(p)
	}

	return nil
}

func (f *fakeRemoteFS) Create(p string) (io.WriteCloser, error) {
	p = path.Clean(p)

	if err := f.injected("create", p); err != nil {
		return nil, err
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	if !f.dirs[path.Dir(p)] {
		return nil, &fs.PathError{Op: "create", Path: p, Err: fs.ErrNotExist}
	}

	f.files[p] = []byte{}
	return &fakeWriter{remote: f, path: p}, nil
}

func (f *fakeRemoteFS) Open(p string) (io.ReadCloser, error) {
	p = path.Clean(p)

	if err := f.injected("open", p); err != nil {
		return nil, err
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	data, ok := f.files[p]
	if !ok {
		return nil, &fs.PathError{Op: "open", Path: p, Err: fs.ErrNotExist}
	}

	return io.NopCloser(bytes.NewReader(bytes.Clone(data))), nil
}

func (f *fakeRemoteFS) Rename(oldPath string, newPath string) error {
	oldPath = path.Clean(oldPath)
	newPath = path.Clean(newPath)

	if err := f.injected("rename", oldPath); err != nil {
		return err
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	data, ok := f.files[oldPath]
	if !ok {
		return &os.LinkError{Op: "rename", Old: oldPath, New: newPath, Err: fs.ErrNotExist}
	}

	if !f.dirs[path.Dir(newPath)] {
		return &os.LinkError{Op: "rename", Old: oldPath, New: newPath, Err: fs.ErrNotExist}
	}

	if _, ok := f.files[newPath]; ok {
		return &os.LinkError{Op: "rename", Old: oldPath, New: newPath, Err: fs.ErrExist}
	}

	f.files[newPath] = data
	delete(f.files, oldPath)

	return nil
}

func (f *fakeRemoteFS) Remove(p string) error {
	p = path.Clean(p)

	if err := f.injected("remove", p); err != nil {
		return err
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	if _, ok := f.files[p]; !ok {
		return &fs.PathError{Op: "remove", Path: p, Err: fs.ErrNotExist}
	}

	delete(f.files, p)
	return nil
}

func (w *fakeWriter) Write(p []byte) (n int, err error) {
	if err := w.remote.injected("write", w.path); err != nil {
		return 0, err
	}
	return w.buf.Write(p)
}

func (w *fakeWriter) Close() error {
	if err := w.remote.injected("close", w.path); err != nil {
		return err
	}

	data := bytes.Clone(w.buf.Bytes())

	if w.remote.injected("corrupt", w.path) != nil && len(data) > 0 {
		data[0] ^= 0xFF // flip the first byte
	}

	w.remote.mu.Lock()
	defer w.remote.mu.Unlock()
	w.remote.files[w.path] = data

	return nil
}

func TestFakeRemoteFS_WriteCommitsOnClose(t *testing.T) {
	fake := newFakeRemoteFS()

	mustMkdirAll(t, fake, "a")

	writer, err := fake.Create("a/notes.txt")
	if err != nil {
		t.Fatalf("unexpected error when creating a file that doesn't exist: %v", err)
	}

	_, err = writer.Write([]byte("hello world"))
	if err != nil {
		t.Fatalf("unexpected error when writing to a file: %v", err)
	}

	if _, ok := fake.files["a/notes.txt"]; !ok {
		t.Errorf("expected a file to exist")
	}
	if len(fake.files["a/notes.txt"]) != 0 {
		t.Errorf("expecting file only to be written after Close()")
	}

	err = writer.Close()
	if err != nil {
		t.Fatalf("unexpected error when writing to a file: %v", err)
	}

	if string(fake.files["a/notes.txt"]) != "hello world" {
		t.Errorf("expected notes.txt to be \"hello world\", got %q", string(fake.files["a/notes.txt"]))
	}
}

func TestFakeRemoteFS_Corrupt(t *testing.T) {
	f := newFakeRemoteFS()
	mustMkdirAll(t, f, "a")
	f.failAt("corrupt", "a/x")

	mustWriteFile(t, f, "a/x", "hello world")

	got := mustReadFile(t, f, "a/x")
	if len(got) != len("hello world") {
		t.Errorf("length: got %d, want %d", len(got), len("hello world"))
	}
	if got == "hello world" {
		t.Errorf("expected corrupted content, got the original %q", got)
	}
}
