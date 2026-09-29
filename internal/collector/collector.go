package collector

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/markusheinemann/downline/packages/openskynetwork"
)

type Collector struct {
	log      *slog.Logger
	client   stateFetcher
	interval time.Duration
	archiver io.Writer
}

type stateFetcher interface {
	ListAllStateVectors(ctx context.Context, opts openskynetwork.StateVectorOptions) (*openskynetwork.StateVectorsRawResponse, error)
}

func New(log *slog.Logger, client stateFetcher, interval time.Duration, archiver io.Writer) *Collector {
	return &Collector{
		log:      log,
		client:   client,
		interval: interval,
		archiver: archiver,
	}
}

func (c *Collector) Run(ctx context.Context) error {
	c.log.Info("collector starting", "interval", c.interval)

	timer := time.NewTimer(untilNext(time.Now(), c.interval))
	defer timer.Stop()

	var backoff time.Duration

	for {
		select {
		case <-ctx.Done():
			c.log.Info("collector stopping")
			return ctx.Err()
		case tick := <-timer.C:

			err := c.cycle(ctx, tick)

			switch {
			case errors.Is(err, openskynetwork.ErrRateLimited):
				backoff = max(backoff*2, c.interval)
				c.log.
					With("for", backoff).
					Warn("rate limited, backing off", "for", backoff)
				timer.Reset(backoff)

			case err != nil:
				c.log.
					With("err", err).
					Error("cycle failed")
				backoff = 0
				timer.Reset(untilNext(time.Now(), c.interval))

			default:
				backoff = 0
				timer.Reset(untilNext(time.Now(), c.interval))
			}
		}
	}
}

func untilNext(now time.Time, every time.Duration) time.Duration {
	return now.Truncate(every).Add(every).Sub(now)
}

func (c *Collector) cycle(ctx context.Context, tick time.Time) error {
	// make sure to never let one cycle run into the next
	ctx, cancel := context.WithTimeout(ctx, c.interval/5)
	defer cancel()

	// fetch the data from openskynetwork
	res, err := c.client.ListAllStateVectors(ctx, openskynetwork.StateVectorOptions{})
	if err != nil {
		return fmt.Errorf("failed to fetch state vectors: %w", err)
	}

	// add a line break to the end of the response which makes the decoding
	// of the archives a lot easier
	var buf bytes.Buffer
	if err := json.Compact(&buf, res.Raw); err != nil {
		return fmt.Errorf("invalid json response: %w", err)
	}
	buf.WriteByte('\n')

	// write the fetched data to archives
	length, err := c.archiver.Write(buf.Bytes())
	if err != nil {
		return fmt.Errorf("write to archive failed: %w", err)
	}
	c.log.
		With("length", length).
		With("delay", time.Since(tick)).
		Info("wrote archive to disk")

	return nil
}
