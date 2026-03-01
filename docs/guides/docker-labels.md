# Docker Labels Reference

PulseBoard uses Docker labels to configure monitoring directly on your containers. No config files, no UI clicks — just add labels to your `docker-compose.yml` and PulseBoard picks them up automatically.

---

## Container Settings

| Label | Values | Description |
|-------|--------|-------------|
| `pulseboard.ignore` | `true` | Exclude this container from monitoring |
| `pulseboard.group` | any string | Custom group name (overrides Compose project) |
| `pulseboard.alert.severity` | `critical`, `warning`, `info` | Default alert severity for this container |
| `pulseboard.alert.restart_threshold` | integer | Number of restarts before triggering a restart loop alert |
| `pulseboard.alert.channels` | comma-separated | Route alerts to specific notification channels |

```yaml
labels:
  pulseboard.ignore: "true"                    # Exclude from monitoring
  pulseboard.group: "backend"                  # Custom group name
  pulseboard.alert.severity: "critical"        # Default severity
  pulseboard.alert.restart_threshold: "5"      # Restart loop threshold
  pulseboard.alert.channels: "slack,email"     # Route to channels
```

---

## Endpoint Monitoring

### Simple — One Endpoint per Container

| Label | Default | Description |
|-------|---------|-------------|
| `pulseboard.endpoint.http` | — | HTTP(S) URL to check |
| `pulseboard.endpoint.tcp` | — | TCP host:port to check |
| `pulseboard.endpoint.interval` | `30s` | Check interval (Go duration) |
| `pulseboard.endpoint.timeout` | `10s` | Request timeout |
| `pulseboard.endpoint.failure-threshold` | `1` | Consecutive failures before marking as down |
| `pulseboard.endpoint.recovery-threshold` | `1` | Consecutive successes before marking as up |

#### HTTP-Specific Options

| Label | Default | Description |
|-------|---------|-------------|
| `pulseboard.endpoint.http.method` | `GET` | HTTP method (`GET`, `HEAD`, `POST`, `PUT`, `DELETE`, `PATCH`, `OPTIONS`) |
| `pulseboard.endpoint.http.expected-status` | `200` | Expected status codes (comma-separated, e.g., `200,201`) |
| `pulseboard.endpoint.http.tls-verify` | `true` | Verify TLS certificates (`false` for self-signed) |
| `pulseboard.endpoint.http.headers` | — | Custom headers (JSON `{"K":"V"}` or `K=V,K=V` format) |
| `pulseboard.endpoint.http.max-redirects` | — | Maximum number of HTTP redirects to follow |

```yaml
labels:
  pulseboard.endpoint.http: "https://api:8443/health"
  pulseboard.endpoint.interval: "15s"
  pulseboard.endpoint.failure-threshold: "3"
  pulseboard.endpoint.http.method: "POST"
  pulseboard.endpoint.http.expected-status: "200,201"
  pulseboard.endpoint.http.tls-verify: "false"
```

### Indexed — Multiple Endpoints per Container

Use a numeric index between `endpoint` and the type to define multiple endpoints:

```yaml
labels:
  # First endpoint — HTTP health check
  pulseboard.endpoint.0.http: "https://app:8443/health"
  pulseboard.endpoint.0.interval: "15s"
  pulseboard.endpoint.0.failure-threshold: "3"

  # Second endpoint — Redis TCP check
  pulseboard.endpoint.1.tcp: "redis:6379"
  pulseboard.endpoint.1.interval: "30s"
```

!!! warning "Do not mix simple and indexed"
    Use either simple labels (`pulseboard.endpoint.http`) or indexed labels
    (`pulseboard.endpoint.0.http`). Do not mix both styles on the same container.

Global config labels (without an index) apply as defaults to all indexed endpoints. Indexed config overrides global config.

---

## Certificate Monitoring

| Label | Description |
|-------|-------------|
| `pulseboard.tls.certificates` | Comma-separated list of hostnames to monitor |

Hostnames without a port default to port 443. Schemes (`https://`) and paths are stripped automatically.

```yaml
labels:
  # Monitor three certificates
  pulseboard.tls.certificates: "api.example.com,dashboard.example.com:8443,mail.example.com"
```

!!! info "Automatic detection"
    HTTPS endpoints configured via `pulseboard.endpoint.http` automatically get
    certificate monitoring. Use `pulseboard.tls.certificates` for additional
    domains not covered by endpoint checks.

---

## Full Stack Example

```yaml
services:
  pulseboard:
    image: ghcr.io/kolapsis/pulseboard:latest
    ports:
      - "8080:8080"
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
      - pulseboard-data:/data
    environment:
      PULSEBOARD_DB: "/data/pulseboard.db"

  api:
    image: myapp:latest
    labels:
      pulseboard.group: "production"
      pulseboard.endpoint.http: "http://api:3000/health"
      pulseboard.endpoint.interval: "15s"
      pulseboard.alert.severity: "critical"
      pulseboard.alert.channels: "ops-webhook"

  postgres:
    image: postgres:16
    labels:
      pulseboard.endpoint.tcp: "postgres:5432"
      pulseboard.alert.severity: "critical"

  redis:
    image: redis:7-alpine
    labels:
      pulseboard.endpoint.tcp: "redis:6379"

  nginx:
    image: nginx:alpine
    labels:
      pulseboard.endpoint.0.http: "https://nginx:443/health"
      pulseboard.endpoint.1.tcp: "nginx:443"
      pulseboard.tls.certificates: "app.example.com,api.example.com"

  backup-runner:
    image: alpine:latest
    labels:
      pulseboard.ignore: "true"

volumes:
  pulseboard-data:
```

---

## Related

- [Endpoint Monitoring](../features/endpoints.md) — HTTP/TCP check details
- [Certificate Monitoring](../features/certificates.md) — TLS monitoring details
- [Container Monitoring](../features/containers.md) — Container labels (ignore, group)
- [Alert Engine](../features/alerts.md) — Alert routing labels
