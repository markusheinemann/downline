package main

import (
	"log/slog"
	"time"

	"github.com/markusheinemann/downline/packages/config"
)

type Config struct {
	OpenSky      config.OpenSky
	PollInterval time.Duration
	ArchivePath  string
}

func loadConfig(logger *slog.Logger, args []string) (Config, error) {
	if err := config.LoadDotEnv(logger, ".env"); err != nil {
		return Config{}, err
	}

	var cfg Config
	s := config.NewSet("collector")
	cfg.OpenSky.Register(s)

	s.RequiredDuration(&cfg.PollInterval, "poll-interval", "Interval between fetches to the api (e.g. 30s or 1m)")
	s.RequiredString(&cfg.ArchivePath, "archive-path",
		"Path to location where the zstd archive files will be stored.")

	return cfg, s.Parse(args)
}
