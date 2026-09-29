# Configuration

Here the configuration properties for each component are listed.

## Collector

| Env                   | Flag                      | Required | Default                                                                                    | Description                                                                              |
|-----------------------|---------------------------|----------|--------------------------------------------------------------------------------------------|------------------------------------------------------------------------------------------|
| OPENSKY_CLIENT_ID     | `--opensky-client-id`     | yes      | -                                                                                          | OAuth client id for the opensky network.                                                 |
| OPENSKY_CLIENT_SECRET | `--opensky-client-secret` | yes      | -                                                                                          | OAuth client secret for the opensky network.                                             |
| OPENSKY_TOKEN_URL     | `--opensky-token-url`     | no       | https://auth.opensky-network.org/auth/realms/opensky-network/protocol/openid-connect/token | OAuth token endpoint from where the client credentials grant can obtain an access token. |
| POLL_INTERVAL         | `--poll-interval`         | yes      | -                                                                                          | Duration between the API calls to fetch ADS-B data.                                      |
| ARCHIVE_PATH          | `--archive-path`          | yes      | -                                                                                          | File location where the .zst archives with raw api response are stored.                  |