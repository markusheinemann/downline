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

## Shipper

The shipper uploads closed hourly archives from `ARCHIVE_PATH` to an SFTP server. Use the same `ARCHIVE_PATH` as the
collector.

| Env                   | Flag                      | Required | Default | Description                                                                                                                               |
|-----------------------|---------------------------|----------|---------|-------------------------------------------------------------------------------------------------------------------------------------------|
| ARCHIVE_PATH          | `--archive-path`          | yes      | -       | Directory with the hourly archives written by the collector. Archives are deleted here after a verified upload.                           |
| SFTP_ADDR             | `--sftp-addr`             | yes      | -       | SFTP server address as `host:port`, e.g. `127.0.0.1:22`. Hetzner Storage Boxes use port 23.                                               |
| SFTP_USER             | `--sftp-user`             | yes      | -       | SFTP user name, e.g. `uploader`.                                                                                                          |
| SFTP_KEY_FILE         | `--sftp-key-file`         | yes      | -       | Path to the SSH private key used to log in. The key must not have a passphrase.                                                           |
| SFTP_KNOWN_HOSTS_FILE | `--sftp-known-hosts-file` | yes      | -       | Path to a `known_hosts` file with the server's host key, e.g. created with `ssh-keyscan`. The connection fails if the key does not match. |
| SFTP_DIR              | `--sftp-dir`              | no       | `tier1` | Directory on the SFTP server to upload archives to, relative to the user's home. Archives are stored in `YYYY/MM/DD` subdirectories.      |
| PING_URL              | `--ping-url`              | no       | -       | Health check URL that receives the exit code of every run, e.g. https://hc-ping.com/<uuid>. Disabled if empty.                            |
| STALE_AFTER           | `--stale-after`           | no       | `0`     | Fail the run if the collector has not written to the current archive for this long, e.g. 5m. If `0` the stale check is turned off.        |