# Deployment

> [!Note]
> **Disclaimer on AI usage**
> This guide was generated with AI and reviewed by a human. If a step does not work as described, please open an
> issue.

This guide explains how to run the collector and the shipper on a Linux server. Both components run as Docker
containers that systemd starts and supervises.

## How the components work together

The **collector** runs continuously. It fetches state vectors from the OpenSky Network API and appends them to an
hourly archive file in a local directory.

The **shipper** runs once per hour. It uploads every closed hourly archive to an SFTP server, verifies the upload, and
then deletes the local file. The archive of the current hour stays local until the hour has ended.

Both containers share the same archive directory on the host, `/var/lib/downline/archive`. The collector writes to it,
and the shipper reads from it and deletes the files it has uploaded.

> [!Note]
> **Why the archives are moved to a separate storage server**
> The collector needs little compute power, but the archives grow by several hundred gigabytes per year. Storage on a
> storage server such as a Hetzner Storage Box costs much less per gigabyte than disk space on a regular server. The
> server that runs the collector therefore only keeps the archive of the current hour, and the shipper moves all
> closed archives to the storage server.

## Prerequisites

You need the following before you start:

* A Linux server with root access. This guide uses Ubuntu 24.04.
* An OpenSky Network account with an OAuth client ID and client secret.
* An SFTP server for the archives. This guide uses a Hetzner Storage Box, but any SFTP server works.
* Network access to the GitHub Container Registry (GHCR), which hosts the public container images.

All commands in this guide run as `root` on the server.

## File layout

After the setup, the server contains these files:

| Path                                             | Purpose                                               |
|--------------------------------------------------|-------------------------------------------------------|
| `/etc/downline/version.env`                      | Image version that both services use                  |
| `/etc/downline/collector.env`                    | Collector configuration, including the OpenSky secret |
| `/etc/downline/shipper.env`                      | Shipper configuration                                 |
| `/etc/downline/ssh/storagebox`                   | Private SSH key for the SFTP server                   |
| `/etc/downline/ssh/known_hosts`                  | Host key of the SFTP server                           |
| `/var/lib/downline/archive/`                     | Hourly archives, written by the collector             |
| `/etc/systemd/system/downline-collector.service` | Service that runs the collector                       |
| `/etc/systemd/system/downline-shipper.service`   | Service that runs the shipper once                    |
| `/etc/systemd/system/downline-shipper.timer`     | Timer that starts the shipper every hour              |
| `/usr/local/bin/downline-update`                 | Script that updates both services to a new version    |

## Step 1: Prepare the SFTP server

The shipper logs in with an SSH key and uploads files into a directory below the home directory of its user.

For a Hetzner Storage Box:

1. In the Hetzner Console, enable **SSH support** for the Storage Box. If the server is outside the Hetzner network,
   also enable **external reachability**.
2. Create a **sub account** for the shipper and set a password. You need the password once in step 3.
3. Note the user name and host name of the sub account, for example `u12345-sub1` and
   `u12345-sub1.your-storagebox.de`.
4. Look up the SSH host key fingerprints of the Storage Box in the Hetzner documentation. You need them in step 4.

Hetzner Storage Boxes accept SSH and SFTP connections on port **23**, not on port 22.

## Step 2: Install Docker and create the directories

```bash
apt update
apt install -y docker.io
systemctl enable --now docker

mkdir -p /etc/downline/ssh /var/lib/downline/archive
chown 65532:65532 /etc/downline/ssh /var/lib/downline/archive
chmod 700 /etc/downline/ssh
chmod 750 /var/lib/downline/archive
```

The containers run as the user with UID `65532`. This user must own the SSH directory and the archive directory, so
that the collector can write archives and the shipper can read and delete them.

To check the result, run `ls -ld /etc/downline/ssh /var/lib/downline/archive`. Both directories must be owned by
`65532`.

## Step 3: Create the SSH key

```bash
ssh-keygen -t ed25519 -N "" -C downline-shipper -f /etc/downline/ssh/storagebox
ssh-copy-id -p 23 -s -i /etc/downline/ssh/storagebox.pub <sftp-user>@<sftp-host>
```

The key has no passphrase, because the shipper cannot unlock encrypted keys. The `-s` option uploads the key over
SFTP, which is required for Storage Boxes because they provide no regular shell. `ssh-copy-id` asks for the sub
account password from step 1.

Give the container user access to the key:

```bash
chown 65532:65532 /etc/downline/ssh/storagebox /etc/downline/ssh/storagebox.pub
chmod 400 /etc/downline/ssh/storagebox
```

## Step 4: Create and verify the known_hosts file

The shipper only connects to a server whose host key is listed in a `known_hosts` file. This prevents the shipper from
uploading data to a server that impersonates your SFTP server.

```bash
ssh-keyscan -p 23 <sftp-host> > /etc/downline/ssh/known_hosts
ssh-keygen -lf /etc/downline/ssh/known_hosts
```

Compare the fingerprints in the output with the fingerprints from step 1. **Continue only if they match.**

```bash
chown 65532:65532 /etc/downline/ssh/known_hosts
chmod 444 /etc/downline/ssh/known_hosts
```

Test the login:

```bash
sftp -P 23 \
  -i /etc/downline/ssh/storagebox \
  -o UserKnownHostsFile=/etc/downline/ssh/known_hosts \
  -o StrictHostKeyChecking=yes \
  <sftp-user>@<sftp-host>
```

The login must succeed without a password prompt. Type `exit` to close the session.

## Step 5: Set the image version

Both services read the image version from one file. This lets you update both services by changing a single value.

```bash
echo 'DOWNLINE_VERSION=<version>' > /etc/downline/version.env
```

Replace `<version>` with an image tag from GHCR, for example `7d15d26`.

## Step 6: Configure the collector

```bash
cat > /etc/downline/collector.env <<'EOF'
OPENSKY_CLIENT_ID=<client-id>
OPENSKY_CLIENT_SECRET=<client-secret>
POLL_INTERVAL=100s
ARCHIVE_PATH=/archive
EOF
chmod 600 /etc/downline/collector.env
```

`ARCHIVE_PATH` is the path inside the container. Step 7 mounts the host directory at this path.

A poll interval of `100s` results in 864 requests per day. Each request costs 4 credits, so the collector uses 3456 of
the 4000 credits that a standard OpenSky account receives per day. For details on all options, see
[Configuration](configuration.md).

## Step 7: Create the collector service

```bash
cat > /etc/systemd/system/downline-collector.service <<'EOF'
[Unit]
Description=Collect OpenSky state vectors into hourly archives
Wants=network-online.target
After=network-online.target docker.service
Requires=docker.service

[Service]
EnvironmentFile=/etc/downline/version.env
ExecStartPre=-/usr/bin/docker rm -f downline-collector
ExecStart=/usr/bin/docker run --rm --name downline-collector \
    --env-file /etc/downline/collector.env \
    -v /var/lib/downline/archive:/archive \
    ghcr.io/markusheinemann/downline/collector:${DOWNLINE_VERSION}
ExecStop=/usr/bin/docker stop downline-collector
Restart=always
RestartSec=30s

[Install]
WantedBy=multi-user.target
EOF
```

The unit uses two different environment files:

* `EnvironmentFile=` is read by **systemd**. It provides `DOWNLINE_VERSION` for the image tag.
* `--env-file` is read by **Docker**. It passes the configuration into the container.

If the collector stops, systemd restarts it after 30 seconds.

Start the collector:

```bash
systemctl daemon-reload
systemctl enable --now downline-collector.service
```

## Step 8: Configure the shipper

```bash
cat > /etc/downline/shipper.env <<'EOF'
ARCHIVE_PATH=/archive
SFTP_ADDR=<sftp-host>:23
SFTP_USER=<sftp-user>
SFTP_KEY_FILE=/ssh/storagebox
SFTP_KNOWN_HOSTS_FILE=/ssh/known_hosts
SFTP_DIR=tier1
EOF
chmod 600 /etc/downline/shipper.env
```

All paths are paths inside the container. The shipper stores the archives on the SFTP server in subdirectories of
`SFTP_DIR`, for example `tier1/2026/10/06/archive_2026-10-06_13.zst`.

> **Warning:** The shipper deletes every archive from `ARCHIVE_PATH` after it has uploaded and verified it. Do not
> point `ARCHIVE_PATH` to a directory that contains files you want to keep.

## Step 9: Create the shipper service and timer

```bash
cat > /etc/systemd/system/downline-shipper.service <<'EOF'
[Unit]
Description=Ship closed downline archives to the SFTP server
Wants=network-online.target
After=network-online.target docker.service
Requires=docker.service

[Service]
Type=oneshot
EnvironmentFile=/etc/downline/version.env
ExecStartPre=-/usr/bin/docker rm -f downline-shipper
ExecStart=/usr/bin/docker run --rm --name downline-shipper \
    --env-file /etc/downline/shipper.env \
    -v /var/lib/downline/archive:/archive \
    -v /etc/downline/ssh:/ssh:ro \
    ghcr.io/markusheinemann/downline/shipper:${DOWNLINE_VERSION}
ExecStopPost=-/usr/bin/docker rm -f downline-shipper
TimeoutStartSec=30min
EOF

cat > /etc/systemd/system/downline-shipper.timer <<'EOF'
[Unit]
Description=Run the downline shipper every hour

[Timer]
OnCalendar=*-*-* *:15:00
Persistent=true

[Install]
WantedBy=timers.target
EOF

systemctl daemon-reload
```

The shipper exits after each run, so the service uses `Type=oneshot`. For this type, `TimeoutStartSec` limits the
duration of a whole run. If a run takes longer than 30 minutes, systemd stops it. The `docker rm -f` commands remove a
container that a stopped or crashed run left behind.

The timer starts the shipper at minute 15 of every hour. `Persistent=true` starts a missed run after the server was
offline, for example after a reboot.

Run the shipper once to test the setup:

```bash
systemctl start downline-shipper.service
journalctl --no-pager -u downline-shipper.service -n 20
```

The log must contain a `finished run` line, and systemd must report `Deactivated successfully`. This shows that the SSH
key, the `known_hosts` file, and the directory permissions are correct.

Enable the timer:

```bash
systemctl enable --now downline-shipper.timer
systemctl list-timers downline-shipper.timer
```

## Step 10: Verify the setup

Check that the collector writes archives. The first request happens at the next poll interval boundary, so wait at
least two minutes.

```bash
journalctl --no-pager -u downline-collector.service -n 30
ls -l /var/lib/downline/archive
```

The archive directory must contain a file for the current hour, for example `archive_2026-10-06_13.zst`. The hour in
the file name is in UTC.

After the next full hour and minute 15 have passed, check that the shipper uploaded the closed archive:

```bash
journalctl --no-pager -u downline-shipper.service --since "1 hour ago"
ls -l /var/lib/downline/archive
```

The shipper log must report `shipped=1`, and the archive directory must contain only the archive of the current hour.

## Updating to a new version

Create the update script once:

```bash
cat > /usr/local/bin/downline-update <<'EOF'
#!/bin/sh
set -eu

version="${1:?usage: downline-update <version>}"

# Pull first, so that a wrong tag fails before anything changes.
docker pull "ghcr.io/markusheinemann/downline/collector:${version}"
docker pull "ghcr.io/markusheinemann/downline/shipper:${version}"

echo "DOWNLINE_VERSION=${version}" > /etc/downline/version.env

systemctl restart downline-collector.service
echo "updated to ${version}; the shipper uses it from its next run"
EOF
chmod +x /usr/local/bin/downline-update
```

To update, run the script with the new image tag:

```bash
downline-update <version>
```

The script restarts the collector. The shipper uses the new version from its next run. To roll back, run the script
with the previous tag.

To see the version that runs, read the version file:

```bash
cat /etc/downline/version.env
```

## Troubleshooting

Read the logs of a service with `journalctl --no-pager -u <service> -n 50`. The table lists common errors.

| Error                                                            | Cause                                                         | Solution                                                                          |
|------------------------------------------------------------------|---------------------------------------------------------------|-----------------------------------------------------------------------------------|
| `Referenced but unset environment variable ... DOWNLINE_VERSION` | `EnvironmentFile=` points to the wrong file                   | Set `EnvironmentFile=/etc/downline/version.env` and run `systemctl daemon-reload` |
| `docker: invalid reference format`                               | The image tag is empty or contains invalid characters         | Check `/etc/downline/version.env`                                                 |
| `manifest unknown` or `not found`                                | The image tag does not exist in GHCR                          | Check the tag on the GHCR package page                                            |
| `failed loading configuration` and exit status 2                 | A required option is missing                                  | Compare the environment file with [Configuration](configuration.md)               |
| `read ssh key ... permission denied`                             | The key is not readable for UID 65532                         | Repeat the `chown` and `chmod` commands from step 3                               |
| `load known hosts ... permission denied`                         | The `known_hosts` file is not readable for UID 65532          | Repeat the `chown` and `chmod` commands from step 4                               |
| `knownhosts: key mismatch`                                       | The host key of the server does not match `known_hosts`       | Repeat step 4 and compare the fingerprints                                        |
| `create remote directory: permission denied`                     | `SFTP_DIR` is outside the directories the SFTP user can write | Use a directory below the home directory of the SFTP user                         |
| `remove local ... permission denied`                             | The archive directory is not owned by UID 65532               | Repeat the `chown` command from step 2                                            |
