# Network Security Insights

Automatic detection of dangerous network configurations across your containers. No setup required — maintenant analyzes container configurations as they are discovered and flags common misconfigurations.

---

## How It Works

maintenant inspects the network configuration of every monitored container and generates security insights for each detected risk. Insights are computed in-memory on each discovery cycle — there is nothing to configure.

For each container, maintenant checks:

- **Port bindings** — which ports are exposed and on which interfaces
- **Network mode** — whether the container uses host networking
- **Privileged mode** — whether the container runs with elevated privileges
- **Image ecosystem** — what software stack the image contains (for CVE context)

---

## Detected Insights

### Docker

| Insight | Severity | Description |
|---------|----------|-------------|
| `port_exposed_all_interfaces` | High | A port is bound to `0.0.0.0`, making it reachable from any network interface |
| `database_port_exposed` | Critical | A known database port (3306, 5432, 6379, 27017, etc.) is exposed without restriction |
| `privileged_container` | Critical | Container runs in privileged mode with full host access |
| `host_network_mode` | High | Container shares the host network namespace |

### Kubernetes

| Insight | Severity | Description |
|---------|----------|-------------|
| `service_load_balancer` | Medium | A Service of type LoadBalancer exposes the workload to external traffic |
| `service_node_port` | Medium | A Service of type NodePort exposes the workload on every cluster node |
| `missing_network_policy` | Medium | No NetworkPolicy restricts traffic to this workload |

---

## CVE Ecosystem Mapping

maintenant maps each container image to its software ecosystem using OCI manifest inspection. This determines which vulnerability databases are relevant for the image.

The resolver uses a multi-level fallback chain:

1. **OCI labels** — `org.opencontainers.image.*` and `maintainer` labels
2. **Manifest config** — OS and architecture metadata from the image manifest
3. **Known image database** — built-in mapping of common images (postgres, nginx, redis, node, etc.)

The detected ecosystem (e.g., `debian`, `alpine`, `python`, `node`) is used by the CVE enrichment engine (Pro) to query the correct vulnerability sources.

---

## Per-Container Detail

Security insights are displayed directly in the container detail panel. Each insight shows its severity, a description of the risk, and relevant details (port number, interface, etc.).

---

## Security Posture Dashboard :material-crown:{ title="Pro" }

With maintenant Pro, a dedicated Security Posture page aggregates insights into a scored view of infrastructure health:

- **Global posture score** — weighted score based on network exposure, update status, and configuration risks
- **Category breakdown** — scores split by network, updates, and configuration
- **Per-container risk ranking** — containers sorted by risk with drill-down to individual findings
- **Severity distribution** — counts across critical, high, and medium findings
- **Risk acknowledgment** — dismiss known findings with an audit trail

---

## API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/v1/security/insights` | List all insights across all containers |
| `GET` | `/api/v1/security/insights/{container_id}` | Get insights for a specific container |
| `GET` | `/api/v1/security/summary` | Aggregated insight counts by severity and type |
| `GET` | `/api/v1/security/posture` | Global infrastructure posture score :material-crown:{ title="Pro" } |
| `GET` | `/api/v1/security/posture/containers` | Per-container posture scores :material-crown:{ title="Pro" } |
| `GET` | `/api/v1/security/posture/containers/{id}` | Single container posture score :material-crown:{ title="Pro" } |
| `POST` | `/api/v1/security/acknowledgments` | Acknowledge a finding :material-crown:{ title="Pro" } |
| `DELETE` | `/api/v1/security/acknowledgments/{id}` | Revoke an acknowledgment :material-crown:{ title="Pro" } |
| `GET` | `/api/v1/security/acknowledgments` | List acknowledgments :material-crown:{ title="Pro" } |

---

## Related

- [Container Monitoring](containers.md) — Container detail panel shows security insights
- [Update Intelligence](updates.md) — CVE enrichment uses ecosystem data from security analysis
