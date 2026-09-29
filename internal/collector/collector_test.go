package collector

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"slices"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/markusheinemann/downline/packages/openskynetwork"
)

type fakeFetcher struct {
	mu    sync.Mutex
	calls []time.Time
	fn    func(ctx context.Context) (*openskynetwork.StateVectorsRawResponse, error)
}

func (f *fakeFetcher) ListAllStateVectors(ctx context.Context, _ openskynetwork.StateVectorOptions) (*openskynetwork.StateVectorsRawResponse, error) {
	f.mu.Lock()
	f.calls = append(f.calls, time.Now())
	f.mu.Unlock()
	return f.fn(ctx)
}

func (f *fakeFetcher) offsets(start time.Time) []time.Duration {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]time.Duration, len(f.calls))
	for i, at := range f.calls {
		out[i] = at.Sub(start)
	}
	return out
}

func runFor(t *testing.T, c *Collector, d time.Duration) error {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error)
	go func() { done <- c.Run(ctx) }()

	time.Sleep(d)
	synctest.Wait()
	cancel()
	return <-done
}

func ok(raw string) func(ctx context.Context) (*openskynetwork.StateVectorsRawResponse, error) {
	return func(ctx context.Context) (*openskynetwork.StateVectorsRawResponse, error) {
		return &openskynetwork.StateVectorsRawResponse{Raw: []byte(raw)}, nil
	}
}

func fail(err error) func(ctx context.Context) (*openskynetwork.StateVectorsRawResponse, error) {
	return func(ctx context.Context) (*openskynetwork.StateVectorsRawResponse, error) {
		return nil, err
	}
}

var rateLimited = &openskynetwork.APIError{StatusCode: http.StatusTooManyRequests, Status: "429 Too Many Requests"}
var logger = slog.New(slog.NewTextHandler(io.Discard, nil))

func assertOffsets(t *testing.T, got, want []time.Duration) {
	t.Helper()
	if !slices.Equal(got, want) {
		t.Fatalf("expected fetches at %v, got %v", want, got)
	}
}

func TestRun_FetchesOnIntervalBoundariesAndStoresLines(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		start := time.Now()
		f := &fakeFetcher{fn: ok("{\n  \"time\": 1\n}")}

		var out bytes.Buffer
		c := New(logger, f, time.Minute, &out)
		err := runFor(t, c, 3*time.Minute+30*time.Second)

		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context.Canceled, got %v", err)
		}
		assertOffsets(t, f.offsets(start), []time.Duration{time.Minute, 2 * time.Minute, 3 * time.Minute})
		if want := "{\"time\":1}\n{\"time\":1}\n{\"time\":1}\n"; out.String() != want {
			t.Errorf("got %q, want %q", out.String(), want)
		}
	})
}

func TestRun_BacksOffExponentiallyWhenRateLimited(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		start := time.Now()
		f := &fakeFetcher{fn: fail(rateLimited)}
		var out bytes.Buffer
		c := New(logger, f, time.Minute, &out)

		err := runFor(t, c, 10*time.Minute)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context.Canceled, got %v", err)
		}

		// first tick at 1m, then waits of 1m, 2m, 4m
		assertOffsets(t, f.offsets(start), []time.Duration{1 * time.Minute, 2 * time.Minute, 4 * time.Minute, 8 * time.Minute})
		if out.Len() != 0 {
			t.Errorf("expected nothing written, got %q", out.String())
		}
	})
}

func TestRun_KeepsRunningAfterFailedCycles(t *testing.T) {
	tests := map[string]struct {
		fn func(context.Context) (*openskynetwork.StateVectorsRawResponse, error)
	}{
		"fetch error":  {fn: fail(errors.New("boom"))},
		"invalid json": {fn: ok(`{"time": 1`)},
		"fetch timeout": {fn: func(ctx context.Context) (*openskynetwork.StateVectorsRawResponse, error) {
			<-ctx.Done()
			return nil, ctx.Err()
		}},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				start := time.Now()
				f := &fakeFetcher{fn: tt.fn}
				var out bytes.Buffer
				c := New(logger, f, time.Minute, &out)

				err := runFor(t, c, 3*time.Minute+30*time.Second)

				if !errors.Is(err, context.Canceled) {
					t.Fatalf("expected context.Canceled, got %v", err)
				}

				assertOffsets(t, f.offsets(start), []time.Duration{time.Minute, 2 * time.Minute, 3 * time.Minute})
				if out.Len() != 0 {
					t.Errorf("expected nothing written, got %q", out.String())
				}
			})
		})
	}
}
