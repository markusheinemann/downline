package main

import (
	"time"

	"github.com/markusheinemann/downline/packages/config"
)

type Config struct {
	OpenSky      config.OpenSky
	PollInterval time.Duration
}

func loadConfig(args []string) (Config, error) {
	if err := config.LoadDotEnv(".env"); err != nil {
		return Config{}, err
	}

	var cfg Config
	s := config.NewSet("collector")
	cfg.OpenSky.Register(s)

	return cfg, s.Parse(args)
}
