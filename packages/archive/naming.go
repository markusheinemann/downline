package archive

import (
	"fmt"
	"strings"
	"time"
)

const fileLayout = "2006-01-02_15"

// FileName returns the archive file name for the given timestamp
func FileName(t time.Time) string {
	return fmt.Sprintf("archive_%s.zst", t.UTC().Format(fileLayout))
}

// ParseFileName returns the timestamp from an archive filename
func ParseFileName(name string) (time.Time, error) {
	rest, ok := strings.CutPrefix(name, "archive_")
	if !ok {
		return time.Time{}, fmt.Errorf("archive %q has no archive_ prefix", name)
	}
	stamp, ok := strings.CutSuffix(rest, ".zst")
	if !ok {
		return time.Time{}, fmt.Errorf("archive %q has no .zst suffix", name)
	}

	t, err := time.Parse(fileLayout, stamp)
	if err != nil {
		return time.Time{}, fmt.Errorf("archive %q time cannot be parsed: %w", name, err)
	}
	return t, nil
}
