package config

import (
	"bufio"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
)

func LoadDotEnv(logger *slog.Logger, path string) error {
	logger.With("path", path)
	logger.
		With("path", path).
		Info("loading environment variables from dotenv file")

	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		logger.Info(".env file does not exist")
		return nil
	}
	if err != nil {
		return err
	}

	scanner := bufio.NewScanner(f)
	for n := 1; scanner.Scan(); n++ {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return fmt.Errorf("%s:%d: expected key=value", path, n)
		}

		key = strings.TrimSpace(key)
		value = strings.Trim(strings.TrimSpace(value), `"'`)

		if _, exists := os.LookupEnv(key); !exists {
			os.Setenv(key, value)
		}
	}
	return scanner.Err()
}
