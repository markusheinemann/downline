package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/markusheinemann/downline/internal/shipper"
)

func main() {
	os.Exit(run())
}

func run() int {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	cfg, err := loadConfig(logger, os.Args[1:])
	if err != nil {
		logger.Error("failed loading configuration", "err", err)
		return 2
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	remote, err := shipper.DialSFTP(cfg.SFTP.Addr, cfg.SFTP.User, cfg.SFTP.KeyFile, cfg.SFTP.KnownHostsFile)
	if err != nil {
		logger.Error("failed to dial sftp",
			"addr", cfg.SFTP.Addr,
			"err", err)
		return 1
	}
	defer func() {
		err := remote.Close()
		if err != nil {
			logger.Error("failed to close remote sftp", "err", err)
		}
	}()

	s := shipper.New(cfg.ArchivePath, remote, cfg.SFTP.Dir, time.Now, logger)
	report, err := s.Run(ctx)

	logger.Info("finished run",
		"shipped", len(report.Shipped),
		"failed", report.Failed,
		"mismatches", report.Mismatches,
		"ignored", report.Ignored,
	)

	for _, name := range report.Mismatches {
		logger.Error("remote archive differs from the local archive", "file", name)
	}

	if err != nil {
		logger.Error("run failed", "err", err)
		return 1
	}
	if len(report.Failed) > 0 || len(report.Mismatches) > 0 {
		return 1
	}

	return 0
}
