package main

import (
	"context"
	"errors"
	"log"
	"log/slog"
	"os"
	"os/signal"

	"github.com/markusheinemann/downline/internal/collector"
	"github.com/markusheinemann/downline/packages/archive"
	"github.com/markusheinemann/downline/packages/config"
	"github.com/markusheinemann/downline/packages/openskynetwork"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	logger := slog.Default()
	cfg, err := loadConfig(logger, os.Args[1:])
	if err != nil {
		log.Fatalf("failed to load config:\n%v", err)
	}

	client := createOpenSkyClient(ctx, cfg.OpenSky)
	archiver := createArchiver(cfg.ArchivePath, logger)
	defer archiver.Close()

	col := collector.New(logger, client, cfg.PollInterval, archiver)
	err = col.Run(ctx)
	if err != nil && !errors.Is(err, context.Canceled) {
		log.Printf("failed to run collector:%v", err)
	}
}

func createOpenSkyClient(ctx context.Context, cfg config.OpenSky) *openskynetwork.Client {
	conf := clientcredentials.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		TokenURL:     cfg.TokenURL,
	}

	ts := conf.TokenSource(ctx)
	tc := oauth2.NewClient(ctx, ts)
	return openskynetwork.NewClient(tc)
}

func createArchiver(outputPath string, logger *slog.Logger) *archive.ZstdArchiver {
	archiver, err := archive.NewZstdArchiver(outputPath, logger)
	if err != nil {
		logger.With("err", err).Error("failed to create archiver")
	}
	return archiver
}
