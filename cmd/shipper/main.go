package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/markusheinemann/downline/internal/monitoring"
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

	code, summary := ship(ctx, cfg, logger)
	monitoring.Ping(cfg.PingURL, code, summary, 10*time.Second, logger)

	return code
}

func ship(ctx context.Context, cfg Config, logger *slog.Logger) (int, string) {
	remote, err := shipper.DialSFTP(cfg.SFTP.Addr, cfg.SFTP.User, cfg.SFTP.KeyFile, cfg.SFTP.KnownHostsFile)
	if err != nil {
		logger.Error("failed to dial sftp",
			"addr", cfg.SFTP.Addr,
			"err", err)
		return 1, "connect to sftp server: " + err.Error()
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

	summary := report.Summary()
	failed := err != nil || len(report.Failed) > 0 || len(report.Mismatches) > 0

	if err != nil {
		logger.Error("run failed", "err", err)
		summary += "\nerror: " + err.Error()
	}
	if cfg.StaleAfter > 0 {
		if stalled, reason := collectorStalled(report.LastWrite, time.Now(), cfg.StaleAfter); stalled {
			logger.Error("collector stalled", "last_write", report.LastWrite, "reason", reason)
			summary += "\n" + reason
			failed = true
		}
	}

	if failed {
		return 1, summary
	}

	return 0, summary
}
