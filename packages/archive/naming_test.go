package archive

import (
	"testing"
	"time"
)

func TestFileName(t *testing.T) {
	cest, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Fatalf("failed to load timezone: %v", err)
	}

	cases := map[string]struct {
		input            time.Time
		expectedFilename string
	}{
		"converts utc time correctly": {
			input:            time.Date(2026, time.March, 8, 13, 55, 0, 0, time.UTC),
			expectedFilename: "archive_2026-03-08_13.zst",
		},
		"converts time from non-utc time correctly": {
			input:            time.Date(2026, time.March, 8, 13, 55, 0, 0, cest),
			expectedFilename: "archive_2026-03-08_12.zst", // on our before because of the offset
		},
	}

	t.Parallel()

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			output := FileName(tc.input)
			if output != tc.expectedFilename {
				t.Errorf("got %q, want %q", output, tc.expectedFilename)
			}
		})
	}
}

func TestParseFileName(t *testing.T) {
	cases := map[string]struct {
		filename    string
		expected    time.Time
		expectedErr string
	}{
		"parses archive name correctly": {
			filename: "archive_2026-03-08_13.zst",
			expected: time.Date(2026, time.March, 8, 13, 0, 0, 0, time.UTC),
		},
		"returns error if file name is malformed": {
			filename:    "archive_202666-03-08_13.zst",
			expectedErr: "archive \"archive_202666-03-08_13.zst\" time cannot be parsed: parsing time \"202666-03-08_13\" as \"2006-01-02_15\": cannot parse \"66-03-08_13\" as \"-\"",
		},
		"returns error if the prefix is missing": {
			filename:    "2026-03-08_13.zst",
			expectedErr: "archive \"2026-03-08_13.zst\" has no archive_ prefix",
		},
		"returns error if the suffix is missing": {
			filename:    "archive_2026-03-08_13",
			expectedErr: "archive \"archive_2026-03-08_13\" has no .zst suffix",
		},
	}

	t.Parallel()

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			output, err := ParseFileName(tc.filename)

			if tc.expectedErr != "" {
				if err == nil || err.Error() != tc.expectedErr {
					t.Fatalf("got %v, want %q", err, tc.expectedErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !tc.expected.Equal(output) {
				t.Errorf("got %v, want %v", output, tc.expected)
			}

		})
	}
}

func TestFileNameRoundTrip(t *testing.T) {
	date := time.Date(2026, time.March, 8, 13, 0, 0, 0, time.UTC)

	name := FileName(date)
	if name != "archive_2026-03-08_13.zst" {
		t.Errorf("got %v, want %v", name, "archive_2026-03-08_13.zst")
	}

	parsed, err := ParseFileName(name)
	if err != nil {
		t.Fatalf("failed to parse %q: %v", name, err)
	}
	if !parsed.Equal(date) {
		t.Errorf("got %v, want %v", parsed, date)
	}
}
