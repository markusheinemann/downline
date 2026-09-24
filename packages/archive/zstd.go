// Package archive provides functionality to write data to a rolling zstd compressed
// files. For each hour a new .zst file is created.
package archive

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/klauspost/compress/zstd"
)

type ZstdArchiver struct {
	mu          sync.Mutex
	currentHour time.Time
	outputPath  string
	outputFile  *os.File
	encoder     *zstd.Encoder
	logger      *log.Logger
	now         func() time.Time
}

// NewZstdArchiver creates a new instance of the archiver. The outputPath specifies where the
// compressed archives will be stored.
func NewZstdArchiver(outputPath string, logger *log.Logger) (*ZstdArchiver, error) {
	return newZstdArchiver(outputPath, logger, time.Now)
}

func newZstdArchiver(outputPath string, logger *log.Logger, now func() time.Time) (*ZstdArchiver, error) {
	za := &ZstdArchiver{
		outputPath: outputPath,
		logger:     logger,
		now:        now,
	}

	if err := za.rotate(); err != nil {
		return nil, fmt.Errorf("unable to create zstd archiver: %w", err)
	}

	return za, nil
}

// Write writes data to the archive files. For each hour a new archive file will be created.
// If an archive file for the hour already exist the content is appended to the existing file.
func (za *ZstdArchiver) Write(content []byte) (int, error) {
	za.mu.Lock()
	defer za.mu.Unlock()

	if !za.currentHour.Equal(za.now().Truncate(time.Hour)) {
		if err := za.rotate(); err != nil {
			return 0, fmt.Errorf("unable to rotate zstd archive: %w", err)
		}
	}

	frame := za.encoder.EncodeAll(content, nil)
	if _, err := za.outputFile.Write(frame); err != nil {
		return 0, err
	}

	return len(content), nil
}

// Close will clean up the archiver.
func (za *ZstdArchiver) Close() error {
	za.mu.Lock()
	defer za.mu.Unlock()

	if za.encoder != nil {
		za.logger.Println("closing zstd encoder")
		za.encoder.Close()
	}

	if za.outputFile != nil {
		za.logger.Println("closing zstd output file")
		return za.outputFile.Close()
	}

	return nil
}

func (za *ZstdArchiver) rotate() error {
	za.logger.Println("rotating zstd archive")

	if za.encoder != nil {
		za.encoder.Close()
		za.encoder = nil
	}
	if za.outputFile != nil {
		za.outputFile.Sync()
		za.outputFile.Close()
		za.outputFile = nil
	}

	now := za.now()
	fileName := fmt.Sprintf("archive_%s.zst", now.Format("2006-01-02_15"))
	fullPath := filepath.Join(za.outputPath, fileName)

	file, err := os.OpenFile(fullPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return fmt.Errorf("failed to open archive: %w", err)
	}

	zw, err := zstd.NewWriter(file, zstd.WithEncoderLevel(zstd.SpeedFastest))
	if err != nil {
		file.Close()
		return fmt.Errorf("failed to create zstd encoder: %w", err)
	}

	za.outputFile = file
	za.encoder = zw
	za.currentHour = startOfHour(now)

	za.logger.Printf("Rotated archive stream to new frame: %s\n", fileName)

	return nil
}

func startOfHour(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), 0, 0, 0, t.Location())
}
