package config

type OpenSky struct {
	ClientID     string
	ClientSecret string
	TokenURL     string
}

func (o *OpenSky) Register(s *Set) {
	s.RequiredString(&o.ClientID, "opensky-client-id", "OpenSky OAuth client ID")
	s.RequiredString(&o.ClientSecret, "opensky-client-secret", "OpenSky OAuth client secret")
	s.String(&o.TokenURL, "opensky-token-url",
		"https://auth.opensky-network.org/auth/realms/opensky-network/protocol/openid-connect/token",
		"OpenSky OAuth token URL")
}
