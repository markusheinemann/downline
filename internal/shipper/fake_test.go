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

	w.remote.mu.Lock()
	defer w.remote.mu.Unlock()

	w.remote.files[w.path] = bytes.Clone(w.buf.Bytes())
	return nil
}

func seedTestDir(t *testing.T, fake *fakeRemoteFS, path string) {
	t.Helper()

	err := fake.MkdirAll(path)
	if err != nil {
		t.Fatalf("failed to mkdir %s: %v", path, err)
	}
}

func seedTestFile(t *testing.T, fake *fakeRemoteFS, path string, content []byte) {
	t.Helper()

	f, err := fake.Create(path)
	if err != nil {
		t.Fatalf("failed to open %s: %v", path, err)
	}

	_, err = f.Write(content)
	if err != nil {
		t.Fatalf("failed to write to %s: %v", path, err)
	}

	err = f.Close()
	if err != nil {
		t.Fatalf("failed to close %s: %v", path, err)
	}
}

func TestFakeFS_MkdirAll(t *testing.T) {
	fake := newFakeRemoteFS()

	err := fake.MkdirAll("a/b/c")
	if err != nil {
		t.Fatal(err)
	}

	if !fake.dirs["a"] {
		t.Errorf("expected a to be a dir but it dose not exists")
	}
	if !fake.dirs["a/b"] {
		t.Errorf("expected a/b to be a dir but it dose not exists")
	}
	if !fake.dirs["a/b/c"] {
		t.Errorf("expected a/b/c to be a dir but it dose exists")
	}
}

func TestFakeFS_Create(t *testing.T) {
	fake := newFakeRemoteFS()

	writer, err := fake.Create("a/notes.txt")
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("expected error ErrNotExist but got %v", err)
	}
	if writer != nil {
		t.Errorf("expected writer to be nil when creating a file in a folder that doesn't exist")
	}

	seedTestDir(t, fake, "a")

	writer, err = fake.Create("a/notes.txt")
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

func TestFakeFS_Open(t *testing.T) {
	fake := newFakeRemoteFS()
	seedTestFile(t, fake, "notes.txt", []byte("hello world"))

	f, err := fake.Open("notes.txt")
	if err != nil {
		t.Fatalf("unexpected error when opening a file: %v", err)
	}

	data, err := io.ReadAll(f)
	if err != nil {
		t.Fatalf("unexpected error when reading from a file: %v", err)
	}
	if string(data) != "hello world" {
		t.Errorf("expected data to be \"hello world\", got %q", string(data))
	}

	// open missing file on existing folder
	seedTestDir(t, fake, "a")
	_, err = fake.Open("a/not_exists.txt")
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("expected os.ErrNotExist error, got %v", err)
	}
}

func TestFakeFS_Rename(t *testing.T) {
	fake := newFakeRemoteFS()

	seedTestDir(t, fake, "a")
	seedTestDir(t, fake, "b")
	seedTestFile(t, fake, "a/notes.txt", []byte("hello world"))

	// normal case works
	err := fake.Rename("a/notes.txt", "b/notizen.txt")
	if err != nil {
		t.Fatalf("unexpected error when renaming a file: %v", err)
	}
	if _, ok := fake.files["a/notes.txt"]; ok {
		t.Errorf("expected old file to be removed")
	}
	if _, ok := fake.files["b/notizen.txt"]; !ok {
		t.Errorf("expected file to exist")
	}

	// Fails if the target file already exists
	seedTestFile(t, fake, "a/notes.txt", []byte("hello world"))
	err = fake.Rename("a/notes.txt", "b/notizen.txt")
	if !errors.Is(err, fs.ErrExist) {
		t.Errorf("expected ErrExist got: %v", err)
	}

	// Fails if the target folder dose not exists
	err = fake.Rename("b/notizen.txt", "c/notes.txt")
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("expected ErrNotExist got: %v", err)
	}
	// should not remove original file on error
	if _, ok := fake.files["b/notizen.txt"]; !ok {
		t.Errorf("expected renamed filed not to be deleted on error")
	}
}

func TestFakeFS_Remove(t *testing.T) {
	fake := newFakeRemoteFS()
	seedTestFile(t, fake, "notes.txt", []byte("hello world"))

	// first remove should pass
	err := fake.Remove("notes.txt")
	if err != nil {
		t.Fatalf("unexpected error when removing a file: %v", err)
	}

	// second remove should fail because the file dose not exists
	err = fake.Remove("notes.txt")
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("expected ErrNotExist got: %v", err)
	}
}
