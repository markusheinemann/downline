package archive

import (
	"bytes"
	"io"
	"log"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/klauspost/compress/zstd"
)

func readZstd(t *testing.T, path string) []byte {
	t.Helper()

	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	dec, err := zstd.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer dec.Close()

	data, err := io.ReadAll(dec)
	if err != nil {
		t.Fatal(err)
	}

	return data
}

func TestWrite(t *testing.T) {
	t.Run("writes to specified output location", func(t *testing.T) {
		dir := t.TempDir()
		now := time.Date(2026, 9, 24, 13, 0, 0, 0, time.UTC)
		za, err := newZstdArchiver(dir, log.New(io.Discard, "", log.LstdFlags), func() time.Time { return now })

		if err != nil {
			t.Fatal(err)
		}
		defer za.Close()

		expectedContent := []byte("hello world")
		length, err := za.Write(expectedContent)
		if err != nil {
			t.Fatal(err)
		}

		if length != len(expectedContent) {
			t.Fatalf("expected %d bytes, got %d", len(expectedContent), length)
		}

		if err := za.Close(); err != nil {
			t.Fatal(err)
		}

		got := readZstd(t, filepath.Join(dir, "archive_2026-09-24_13.zst"))
		if !bytes.Equal(got, expectedContent) {
			t.Fatalf("expected %s, got %s", string(expectedContent), string(got))
		}
	})

	t.Run("appends to an existing archive within the same hour", func(t *testing.T) {
		dir := t.TempDir()
		now := time.Date(2026, 9, 24, 13, 0, 0, 0, time.UTC)
		clock := func() time.Time { return now }

		for _, chunk := range []string{"first", "second", "third", "fourth"} {
			za, err := newZstdArchiver(dir, log.New(io.Discard, "", log.LstdFlags), clock)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := za.Write([]byte(chunk)); err != nil {
				t.Fatal(err)
			}
			if err := za.Close(); err != nil {
				t.Fatal(err)
			}
		}

		got := readZstd(t, filepath.Join(dir, "archive_2026-09-24_13.zst"))
		if !bytes.Equal(got, []byte("firstsecondthirdfourth")) {
			t.Fatalf("expected %s, got %s", "firstsecondthirdfourth", string(got))
		}
	})

	t.Run("correctly rotates the archive file", func(t *testing.T) {
		dir := t.TempDir()
		current := time.Date(2026, 9, 24, 13, 59, 0, 0, time.UTC)
		clock := func() time.Time { return current }

		za, err := newZstdArchiver(dir, log.New(io.Discard, "", log.LstdFlags), clock)
		if err != nil {
			t.Fatal(err)
		}
		defer za.Close()

		if _, err := za.Write([]byte("first")); err != nil {
			t.Fatal(err)
		}

		current = current.Add(2 * time.Minute)

		if _, err := za.Write([]byte("second")); err != nil {
			t.Fatal(err)
		}

		for _, name := range []string{"archive_2026-09-24_13.zst", "archive_2026-09-24_14.zst"} {
			if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
				t.Errorf("expected %s to exist: %v", name, err)
			}
		}
	})

	t.Run("retries rotation after a failed rotation", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "output")
		now := time.Date(2026, 9, 24, 13, 59, 0, 0, time.UTC)
		clock := func() time.Time { return now }
		if err := os.Mkdir(dir, 0777); err != nil {
			t.Fatal(err)
		}

		za, err := newZstdArchiver(dir, log.New(io.Discard, "", log.LstdFlags), clock)
		if err != nil {
			t.Fatal(err)
		}
		defer za.Close()

		if _, err := za.Write([]byte("first")); err != nil {
			t.Fatal(err)
		}

		// we remove the output dir so opening the current archive fails
		if err := os.RemoveAll(dir); err != nil {
			t.Fatal(err)
		}
		now = now.Add(2 * time.Minute)

		if _, err := za.Write([]byte("lost")); err == nil {
			t.Fatalf("expected write to fail while output folder is missing")
		}

		if err := os.Mkdir(dir, 0777); err != nil {
			t.Fatal(err)
		}
		now = now.Add(time.Minute)

		if _, err := za.Write([]byte("second")); err != nil {
			t.Fatalf("expected write to recover, got: %v", err)
		}

		if err := za.Close(); err != nil {
			t.Fatal(err)
		}

		got := readZstd(t, filepath.Join(dir, "archive_2026-09-24_14.zst"))
		if !bytes.Equal(got, []byte("second")) {
			t.Fatalf("expected %s, got %s", "second", string(got))
		}
	})
}
