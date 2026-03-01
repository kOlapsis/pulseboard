# Endpoint Monitoring

Define HTTP or TCP checks directly as Docker labels — no config files, no UI clicks. PulseBoard picks them up automatically when a container starts.

---

## How It Works

PulseBoard reads endpoint definitions from Docker labels on your containers. When a container with endpoint labels starts, PulseBoard automatically begins monitoring those endpoints at the configured interval.

Each check records:

- **Response time** — How long the endpoint took to respond
- **Status** — Up or down, based on HTTP status code or TCP connection success
- **Uptime history** — 90-day uptime with daily breakdowns and sparkline charts

---

## Quick Start

Add labels to any container in your `docker-compose.yml`:

```yaml
services:
  api:
    image: myapp:latest
    labels:
      pulseboard.endpoint.http: "http://api:3000/health"
      pulseboard.endpoint.interval: "15s"
```

That is it. PulseBoard starts checking `http://api:3000/health` every 15 seconds as soon as the container starts.

---

## HTTP Checks

HTTP checks send a request to the configured URL and validate the response status code.

```yaml
labels:
  pulseboard.endpoint.http: "https://api:8443/health"
```

### Configuration Options

| Label | Default | Description |
|-------|---------|-------------|
| `pulseboard.endpoint.http` | — | URL to check (required for HTTP) |
| `pulseboard.endpoint.http.method` | `GET` | HTTP method (`GET`, `POST`, `HEAD`, etc.) |
| `pulseboard.endpoint.http.expected-status` | `200` | Expected status codes (comma-separated, e.g., `200,201`) |
| `pulseboard.endpoint.http.tls-verify` | `true` | Verify TLS certificates. Set to `false` for self-signed certs. |
| `pulseboard.endpoint.interval` | `30s` | Check interval (Go duration format) |
| `pulseboard.endpoint.timeout` | `10s` | Request timeout |
| `pulseboard.endpoint.failure-threshold` | `1` | Consecutive failures before marking as down |
| `pulseboard.endpoint.recovery-threshold` | `1` | Consecutive successes before marking as up |

---

## TCP Checks

TCP checks attempt to establish a connection to the configured host and port.

```yaml
labels:
  pulseboard.endpoint.tcp: "postgres:5432"
```

Useful for databases, caches, and services that do not expose HTTP endpoints.

---

## Multiple Endpoints per Container

Use indexed labels to monitor multiple endpoints from a single container:

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

!!! info "Indexed vs simple labels"
    You can use either **simple** labels (`pulseboard.endpoint.http`) for a single endpoint
    or **indexed** labels (`pulseboard.endpoint.0.http`, `pulseboard.endpoint.1.tcp`) for multiple
    endpoints. Do not mix both styles on the same container.

---

## Failure and Recovery Thresholds

Thresholds control how many consecutive check results are needed to change the endpoint status. This prevents flapping from transient network issues.

```yaml
labels:
  pulseboard.endpoint.http: "https://api:3000/health"
  pulseboard.endpoint.failure-threshold: "3"   # 3 consecutive failures = down
  pulseboard.endpoint.recovery-threshold: "2"  # 2 consecutive successes = up
```

- **failure-threshold** — Number of consecutive failures before the endpoint is marked as `down` and an alert is triggered.
- **recovery-threshold** — Number of consecutive successes before the endpoint is marked as `up` again.

---

## Uptime History and Sparklines

PulseBoard records every check result and computes:

- **Daily uptime percentage** — Available via `GET /api/v1/endpoints/{id}/uptime/daily`
- **Response time trends** — Visualized as sparkline charts in the dashboard
- **90-day history** — Long-term uptime tracking

---

## Related

- [Docker Labels Reference](../guides/docker-labels.md) — Complete label reference
- [TLS Certificate Monitoring](certificates.md) — HTTPS endpoints automatically get certificate monitoring
- [Alert Engine](alerts.md) — Configure alerts for endpoint failures
