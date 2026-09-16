package config

import (
	"errors"
	"flag"
	"io"
	"testing"
)

func TestNewSet(t *testing.T) {
	t.Parallel()

	t.Run("dose not return nil", func(t *testing.T) {
		s := NewSet("test")
		if s == nil {
			t.Error("expected non-nil set")
		}
	})

	t.Run("returns a set with the correct name", func(t *testing.T) {
		s := NewSet("test")
		if s.fs.Name() != "test" {
			t.Errorf("expected set name to be 'test', got '%s'", s.fs.Name())
		}
	})

	t.Run("returns a set with ContinueOnError handling", func(t *testing.T) {
		s := NewSet("test")
		if s.fs.ErrorHandling() != flag.ContinueOnError {
			t.Errorf("expected set to have ContinueOnError set to true, got '%v'", s.fs.ErrorHandling())
		}
	})
}

func TestEnvKey(t *testing.T) {
	cases := map[string]struct {
		input          string
		expectedOutput string
	}{
		"converts to uppercase": {
			input:          "teSt_input",
			expectedOutput: "TEST_INPUT",
		},
		"replaces dashes with underscore": {
			input:          "teSt-input-123",
			expectedOutput: "TEST_INPUT_123",
		},
	}

	t.Parallel()
	for name, test := range cases {
		t.Run(name, func(t *testing.T) {
			actualOutput := EnvKey(test.input)
			if actualOutput != test.expectedOutput {
				t.Errorf("expected EnvKey(%q) to return %q, got %q", test.input, test.expectedOutput, actualOutput)
			}
		})
	}
}

func TestString(t *testing.T) {
	cases := map[string]struct {
		env            map[string]string
		args           []string
		expectedOutput string
	}{
		"uses the default when the env var is unset": {
			args:           []string{},
			expectedOutput: "default",
		},
		"uses the env var as default when set": {
			env:            map[string]string{"LOG_LEVEL": "from-env"},
			args:           []string{},
			expectedOutput: "from-env",
		},
		"uses an empty env var as default when set": {
			env:            map[string]string{"LOG_LEVEL": ""},
			args:           []string{},
			expectedOutput: "",
		},
		"lets the flag override the default": {
			args:           []string{"--log-level", "from-flag"},
			expectedOutput: "from-flag",
		},
		"lets the flag override the env var": {
			env:            map[string]string{"LOG_LEVEL": "from-env"},
			args:           []string{"--log-level", "from-flag"},
			expectedOutput: "from-flag",
		},
	}

	for name, test := range cases {
		t.Run(name, func(t *testing.T) {
			for k, v := range test.env {
				t.Setenv(k, v)
			}

			var got string
			s := NewSet("test")
			s.String(&got, "log-level", "default", "the log level")

			if err := s.Parse(test.args); err != nil {
				t.Fatalf("expected Parse(%q) to succeed, got %v", test.args, err)
			}
			if got != test.expectedOutput {
				t.Errorf("expected log-level to be %q, got %q", test.expectedOutput, got)
			}
		})
	}
}

func TestRequiredString(t *testing.T) {
	cases := map[string]struct {
		env            map[string]string
		args           []string
		expectedOutput string
		expectedErr    string
	}{
		"is satisfied by the flag": {
			args:           []string{"--test-required", "from-flag"},
			expectedOutput: "from-flag",
		},
		"is satisfied by the env var": {
			env:            map[string]string{"TEST_REQUIRED": "from-env"},
			args:           []string{},
			expectedOutput: "from-env",
		},
		"lets the flag override the env var": {
			env:            map[string]string{"TEST_REQUIRED": "from-env"},
			args:           []string{"--test-required", "from-flag"},
			expectedOutput: "from-flag",
		},
		"fails when neither flag nor env var is set": {
			args:        []string{},
			expectedErr: "\t--test-required or TEST_REQUIRED is required",
		},
		"fails when the env var is empty": {
			env:         map[string]string{"TEST_REQUIRED": ""},
			args:        []string{},
			expectedErr: "\t--test-required or TEST_REQUIRED is required",
		},
		"fails when the flag is empty": {
			env:         map[string]string{"TEST_REQUIRED": "from-env"},
			args:        []string{"--test-required", ""},
			expectedErr: "\t--test-required or TEST_REQUIRED is required",
		},
	}

	for name, test := range cases {
		t.Run(name, func(t *testing.T) {
			for k, v := range test.env {
				t.Setenv(k, v)
			}

			var got string
			s := NewSet("test")
			s.RequiredString(&got, "test-required", "a required value")

			err := s.Parse(test.args)
			if test.expectedErr != "" {
				if err == nil || err.Error() != test.expectedErr {
					t.Fatalf("expected Parse(%q) to fail with %q, got %v", test.args, test.expectedErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("expected Parse(%q) to succeed, got %v", test.args, err)
			}
			if got != test.expectedOutput {
				t.Errorf("expected test-required to be %q, got %q", test.expectedOutput, got)
			}
		})
	}
}

func TestParse(t *testing.T) {
	t.Parallel()

	t.Run("succeeds when no required options are registered", func(t *testing.T) {
		var got string
		s := NewSet("test")
		s.String(&got, "test-optional", "", "an optional value")

		if err := s.Parse([]string{}); err != nil {
			t.Errorf("expected Parse to succeed, got %v", err)
		}
	})

	t.Run("reports every missing required option", func(t *testing.T) {
		var a, b, c string
		s := NewSet("test")
		s.RequiredString(&a, "test-required-a", "value a")
		s.RequiredString(&b, "test-required-b", "value b")
		s.RequiredString(&c, "test-required-c", "value c")

		err := s.Parse([]string{"--test-required-b", "set"})

		expected := "\t--test-required-a or TEST_REQUIRED_A is required\n" +
			"\t--test-required-c or TEST_REQUIRED_C is required"
		if err == nil || err.Error() != expected {
			t.Errorf("expected Parse to fail with %q, got %v", expected, err)
		}
	})

	t.Run("includes collected errors before missing required options", func(t *testing.T) {
		var got string
		s := NewSet("test")
		s.RequiredString(&got, "test-required", "a required value")
		s.errs = []error{errors.New("collected")}

		err := s.Parse([]string{})

		expected := "collected\n\t--test-required or TEST_REQUIRED is required"
		if err == nil || err.Error() != expected {
			t.Errorf("expected Parse to fail with %q, got %v", expected, err)
		}
	})

	t.Run("returns the flag parse error without checking required options", func(t *testing.T) {
		var got string
		s := NewSet("test")
		s.fs.SetOutput(io.Discard)
		s.RequiredString(&got, "test-required", "a required value")

		err := s.Parse([]string{"-h"})
		if !errors.Is(err, flag.ErrHelp) {
			t.Errorf("expected Parse to return flag.ErrHelp only, got %v", err)
		}
	})
}
