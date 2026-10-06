package monitoring

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Ping reports the exit code of a run to a health check endpoint (e.g. healthchecks.io).
// It does noting if the url is empty. Potential errors are only logged.
func Ping(pingUrl string, exitCode int, body string, timeout time.Duration, logger *slog.Logger) {
	if pingUrl == "" {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	fullURL := pingUrl + "/" + strconv.Itoa(exitCode)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fullURL, strings.NewReader(body))
	if err != nil {
		logger.Warn("failed to create request for monitoring ping", "err", err)
		return
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		if urlErr, ok := errors.AsType[*url.Error](err); ok {
			err = urlErr.Err
		}
		logger.Warn("failed to send monitoring ping", "err", err)
		return
	}
	defer res.Body.Close()

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		logger.Warn("monitoring ping rejected", "status", res.StatusCode)
	}
}
