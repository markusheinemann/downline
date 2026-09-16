package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeDotEnv(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("failed to write %s: %v", path, err)
	}
	return path
}

func unsetEnv(t *testing.T, keys ...string) {
	t.Helper()
	for _, k := range keys {
		t.Setenv(k, "")
		err := os.Unsetenv(k)
		if err != nil {
			t.Fatalf("failed to unset %s: %v", k, err)
		}
	}
}

func TestLoadDotEnv(t *testing.T) {
	cases := map[string]struct {
		env           map[string]string
		content       string
		expectedEnv   map[string]string
		expectedUnset []string
	}{
		"sets a key=value pair": {
			content:     "TEST_DOTENV_A=value",
			expectedEnv: map[string]string{"TEST_DOTENV_A": "value"},
		},
		"skips blank lines and comments": {
			content:       "\n   \n# TEST_DOTENV_A=commented\n  # TEST_DOTENV_B=indented\nTEST_DOTENV_C=value\n",
			expectedEnv:   map[string]string{"TEST_DOTENV_C": "value"},
			expectedUnset: []string{"TEST_DOTENV_A", "TEST_DOTENV_B"},
		},
		"strips the export prefix": {
			content:     "export TEST_DOTENV_A=value",
			expectedEnv: map[string]string{"TEST_DOTENV_A": "value"},
		},
		"trims whitespace around key and value": {
			content:     "  TEST_DOTENV_A  =  value  ",
			expectedEnv: map[string]string{"TEST_DOTENV_A": "value"},
		},
		"strips double quotes from the value": {
			content:     `TEST_DOTENV_A="quoted value"`,
			expectedEnv: map[string]string{"TEST_DOTENV_A": "quoted value"},
		},
		"strips single quotes from the value": {
			content:     `TEST_DOTENV_A='quoted value'`,
			expectedEnv: map[string]string{"TEST_DOTENV_A": "quoted value"},
		},
		"keeps equal signs in the value": {
			content:     "TEST_DOTENV_A=a=b=c",
			expectedEnv: map[string]string{"TEST_DOTENV_A": "a=b=c"},
		},
		"sets an empty value": {
			content:     "TEST_DOTENV_A=",
			expectedEnv: map[string]string{"TEST_DOTENV_A": ""},
		},
		"does not override an existing env var": {
			env:         map[string]string{"TEST_DOTENV_A": "from-env"},
			content:     "TEST_DOTENV_A=from-file",
			expectedEnv: map[string]string{"TEST_DOTENV_A": "from-env"},
		},
		"does not override an existing empty env var": {
			env:         map[string]string{"TEST_DOTENV_A": ""},
			content:     "TEST_DOTENV_A=from-file",
			expectedEnv: map[string]string{"TEST_DOTENV_A": ""},
		},
	}

	for name, test := range cases {
		t.Run(name, func(t *testing.T) {
			unsetEnv(t, "TEST_DOTENV_A", "TEST_DOTENV_B", "TEST_DOTENV_C")
			for k, v := range test.env {
				t.Setenv(k, v)
			}

			path := writeDotEnv(t, test.content)
			if err := LoadDotEnv(path); err != nil {
				t.Fatalf("expected LoadDotEnv to succeed, got %v", err)
			}

			for k, expected := range test.expectedEnv {
				actual, ok := os.LookupEnv(k)
				if !ok {
					t.Errorf("expected %s to be set", k)
				} else if actual != expected {
					t.Errorf("expected %s to be %q, got %q", k, expected, actual)
				}
			}
			for _, k := range test.expectedUnset {
				if v, ok := os.LookupEnv(k); ok {
					t.Errorf("expected %s to be unset, got %q", k, v)
				}
			}
		})
	}

	t.Run("succeeds when the file does not exist", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), ".env")
		if err := LoadDotEnv(path); err != nil {
			t.Errorf("expected LoadDotEnv to succeed, got %v", err)
		}
	})

	t.Run("returns open errors other than not exist", func(t *testing.T) {
		path := filepath.Join(writeDotEnv(t, ""), ".env")
		if err := LoadDotEnv(path); err == nil {
			t.Error("expected LoadDotEnv to fail")
		}
	})

	t.Run("fails with the line number on a line without equal sign", func(t *testing.T) {
		unsetEnv(t, "TEST_DOTENV_A", "TEST_DOTENV_B")

		path := writeDotEnv(t, "# comment\nTEST_DOTENV_A=value\nINVALID\nTEST_DOTENV_B=value\n")
		err := LoadDotEnv(path)

		expected := path + ":3: expected key=value"
		if err == nil || err.Error() != expected {
			t.Errorf("expected LoadDotEnv to fail with %q, got %v", expected, err)
		}
		if _, ok := os.LookupEnv("TEST_DOTENV_B"); ok {
			t.Error("expected TEST_DOTENV_B after the invalid line to be unset")
		}
	})
}
