# API Reference

All endpoints are under `/api/v1/`. Responses are JSON. Errors follow a standard format:

```json
{
  "error": {
    "code": "not_found",
    "message": "Container not found"
  }
}
```

---

## Health

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/v1/health` | Health check, returns `{"status": "ok", "version": "..."}` |
| `GET` | `/api/v1/runtime/status` | Runtime info (docker/kubernetes, connection state) |
| `GET` | `/api/v1/edition` | Edition and feature flags |

---

## Containers

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/v1/containers` | List all containers with groups |
| `GET` | `/api/v1/containers/{id}` | Get container details with uptime stats |
| `GET` | `/api/v1/containers/{id}/transitions` | List state transitions |
| `GET` | `/api/v1/containers/{id}/logs` | Fetch recent logs |
| `GET` | `/api/v1/containers/{id}/logs/stream` | Stream logs in real time (SSE) |
| `GET` | `/api/v1/containers/{id}/endpoints` | List endpoints for a container |

---

## Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/v1/endpoints` | List all monitored endpoints |
| `GET` | `/api/v1/endpoints/{id}` | Get endpoint details |
| `GET` | `/api/v1/endpoints/{id}/checks` | List check results |
| `GET` | `/api/v1/endpoints/{id}/uptime/daily` | Daily uptime percentages |

---

## Heartbeats

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/v1/heartbeats` | List all heartbeat monitors |
| `POST` | `/api/v1/heartbeats` | Create a heartbeat monitor |
| `GET` | `/api/v1/heartbeats/{id}` | Get a heartbeat monitor |
| `PUT` | `/api/v1/heartbeats/{id}` | Update a heartbeat monitor |
| `DELETE` | `/api/v1/heartbeats/{id}` | Delete a heartbeat monitor |
| `POST` | `/api/v1/heartbeats/{id}/pause` | Pause deadline checking |
| `POST` | `/api/v1/heartbeats/{id}/resume` | Resume deadline checking |
| `GET` | `/api/v1/heartbeats/{id}/executions` | List executions |
| `GET` | `/api/v1/heartbeats/{id}/pings` | List raw pings |
| `GET` | `/api/v1/heartbeats/{id}/uptime/daily` | Daily uptime percentages |

### Ping Endpoints (Public)

These routes do not require authentication:

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET/POST` | `/ping/{uuid}` | Simple ping (success) |
| `GET/POST` | `/ping/{uuid}/start` | Signal job start |
| `GET/POST` | `/ping/{uuid}/{exit_code}` | Ping with exit code (0 = success) |

---

## Certificates

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/v1/certificates` | List all certificate monitors |
| `POST` | `/api/v1/certificates` | Create a standalone certificate monitor |
| `GET` | `/api/v1/certificates/{id}` | Get certificate details |
| `PUT` | `/api/v1/certificates/{id}` | Update a certificate monitor |
| `DELETE` | `/api/v1/certificates/{id}` | Delete a certificate monitor |
| `GET` | `/api/v1/certificates/{id}/checks` | List check history |

---

## Resources

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/v1/containers/{id}/resources/current` | Current CPU, memory, network, I/O |
| `GET` | `/api/v1/containers/{id}/resources/history` | Historical metrics (`?range=24h`) |
| `GET` | `/api/v1/containers/{id}/resources/alerts` | Get alert thresholds |
| `PUT` | `/api/v1/containers/{id}/resources/alerts` | Set alert thresholds |
| `GET` | `/api/v1/resources/summary` | Aggregate resource summary |
| `GET` | `/api/v1/resources/top` | Top consumers (`?sort=cpu&limit=10`) |

---

## Alerts

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/v1/alerts` | List all alerts (including resolved) |
| `GET` | `/api/v1/alerts/active` | List active (unresolved) alerts |
| `GET` | `/api/v1/alerts/{id}` | Get alert details |

---

## Notification Channels

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/v1/channels` | List notification channels |
| `POST` | `/api/v1/channels` | Create a channel (slack, discord, teams, webhook) |
| `PUT` | `/api/v1/channels/{id}` | Update a channel |
| `DELETE` | `/api/v1/channels/{id}` | Delete a channel |
| `POST` | `/api/v1/channels/{id}/test` | Send a test alert |
| `POST` | `/api/v1/channels/{id}/rules` | Create a routing rule |
| `DELETE` | `/api/v1/channels/{id}/rules/{rule_id}` | Delete a routing rule |

---

## Silence Rules

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/v1/silence` | List active silence rules |
| `POST` | `/api/v1/silence` | Create a silence rule |
| `DELETE` | `/api/v1/silence/{id}` | Cancel a silence rule |

---

## Webhooks

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/v1/webhooks` | List webhook subscriptions |
| `POST` | `/api/v1/webhooks` | Create a webhook subscription |
| `DELETE` | `/api/v1/webhooks/{id}` | Delete a webhook subscription |
| `POST` | `/api/v1/webhooks/{id}/test` | Send a test payload |

---

## Status Page (Admin)

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/v1/status/groups` | List component groups |
| `POST` | `/api/v1/status/groups` | Create a group |
| `PUT` | `/api/v1/status/groups/{id}` | Update a group |
| `DELETE` | `/api/v1/status/groups/{id}` | Delete a group |
| `GET` | `/api/v1/status/components` | List components |
| `POST` | `/api/v1/status/components` | Create a component |
| `PUT` | `/api/v1/status/components/{id}` | Update a component |
| `DELETE` | `/api/v1/status/components/{id}` | Delete a component |

---

## Updates

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/v1/updates` | List available updates (`?status=&update_type=`) |
| `GET` | `/api/v1/updates/summary` | Update summary with counts |
| `POST` | `/api/v1/updates/scan` | Trigger a manual scan |
| `GET` | `/api/v1/updates/scan/{scan_id}` | Get scan status |
| `GET` | `/api/v1/updates/dry-run` | Preview what a scan would check |
| `GET` | `/api/v1/updates/container/{container_id}` | Update details for a container |
| `POST` | `/api/v1/updates/pin/{container_id}` | Pin current version |
| `DELETE` | `/api/v1/updates/pin/{container_id}` | Unpin version |
| `GET` | `/api/v1/updates/exclusions` | List exclusions |
| `POST` | `/api/v1/updates/exclusions` | Create an exclusion |
| `DELETE` | `/api/v1/updates/exclusions/{id}` | Delete an exclusion |

---

## Dashboard

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/v1/dashboard/sparklines` | Sparkline data for all endpoints |

---

## SSE Event Stream

Connect to the real-time event stream:

```
GET /api/v1/containers/events
```

This is a Server-Sent Events (SSE) endpoint. Each event has a `type` field and a JSON `data` payload.

### Event Types

| Event | Source | Description |
|-------|--------|-------------|
| `container.state_changed` | Container | State transition (running, stopped, etc.) |
| `container.health_changed` | Container | Health check status change |
| `container.restart_alert` | Container | Restart loop detected |
| `endpoint.check_result` | Endpoint | Check completed (up/down, response time) |
| `endpoint.alert` | Endpoint | Consecutive failure threshold reached |
| `endpoint.recovery` | Endpoint | Endpoint recovered |
| `heartbeat.pinged` | Heartbeat | Ping received |
| `heartbeat.deadline_missed` | Heartbeat | Missed deadline |
| `certificate.alert` | Certificate | Expiry warning |
| `certificate.recovery` | Certificate | Certificate renewed |
| `resource.snapshot` | Resource | New metrics snapshot |
| `resource.alert` | Resource | Threshold exceeded |
| `resource.recovery` | Resource | Usage returned to normal |
| `update.scan_started` | Update | Scan in progress |
| `update.scan_completed` | Update | Scan finished |
| `update.detected` | Update | New update found |
| `update.pinned` | Update | Version pinned |
| `update.unpinned` | Update | Version unpinned |

### Status Page SSE

The public status page has its own event stream:

```
GET /status/events
```
