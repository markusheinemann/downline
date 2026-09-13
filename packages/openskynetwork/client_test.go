package openskynetwork_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/markusheinemann/downline/packages/openskynetwork"
)

func TestNewClient(t *testing.T) {
	c := openskynetwork.NewClient(http.DefaultClient)

	if c == nil {
		t.Fatal("expected non-nil http client")
	}

	if c.UserAgent != "downline/0.1" {
		t.Fatalf("expected user agent to be 'downline/0.1', got '%s'", c.UserAgent)
	}

	if c.BaseURL.String() != "https://opensky-network.org/api" {
		t.Fatalf("expected base url to be https://opensky-network.org/api, got '%s'", c.BaseURL.String())
	}
}

func TestNewClientWithNilHandler(t *testing.T) {
	c := openskynetwork.NewClient(nil)
	if c == nil {
		t.Fatal("expected non-nil http client")
	}
}

// newTestClient starts a local HTTP server running handler and returns a
// client pointed at it. The server is shut down when the test finishes.
func newTestClient(t *testing.T, handler http.HandlerFunc) *openskynetwork.Client {
	t.Helper()

	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	c := openskynetwork.NewClient(srv.Client())
	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}

	c.BaseURL = u

	return c
}
