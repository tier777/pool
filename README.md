# Pool

Pool is a small, single-user shared note and file drop: one Go process, one SQLite database, one adjacent file directory, and a self-contained browser client.

## Quick start with Docker Compose

Requirements: Docker Engine with Compose v2 and OpenSSL.

```sh
mkdir -p secrets
umask 077
openssl rand -base64 32 > secrets/app_password
docker compose up -d --build
```

Open `http://127.0.0.1:8080` and enter the generated password. The Compose port is bound to localhost on purpose. Set `POOL_PORT` to change the host port.

Compose runs a short-lived `prepare-data` service to give UID 10001 access to the named volume. The long-running `pool` service itself is non-root, read-only outside `/data`, and runs without Linux capabilities.

Do not expose the HTTP port directly to a network. For remote access, put Caddy, nginx, Traefik, or another maintained reverse proxy in front of `127.0.0.1:8080`, terminate HTTPS there, and enable HSTS there after HTTPS is working. The browser sends the shared password during login, so plain HTTP is safe only on the local machine.

Use the gear button to open Settings. The field shows the active PIN masked; the eye reveals it. Edit the field and press Save to change it, or use the switch and Save to disable protection. The existing PIN is available only to its authenticated session: it is retained in server process memory and returned in a non-cacheable settings response, never written to the database or browser storage. Reloading the page preserves access through the session; session expiry or a server restart requires login again. Click Pool to return without saving. PINs require at least 3 characters and at most 1024 bytes. Turning protection off allows anyone who can reach Pool to read and edit its contents and settings. Changes revoke other sessions and persist in SQLite as a salted PBKDF2-SHA256 hash (600,000 iterations). The configured startup password initializes a new database only; keep the startup configuration present for server startup.

Successful login creates an eight-hour server-side session. The browser keeps only an `HttpOnly`, `SameSite=Strict` cookie; the cookie is `Secure` through HTTPS and intentionally omits `Secure` only for loopback HTTP development. Logging out, session expiry, or restarting Pool revokes the session.

## Configuration

| Variable | Default | Purpose |
| --- | --- | --- |
| `APP_PASSWORD_FILE` | unset natively | Preferred path to a password file. Compose mounts `/run/secrets/app_password`. |
| `APP_PASSWORD` | unset | Direct password fallback for native runs. Minimum 3 characters; maximum 1024 bytes. |
| `BIND_ADDRESS` | `127.0.0.1` | Listen address. Compose sets `0.0.0.0` inside the container only. |
| `PORT` | `8080` | Listen port. |
| `DATA_PATH` | `./data.db` | SQLite database path. Compose uses `/data/pool.db`. |

For a native run, install Go 1.26.6 or newer, then run:

```sh
APP_PASSWORD_FILE=/absolute/path/to/password go run .
```

## Operations

Check health:

```sh
docker compose ps
curl --fail http://127.0.0.1:8080/api/ping
```

Create a consistent backup by stopping writes before copying the database and uploaded files together:

```sh
mkdir -p backups
docker compose stop pool
docker compose cp pool:/data/. ./backups/data
docker compose start pool
```

Restore a backup into the named volume:

```sh
docker compose down
docker run --rm \
  --volume pool-data:/data \
  --volume "$PWD/backups:/backup:ro" \
  alpine:3.24 sh -c 'cp -a /backup/data/. /data/ && chown -R 10001:10001 /data && chmod -R u=rwX,go= /data'
docker compose up -d
```

Upgrade only after taking a backup:

```sh
git pull --ff-only
docker compose up -d --build
```

The server handles `SIGTERM` with a ten-second graceful shutdown, so normal Compose stop and upgrade operations close SQLite cleanly.
