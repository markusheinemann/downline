package config

import (
	"testing"
)

func TestOpenSkyRegister(t *testing.T) {
	t.Run("maps the flags to the fields", func(t *testing.T) {
		unsetEnv(t, "OPENSKY_CLIENT_ID", "OPENSKY_CLIENT_SECRET", "OPENSKY_TOKEN_URL")

		var o OpenSky
		s := NewSet("test")
		o.Register(s)

		err := s.Parse([]string{
			"--opensky-client-id", "id",
			"--opensky-client-secret", "secret",
			"--opensky-token-url", "https://example.com/token",
		})
		if err != nil {
			t.Fatalf("expected Parse to succeed, got %v", err)
		}

		expected := OpenSky{ClientID: "id", ClientSecret: "secret", TokenURL: "https://example.com/token"}
		if o != expected {
			t.Errorf("expected %+v, got %+v", expected, o)
		}
	})

	t.Run("defaults the token URL to the OpenSky auth server", func(t *testing.T) {
		unsetEnv(t, "OPENSKY_TOKEN_URL")

		var o OpenSky
		s := NewSet("test")
		o.Register(s)

		if err := s.Parse([]string{"--opensky-client-id", "id", "--opensky-client-secret", "secret"}); err != nil {
			t.Fatalf("expected Parse to succeed, got %v", err)
		}

		expected := "https://auth.opensky-network.org/auth/realms/opensky-network/protocol/openid-connect/token"
		if o.TokenURL != expected {
			t.Errorf("expected TokenURL to be %q, got %q", expected, o.TokenURL)
		}
	})

	t.Run("requires the client ID and secret", func(t *testing.T) {
		unsetEnv(t, "OPENSKY_CLIENT_ID", "OPENSKY_CLIENT_SECRET", "OPENSKY_TOKEN_URL")

		var o OpenSky
		s := NewSet("test")
		o.Register(s)

		err := s.Parse([]string{})

		expected := "\t--opensky-client-id or OPENSKY_CLIENT_ID is required\n" +
			"\t--opensky-client-secret or OPENSKY_CLIENT_SECRET is required"
		if err == nil || err.Error() != expected {
			t.Errorf("expected Parse to fail with %q, got %v", expected, err)
		}
	})
}
