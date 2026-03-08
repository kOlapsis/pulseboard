// Copyright 2026 Benjamin Touchard (Kolapsis)
//
// Licensed under the GNU Affero General Public License v3.0 (AGPL-3.0)
// or a commercial license. You may not use this file except in compliance
// with one of these licenses.
//
// AGPL-3.0: https://www.gnu.org/licenses/agpl-3.0.html
// Commercial: See COMMERCIAL-LICENSE.md
//
// Source: https://github.com/kolapsis/maintenant

package app

import (
	"context"

	"github.com/kolapsis/maintenant/internal/certificate"
	"github.com/kolapsis/maintenant/internal/container"
	"github.com/kolapsis/maintenant/internal/endpoint"
	"github.com/kolapsis/maintenant/internal/heartbeat"
	"github.com/kolapsis/maintenant/internal/status"
)

// ContainerStatus derives the status page status for a container.
func ContainerStatus(c *container.Container) string {
	switch c.State {
	case container.StateRunning:
		if c.HealthStatus != nil && *c.HealthStatus == container.HealthUnhealthy {
			return status.StatusDegraded
		}
		return status.StatusOperational
	case container.StateCompleted:
		return status.StatusOperational
	default:
		return status.StatusMajorOutage
	}
}

// EndpointStatus derives the status page status for an endpoint.
func EndpointStatus(ep *endpoint.Endpoint) string {
	switch ep.Status {
	case endpoint.StatusUp:
		return status.StatusOperational
	case endpoint.StatusDown:
		return status.StatusMajorOutage
	default:
		return status.StatusOperational
	}
}

// HeartbeatStatus derives the status page status for a heartbeat.
func HeartbeatStatus(hb *heartbeat.Heartbeat) string {
	switch hb.Status {
	case heartbeat.StatusUp:
		return status.StatusOperational
	case heartbeat.StatusDown:
		return status.StatusMajorOutage
	default:
		return status.StatusDegraded
	}
}

// CertificateStatus derives the status page status for a certificate monitor.
func CertificateStatus(cert *certificate.CertMonitor) string {
	switch cert.Status {
	case certificate.StatusValid:
		return status.StatusOperational
	case certificate.StatusExpiring:
		return status.StatusDegraded
	default:
		return status.StatusMajorOutage
	}
}

// WorstStatus returns the most severe status between two values.
func WorstStatus(a, b string) string {
	if status.Severity(a) >= status.Severity(b) {
		return a
	}
	return b
}

// wireStatusProvider sets up the monitor status provider for the status page.
func (a *App) wireStatusProvider() {
	a.statusSvc.SetMonitorStatusProvider(func(ctx context.Context, monitorType string, monitorID int64) string {
		switch monitorType {
		case "container":
			if monitorID != 0 {
				c, err := a.containerSvc.GetContainer(ctx, monitorID)
				if err != nil || c == nil {
					return status.StatusOperational
				}
				return ContainerStatus(c)
			}
			containers, err := a.containerSvc.ListContainers(ctx, container.ListContainersOpts{})
			if err != nil {
				return status.StatusOperational
			}
			worst := status.StatusOperational
			for _, c := range containers {
				worst = WorstStatus(worst, ContainerStatus(c))
			}
			return worst
		case "endpoint":
			if monitorID != 0 {
				ep, err := a.endpointSvc.GetEndpoint(ctx, monitorID)
				if err != nil || ep == nil {
					return status.StatusOperational
				}
				return EndpointStatus(ep)
			}
			endpoints, err := a.endpointSvc.ListEndpoints(ctx, endpoint.ListEndpointsOpts{})
			if err != nil {
				return status.StatusOperational
			}
			worst := status.StatusOperational
			for _, ep := range endpoints {
				worst = WorstStatus(worst, EndpointStatus(ep))
			}
			return worst
		case "heartbeat":
			if monitorID != 0 {
				hb, err := a.heartbeatSvc.GetHeartbeat(ctx, monitorID)
				if err != nil || hb == nil {
					return status.StatusOperational
				}
				return HeartbeatStatus(hb)
			}
			heartbeats, err := a.heartbeatSvc.ListHeartbeats(ctx, heartbeat.ListHeartbeatsOpts{})
			if err != nil {
				return status.StatusOperational
			}
			worst := status.StatusOperational
			for _, hb := range heartbeats {
				worst = WorstStatus(worst, HeartbeatStatus(hb))
			}
			return worst
		case "certificate":
			if monitorID != 0 {
				cert, err := a.certSvc.GetMonitor(ctx, monitorID)
				if err != nil || cert == nil {
					return status.StatusOperational
				}
				return CertificateStatus(cert)
			}
			certs, err := a.certSvc.ListMonitors(ctx, certificate.ListCertificatesOpts{})
			if err != nil {
				return status.StatusOperational
			}
			worst := status.StatusOperational
			for _, cert := range certs {
				worst = WorstStatus(worst, CertificateStatus(cert))
			}
			return worst
		}
		return status.StatusOperational
	})
}
