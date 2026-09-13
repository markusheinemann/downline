// Package openskynetwork is a client for the OpenSky Network REST API.
package openskynetwork

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
)

const defaultEndpoint = "https://opensky-network.org/api"

// Client sends requests to the OpenSky Network API. It is safe for concurrent
// use as long as its fields are not changed while requests are running.
type Client struct {
	// BaseURL is the API root
	BaseURL *url.URL
	// UserAgent is sent with every request.
	UserAgent string

	httpClient *http.Client
}

// NewClient returns a Client that sends requests through httpClient.
// If httpClient is nil, http.DefaultClient is used and requests are anonymous,
// which means lower rate limits and no access to historical data.
//
// To authenticate, pass the client created by golang.org/x/oauth2/clientcredentials.
func NewClient(httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	baseURL, _ := url.Parse(defaultEndpoint)
	return &Client{
		httpClient: httpClient,
		BaseURL:    baseURL,
		UserAgent:  "downline/0.1",
	}
}

func (c *Client) newRequest(ctx context.Context, method string, path string, body any) (*http.Request, error) {
	u := c.BaseURL.JoinPath(path)

	var buf io.ReadWriter
	if body != nil {
		buf = new(bytes.Buffer)
		if err := json.NewEncoder(buf).Encode(body); err != nil {
			return nil, err
		}
	}

	req, err := http.NewRequestWithContext(ctx, method, u.String(), buf)
	if err != nil {
		return nil, err
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.UserAgent)

	return req, nil
}

func (c *Client) do(req *http.Request, v any) (*http.Response, error) {
	res, err := c.httpClient.Do(req)
	if err != nil {
		return res, err
	}
	defer func() {
		io.Copy(io.Discard, io.LimitReader(res.Body, 64<<10))
		res.Body.Close()
	}()

	if res.StatusCode < 200 || res.StatusCode > 299 {
		b, _ := io.ReadAll(io.LimitReader(res.Body, 1024))
		return res, &APIError{
			StatusCode: res.StatusCode,
			Status:     res.Status,
			Body:       string(b),
		}
	}

	if v == nil {
		return res, nil
	}

	if err := json.NewDecoder(res.Body).Decode(v); err != nil {
		return res, err
	}

	return res, nil
}
