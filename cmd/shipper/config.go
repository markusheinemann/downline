package main

import (
	"log/slog"

	"github.com/markusheinemann/downline/packages/config"
)

type SFTPConfig struct {
	Addr           string
	User           string
	KeyFile        string
	KnownHostsFile string
	Dir            string
}

type Config struct {
	ArchivePath string
	SFTP        SFTPConfig
}

func (c *SFTPConfig) Register(s *config.Set) {
	s.RequiredString(&c.Addr, "sftp-addr",
		"SFTP server address as host:port, e.g. 127.0.0.1:22")
	s.RequiredString(&c.User, "sftp-user",
		"SFTP user name, e.g. uploader")
	s.RequiredString(&c.KeyFile, "sftp-key-file",
		"Path to the SSH private key used to log in. The key must not have a passphrase")
	s.RequiredString(&c.KnownHostsFile, "sftp-known-hosts-file",
		"Path to a known_hosts file with the server's host key. The connection fails if the key does not match")
	s.String(&c.Dir, "sftp-dir", "tier1",
		"Directory on the SFTP server to upload archives to, relative to the user's home")
}

func loadConfig(logger *slog.Logger, args []string) (Config, error) {
	if err := config.LoadDotEnv(logger, ".env"); err != nil {
		return Config{}, err
	}

	var cfg Config
	s := config.NewSet("shipper")

	s.RequiredString(&cfg.ArchivePath, "archive-path",
		"Directory with the hourly archives written by the collector. Archives are deleted here after a verified upload.")
	cfg.SFTP.Register(s)

	return cfg, s.Parse(args)
}
