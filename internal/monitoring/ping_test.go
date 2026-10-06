package monitoring

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

type recorded struct {
	method string
	path   string
	body   string
}

func newRecordingServer(t *testing.T, handler http.HandlerFunc) (*httptest.Server, func() []recorded) {
	t.Helper()

	var (
		mu   sync.Mutex
		reqs []recorded
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		reqs = append(reqs, recorded{method: r.Method, path: r.URL.Path, body: string(body)})
		mu.Unlock()
		handler(w, r)
	}))
	t.Cleanup(srv.Close)

	return srv, func() []recorded {
		mu.Lock()
		defer mu.Unlock()
		return append([]recorded(nil), reqs...)
	}
}

func bufferLogger() (*slog.Logger, *bytes.Buffer) {
	var buf bytes.Buffer
	return slog.New(slog.NewTextHandler(&buf, nil)), &buf
}

func ok(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func TestPing_DoesNothingWithoutURL(t *testing.T) {
	logger, logs := bufferLogger()

	Ping("", 1, "body", time.Second, logger)

	if logs.Len() != 0 {
		t.Errorf("expected no log output, got %q", logs.String())
	}
}

func TestPing_SendsExitCodeAndBody(t *testing.T) {
	cases := map[string]struct {
		exitCode int
		body     string
		wantPath string
	}{
		"success": {exitCode: 0, body: "shipped=1", wantPath: "/check/0"},
		"failure": {exitCode: 1, body: "failed=[a.zst]", wantPath: "/check/1"},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			srv, requests := newRecordingServer(t, ok)
			logger, logs := bufferLogger()

			Ping(srv.URL+"/check", tc.exitCode, tc.body, time.Second, logger)

			got := requests()
			if len(got) != 1 {
				t.Fatalf("requests: got %d, want 1", len(got))
			}
			want := recorded{method: http.MethodPost, path: tc.wantPath, body: tc.body}
			if got[0] != want {
				t.Fatalf("requests: got %+v, want %v", got[0], want)
			}
			if logs.Len() != 0 {
				t.Errorf("expected no log output, got %q", logs.String())
			}
		})
	}
}

func TestPing_OnlyLogsFailures(t *testing.T) {
	cases := map[string]struct {
		handler     http.HandlerFunc
		closeServer bool
		wantLog     string
	}{
		"server error": {
			handler: func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusInternalServerError) },
			wantLog: "monitoring ping rejected",
		},
		"server unreachable": {
			handler:     ok,
			closeServer: true,
			wantLog:     "failed to send monitoring ping",
		},
		"server too slow": {
			handler: func(w http.ResponseWriter, r *http.Request) {
				select {
				case <-r.Context().Done():
				case <-time.After(time.Second):
				}
			},
			wantLog: "failed to send monitoring ping",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			srv, _ := newRecordingServer(t, tc.handler)
			if tc.closeServer {
				srv.Close()
			}
			logger, logs := bufferLogger()

			start := time.Now()
			Ping(srv.URL+"/check", 0, "", 50*time.Millisecond, logger)

			if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
				t.Errorf("ping took %v, want it to give up after the timeout", elapsed)
			}
			if !strings.Contains(logs.String(), tc.wantLog) {
				t.Errorf("log: got %q, want it to contain %q", logs.String(), tc.wantLog)
			}
			if strings.Contains(logs.String(), srv.URL) {
				t.Errorf("log must not contain the ping URL, got %q", logs.String())
			}
		})
	}
}
