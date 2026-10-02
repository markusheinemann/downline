package shipper

import (
	"context"
	"errors"
	"log/slog"
	"maps"
	"os"
	"path"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

func newTestShipper(t *testing.T, now time.Time) (*Shipper, string, *fakeRemoteFS) {
	t.Helper()

	localDir := t.TempDir()
	fsFake := newFakeRemoteFS()
	s := &Shipper{
		localDir:   localDir,
		remote:     fsFake,
		remoteRoot: "tier1",
		now: func() time.Time {
			return now
		},
		log: slog.New(slog.DiscardHandler),
	}

	return s, localDir, fsFake
}

func writeLocal(t *testing.T, dir string, name string, content []byte) {
	t.Helper()

	p := filepath.Join(dir, name)
	err := os.WriteFile(p, content, 0644)
	if err != nil {
		t.Fatal("failed to write local file:", err)
	}
}

func localNames(t *testing.T, dir string) []string {
	t.Helper()

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("failed to read local directory: %s", err)
	}
	var names []string
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names
}

func remotePaths(f *fakeRemoteFS) []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Sorted(maps.Keys(f.files))
}

func TestRun_ShipsOnlyClosedHours(t *testing.T) {
	cases := map[string]struct {
		local       []string
		now         time.Time
		wantRemote  []string
		wantLocal   []string
		wantIgnored []string
	}{
		"empty folder": {
			local:       []string{},
			now:         time.Now(),
			wantRemote:  []string{},
			wantLocal:   []string{},
			wantIgnored: []string{},
		},
		"current hour only": {
			local:       []string{"archive_2026-03-08_13.zst", "archive_2026-03-08_14.zst"},
			now:         time.Date(2026, time.March, 8, 14, 22, 0, 0, time.UTC),
			wantRemote:  []string{"archive_2026-03-08_13.zst"},
			wantLocal:   []string{"archive_2026-03-08_14.zst"},
			wantIgnored: []string{},
		},
		"one closed hour": {
			local:       []string{"archive_2026-03-08_13.zst"},
			now:         time.Date(2026, time.March, 8, 14, 00, 0, 0, time.UTC),
			wantRemote:  []string{"archive_2026-03-08_13.zst"},
			wantLocal:   []string{},
			wantIgnored: []string{},
		},
		"several hours": {
			local:       []string{"archive_2026-03-08_11.zst", "archive_2026-03-08_12.zst", "archive_2026-03-08_13.zst"},
			now:         time.Date(2026, time.March, 8, 14, 00, 0, 0, time.UTC),
			wantRemote:  []string{"archive_2026-03-08_11.zst", "archive_2026-03-08_12.zst", "archive_2026-03-08_13.zst"},
			wantLocal:   []string{},
			wantIgnored: []string{},
		},
		"invalid names": {
			local:       []string{"archive_2026-03-08_12", "archive_2026-03-08_13.zst", "invalid-name"},
			now:         time.Date(2026, time.March, 8, 14, 00, 0, 0, time.UTC),
			wantRemote:  []string{"archive_2026-03-08_13.zst"},
			wantLocal:   []string{"archive_2026-03-08_12", "invalid-name"},
			wantIgnored: []string{"archive_2026-03-08_12", "invalid-name"},
		},
		"future file": {
			local:       []string{"archive_2026-03-08_13.zst"},
			now:         time.Date(2026, time.March, 8, 12, 00, 0, 0, time.UTC),
			wantRemote:  []string{},
			wantLocal:   []string{"archive_2026-03-08_13.zst"},
			wantIgnored: []string{},
		},
	}

	t.Parallel()

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			s, localDir, _ := newTestShipper(t, tc.now)

			for _, name := range tc.local {
				writeLocal(t, localDir, name, []byte(name))
			}

			report, err := s.Run(t.Context())
			if err != nil {
				t.Fatal("failed to run shipper:", err)
			}

			if !slices.Equal(report.Shipped, tc.wantRemote) {
				t.Errorf("expected shipped files %v, got: %v", tc.wantRemote, report.Shipped)
			}
			if !slices.Equal(report.Ignored, tc.wantIgnored) {
				t.Errorf("expected ignored files %v, got: %v", tc.wantIgnored, report.Ignored)
			}

			localFiles, err := os.ReadDir(localDir)
			if err != nil {
				t.Fatal("failed to read local dir:", err)
			}
			ok := slices.EqualFunc(localFiles, tc.wantLocal, func(a os.DirEntry, b string) bool {
				return a.Name() == b && !a.IsDir()
			})
			if !ok {
				t.Errorf("expected local files %v, got: %v", tc.wantLocal, localFiles)
			}
		})
	}
}

func TestRun_ContinuesAfterFailedFile(t *testing.T) {
	const (
		first  = "archive_2026-03-06_13.zst"
		middle = "archive_2026-03-07_13.zst"
		last   = "archive_2026-03-08_13.zst"
	)
	middleTmp := "tier1/2026/03/07/" + middle + ".tmp"

	cases := map[string]struct {
		op   string
		path string
	}{
		"mkdir fails":  {op: "mkdir", path: "tier1/2026/03/07"},
		"create fails": {op: "create", path: middleTmp},
		"open fails":   {op: "open", path: "tier1/2026/03/07/" + middle},
		"write fails":  {op: "write", path: middleTmp},
		"close fails":  {op: "close", path: middleTmp},
		"rename fails": {op: "rename", path: middleTmp},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			s, localDir, remote := newTestShipper(t, time.Date(2026, time.March, 8, 14, 00, 0, 0, time.UTC))
			for _, n := range []string{first, middle, last} {
				writeLocal(t, localDir, n, []byte(n))
			}
			remote.failAt(tc.op, tc.path)

			report, err := s.Run(t.Context())
			if err != nil {
				t.Fatalf("expected Run to succeed despite a failed file, got: %v", err)
			}
			if want := []string{first, last}; !slices.Equal(report.Shipped, want) {
				t.Errorf("expected shipped files %v, got: %v", want, report.Shipped)
			}
			if want := []string{middle}; !slices.Equal(report.Failed, want) {
				t.Errorf("expected failed files %v, got: %v", want, report.Failed)
			}
			if got, want := localNames(t, localDir), []string{middle}; !slices.Equal(got, want) {
				t.Errorf("expected local files %v, got: %v", want, got)
			}
			wantRemote := []string{
				"tier1/2026/03/06/" + first,
				"tier1/2026/03/08/" + last,
			}
			if got := remotePaths(remote); !slices.Equal(got, wantRemote) {
				t.Errorf("expected remote files %v, got: %v", wantRemote, got)
			}
		})
	}
}

func TestRun_HandlesArchiveAlreadyOnRemote(t *testing.T) {
	const (
		name       = "archive_2026-03-08_13.zst"
		remotePath = "tier1/2026/03/08/" + name
	)
	localContent := []byte("local archive")

	cases := map[string]struct {
		remoteContent  []byte
		wantShipped    []string
		wantMismatches []string
		wantLocal      []string
		wantRemote     []string
	}{
		"same content": {
			remoteContent: localContent,
			wantShipped:   []string{name},
		},
		"different content": {
			remoteContent:  []byte("sth. else"),
			wantMismatches: []string{name},
			wantLocal:      []string{name},
		},
	}

	t.Parallel()

	for tcName, tc := range cases {
		t.Run(tcName, func(t *testing.T) {
			t.Parallel()

			s, localDir, remote := newTestShipper(t, time.Date(2026, time.March, 8, 14, 30, 0, 0, time.UTC))
			writeLocal(t, localDir, name, localContent)
			seedTestDir(t, remote, path.Dir(remotePath))
			seedTestFile(t, remote, remotePath, tc.remoteContent)

			remote.failAt("create", remotePath+".tmp")

			report, err := s.Run(t.Context())
			if err != nil {
				t.Fatalf("expected Run to succeed, got :%v", err)
			}
			if !slices.Equal(report.Shipped, tc.wantShipped) {
				t.Errorf("expected shipped files %v, got: %v", tc.wantShipped, report.Shipped)
			}
			if !slices.Equal(report.Mismatches, tc.wantMismatches) {
				t.Errorf("expected mismatches files %v, got: %v", tc.wantMismatches, report.Mismatches)
			}
			if len(report.Failed) != 0 {
				t.Errorf("failed: got %v, want none", report.Failed)
			}
			if got := localNames(t, localDir); !slices.Equal(got, tc.wantLocal) {
				t.Errorf("expected local files %v, got: %v", tc.wantLocal, got)
			}
			// remote files are never overwritten
			if got := string(remote.files[remotePath]); got != string(tc.remoteContent) {
				t.Errorf("remote content: got %q, want %q", got, tc.remoteContent)
			}
			if got, want := remotePaths(remote), []string{remotePath}; !slices.Equal(got, want) {
				t.Errorf("remote paths: got %v, want %v", got, want)
			}
		})
	}
}

func TestRun_StopsWhenContextIsCancelled(t *testing.T) {
	s, localDir, remote := newTestShipper(t, time.Date(2026, time.March, 8, 14, 30, 0, 0, time.UTC))
	writeLocal(t, localDir, "archive_2026-03-08_13.zst", []byte("data"))

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	report, err := s.Run(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancelled error, got: %v", err)
	}
	if len(report.Shipped) != 0 {
		t.Errorf("expected no shipped files, got: %v", report.Shipped)
	}
	if got := remotePaths(remote); len(got) != 0 {
		t.Errorf("expected remote files: got %v, want none", got)
	}
	if got, want := localNames(t, localDir), []string{"archive_2026-03-08_13.zst"}; !slices.Equal(got, want) {
		t.Errorf("expected local files %v, got: %v", want, got)
	}
}

func TestRun_IsIdempotent(t *testing.T) {
	wantRemote := []string{
		"tier1/2026/03/08/archive_2026-03-08_13.zst",
		"tier1/2026/03/08/archive_2026-03-08_14.zst",
	}

	now := time.Date(2026, time.March, 8, 14, 10, 0, 0, time.UTC)
	s, localDir, remote := newTestShipper(t, now)
	s.now = func() time.Time { return now }

	writeLocal(t, localDir, "archive_2026-03-08_13.zst", []byte("thirteen"))
	writeLocal(t, localDir, "archive_2026-03-08_14.zst", []byte("fourteen"))

	report, err := s.Run(t.Context())
	if err != nil {
		t.Fatalf("expected Run to succeed, got: %v", err)
	}
	if got, want := report.Shipped, []string{"archive_2026-03-08_13.zst"}; !slices.Equal(got, want) {
		t.Errorf("expected shipped files %v, got: %v", want, got)
	}
	if len(report.Failed) != 0 {
		t.Errorf("expected failed files: got %v, want none", report.Failed)
	}
	if len(report.Mismatches) != 0 {
		t.Errorf("expected mismatched files: got %v, want none", report.Mismatches)
	}

	// move one hour further
	now = now.Add(time.Hour)

	report, err = s.Run(t.Context())
	if err != nil {
		t.Fatalf("expected Run to succeed, got: %v", err)
	}
	if got, want := report.Shipped, []string{"archive_2026-03-08_14.zst"}; !slices.Equal(got, want) {
		t.Errorf("expected shipped files %v, got: %v", want, got)
	}
	if len(report.Failed) != 0 {
		t.Errorf("failed: got %v, want none", report.Failed)
	}
	if len(report.Mismatches) != 0 {
		t.Errorf("mismatches: got %v, want none", report.Mismatches)
	}

	if got := remotePaths(remote); !slices.Equal(got, wantRemote) {
		t.Errorf("expected remote files %v, got: %v", wantRemote, got)
	}
	if got := string(remote.files[wantRemote[0]]); got != "thirteen" {
		t.Errorf("expected remote content: got %q, want %q", got, "thirteen")
	}
	if got := string(remote.files[wantRemote[1]]); got != "fourteen" {
		t.Errorf("expected remote content: got %q, want %q", got, "fourteen")
	}
	if got := localNames(t, localDir); got != nil {
		t.Errorf("expected local directory: got %v, want none", got)
	}
}
