package mcp

import (
	"context"
	"log/slog"

	"github.com/kolapsis/pulseboard/internal/alert"
	"github.com/kolapsis/pulseboard/internal/certificate"
	"github.com/kolapsis/pulseboard/internal/container"
	"github.com/kolapsis/pulseboard/internal/endpoint"
	"github.com/kolapsis/pulseboard/internal/extension"
	"github.com/kolapsis/pulseboard/internal/heartbeat"
	"github.com/kolapsis/pulseboard/internal/resource"
	"github.com/kolapsis/pulseboard/internal/runtime"
	"github.com/kolapsis/pulseboard/internal/update"
	gomcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// LogFetcher fetches container logs from the runtime.
type LogFetcher interface {
	FetchLogs(ctx context.Context, externalID string, lines int, timestamps bool) ([]string, error)
}

// Services holds all dependencies required by MCP tool handlers.
type Services struct {
	Containers   *container.Service
	Endpoints    *endpoint.Service
	Heartbeats   *heartbeat.Service
	Certificates *certificate.Service
	Resources    *resource.Service
	Alerts       alert.AlertStore
	Updates      *update.Service
	Incidents    extension.IncidentManager
	Maintenance  extension.MaintenanceScheduler
	Runtime      runtime.Runtime
	LogFetcher   LogFetcher
	Version      string
	Logger       *slog.Logger
}

// NewServer creates and configures an MCP server with all PulseBoard tools registered.
func NewServer(svc *Services) *gomcp.Server {
	server := gomcp.NewServer(&gomcp.Implementation{
		Name:    "pulseboard",
		Version: svc.Version,
	}, &gomcp.ServerOptions{
		Instructions: "PulseBoard infrastructure monitoring server. Provides real-time access to container states, endpoint health, heartbeat monitors, TLS certificates, resource metrics, alerts, and update intelligence.",
		Logger:       svc.Logger,
	})

	registerReadTools(server, svc)
	registerWriteTools(server, svc)

	return server
}
