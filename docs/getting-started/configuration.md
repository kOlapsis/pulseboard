# Configuration

PulseBoard is configured entirely through environment variables. No configuration files required.

---

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `PULSEBOARD_ADDR` | `127.0.0.1:8080` | HTTP bind address. Use `0.0.0.0:8080` inside containers. |
| `PULSEBOARD_DB` | `./pulseboard.db` | SQLite database file path. |
| `PULSEBOARD_BASE_URL` | `http://localhost:8080` | Public base URL. Used for heartbeat ping URLs and status page links. |
| `PULSEBOARD_CORS_ORIGINS` | same-origin | CORS allowed origins (comma-separated). Empty means same-origin only. Set to `*` for wildcard. |
| `PULSEBOARD_RUNTIME` | auto-detect | Force container runtime: `docker` or `kubernetes`. Auto-detected by default. |
| `PULSEBOARD_MAX_BODY_SIZE` | `1048576` | Maximum request body size in bytes for POST/PUT requests (default: 1 MB). |
| `PULSEBOARD_UPDATE_INTERVAL` | `24h` | Update intelligence scan interval. Accepts Go duration format (e.g., `12h`, `30m`). |
| `PULSEBOARD_K8S_NAMESPACES` | all | Kubernetes namespace allowlist (comma-separated). Empty monitors all namespaces. |
| `PULSEBOARD_K8S_EXCLUDE_NAMESPACES` | none | Kubernetes namespace blocklist (comma-separated). |

### Example `.env` File

```bash
# Listen address (use 0.0.0.0 inside containers, 127.0.0.1 on host)
PULSEBOARD_ADDR=127.0.0.1:8080

# SQLite database path
PULSEBOARD_DB=./pulseboard.db

# Public base URL (used for heartbeat ping URLs and status page links)
PULSEBOARD_BASE_URL=https://pulseboard.example.com

# CORS allowed origins (comma-separated, empty = same-origin only)
# PULSEBOARD_CORS_ORIGINS=http://localhost:5173

# Container runtime override (auto-detected by default: docker or kubernetes)
# PULSEBOARD_RUNTIME=docker

# Max request body size in bytes (default: 1MB)
# PULSEBOARD_MAX_BODY_SIZE=1048576

# Update intelligence scan interval (Go duration, default: 24h)
# PULSEBOARD_UPDATE_INTERVAL=24h

# Kubernetes namespaces to monitor (comma-separated, empty = all)
# PULSEBOARD_K8S_NAMESPACES=default,production

# Kubernetes namespaces to exclude (comma-separated)
# PULSEBOARD_K8S_EXCLUDE_NAMESPACES=kube-system
```

---

## Security Model

PulseBoard does not include built-in authentication — by design.

Like Dozzle, Prometheus, and most self-hosted monitoring tools, PulseBoard is designed to sit behind your existing reverse proxy and auth middleware. No need to manage yet another set of user accounts.

```
Internet  ->  Reverse Proxy (Traefik / Caddy / nginx)
          ->  Auth (Authelia / Authentik / OAuth2 Proxy)
          ->  PulseBoard
```

### Example: Traefik + Authelia

```yaml
services:
  pulseboard:
    image: ghcr.io/kolapsis/pulseboard:latest
    labels:
      traefik.enable: "true"
      traefik.http.routers.pulseboard.rule: "Host(`pulse.example.com`)"
      traefik.http.routers.pulseboard.middlewares: "authelia@docker"
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
      - pulseboard-data:/data
    environment:
      PULSEBOARD_DB: "/data/pulseboard.db"
      PULSEBOARD_BASE_URL: "https://pulse.example.com"
```

---

## Public Routes

Two route prefixes are designed to be publicly accessible and should bypass your authentication middleware:

| Route | Purpose |
|-------|---------|
| `/ping/{uuid}` | Heartbeat ping endpoint. Called by cron jobs and external services. |
| `/status/` | Public status page. Visible to your end users. |

!!! warning "Proxy configuration"
    Make sure your reverse proxy rules allow unauthenticated access to `/ping/` and `/status/` paths.
    All other routes (especially `/api/v1/`) should require authentication.

### Traefik Example: Bypassing Auth for Public Routes

```yaml
labels:
  # Main router with auth
  traefik.http.routers.pulseboard.rule: "Host(`pulse.example.com`)"
  traefik.http.routers.pulseboard.middlewares: "authelia@docker"

  # Public routes without auth
  traefik.http.routers.pulseboard-public.rule: >
    Host(`pulse.example.com`) &&
    (PathPrefix(`/ping/`) || PathPrefix(`/status/`))
  traefik.http.routers.pulseboard-public.priority: "100"
```

---

## Database

PulseBoard uses SQLite in WAL (Write-Ahead Logging) mode with a single-writer pattern. This provides excellent read performance while maintaining data integrity.

- The database file is created automatically on first startup.
- Migrations run automatically — no manual steps required.
- Back up the database by copying the `.db`, `.db-wal`, and `.db-shm` files while PulseBoard is stopped, or use `sqlite3 backup` while running.

!!! tip "Persistence in Docker"
    Always mount a volume for the database directory to persist data across container restarts:
    ```yaml
    volumes:
      - pulseboard-data:/data
    environment:
      PULSEBOARD_DB: "/data/pulseboard.db"
    ```
