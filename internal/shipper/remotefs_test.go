package shipper

import (
	"errors"
	"io"
	"io/fs"
	"testing"
)

func TestFakeRemoteFS_Contract(t *testing.T) {
	testRemoteFSContract(t, func(t *testing.T) remoteFS {
		return newFakeRemoteFS()
	})
}

func testRemoteFSContract(t *testing.T, newFS func(t *testing.T) remoteFS) {
	t.Run("open missing file returns ErrNotExist", func(t *testing.T) {
		fsys := newFS(t)
		mustMkdirAll(t, fsys, "a")

		_, err := fsys.Open("a/missing")
		if !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("got %v, want fs.ErrNotExist", err)
		}
	})

	t.Run("rename onto existing file fails and keeps target", func(t *testing.T) {
		fsys := newFS(t)
		mustMkdirAll(t, fsys, "a")
		mustWriteFile(t, fsys, "a/target", "old")
		mustWriteFile(t, fsys, "a/source", "new")

		err := fsys.Rename("a/source", "a/target")
		if !errors.Is(err, fs.ErrExist) {
			t.Fatalf("got %v, want fs.ErrExist", err)
		}
		if got := mustReadFile(t, fsys, "a/target"); got != "old" {
			t.Errorf("target content: got %q, want %q", got, "old")
		}
		if got := mustReadFile(t, fsys, "a/source"); got != "new" {
			t.Errorf("source content: got %q, want %q", got, "new")
		}
	})

	t.Run("rename moves the file", func(t *testing.T) {
		fsys := newFS(t)

		mustMkdirAll(t, fsys, "a")
		mustWriteFile(t, fsys, "a/source", "content")

		err := fsys.Rename("a/source", "a/target")
		if err != nil {
			t.Fatalf("rename: expected no error, got %v", err)
		}
		if got := mustReadFile(t, fsys, "a/target"); got != "content" {
			t.Errorf("target content: got %q, want %q", got, "content")
		}
		if _, err := fsys.Open("a/source"); !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("source after rename: got %v, want fs.ErrNotExist", err)
		}
	})

	t.Run("create without a parent folder fails", func(t *testing.T) {
		fsys := newFS(t)
		_, err := fsys.Create("a/target")
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
	})

	t.Run("MkdirAll passes on existing folder", func(t *testing.T) {
		fsys := newFS(t)
		mustMkdirAll(t, fsys, "a/b")

		err := fsys.MkdirAll("a/b")
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
	})

	t.Run("MkdirAll creates missing parents", func(t *testing.T) {
		fsys := newFS(t)

		mustMkdirAll(t, fsys, "a/b/c")
		mustWriteFile(t, fsys, "a/b/c/target", "new")
	})

	t.Run("content is written after close", func(t *testing.T) {
		fsys := newFS(t)
		expectedContent := "hello world"

		mustWriteFile(t, fsys, "notes.txt", expectedContent)
		if got := mustReadFile(t, fsys, "notes.txt"); got != expectedContent {
			t.Errorf("got %q, want %q", got, expectedContent)
		}
	})

	t.Run("remove on a missing file fails", func(t *testing.T) {
		fsys := newFS(t)

		err := fsys.Remove("not-existent.txt")
		if !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("expected error to be ErrNotExist, got: %v", err)
		}
	})

	t.Run("remove deletes the file", func(t *testing.T) {
		fsys := newFS(t)
		mustWriteFile(t, fsys, "notes.txt", "hello world")

		err := fsys.Remove("notes.txt")
		if err != nil {
			t.Fatalf("remove: expected no error, got %v", err)
		}
		if _, err := fsys.Open("notes.txt"); !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("after remove: expected error to be ErrNotExist, got: %v", err)
		}
	})
}

func mustMkdirAll(t *testing.T, fsys remoteFS, p string) {
	t.Helper()
	if err := fsys.MkdirAll(p); err != nil {
		t.Fatalf("mkdir %s: %v", p, err)
	}
}

func mustWriteFile(t *testing.T, fsys remoteFS, p string, c string) {
	t.Helper()

	w, err := fsys.Create(p)
	if err != nil {
		t.Fatalf("create %s: %v", p, err)
	}
	if _, err := w.Write([]byte(c)); err != nil {
		w.Close()
		t.Fatalf("write %s: %v", p, err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close %s: %v", p, err)
	}
}

func mustReadFile(t *testing.T, fsys remoteFS, p string) string {
	t.Helper()

	r, err := fsys.Open(p)
	if err != nil {
		t.Fatalf("open %s: %v", p, err)
	}
	defer r.Close()

	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read %s: %v", p, err)
	}
	return string(data)
}
