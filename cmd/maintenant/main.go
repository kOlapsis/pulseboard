// Copyright 2026 Benjamin Touchard (Kolapsis)
//
// Licensed under the GNU Affero General Public License v3.0 (AGPL-3.0)
// or a commercial license. You may not use this file except in compliance
// with one of these licenses.
//
// AGPL-3.0: https://www.gnu.org/licenses/agpl-3.0.html
// Commercial: See LICENSE-COMMERCIAL.md
//
// Source: https://github.com/kolapsis/maintenant

package main

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/kolapsis/maintenant/cmd/maintenant/web"
	"github.com/kolapsis/maintenant/internal/alert"
	v1 "github.com/kolapsis/maintenant/internal/api/v1"
	"github.com/kolapsis/maintenant/internal/certificate"
	"github.com/kolapsis/maintenant/internal/container"
	"github.com/kolapsis/maintenant/internal/docker"
	"github.com/kolapsis/maintenant/internal/endpoint"
	"github.com/kolapsis/maintenant/internal/event"
	"github.com/kolapsis/maintenant/internal/extension"
	"github.com/kolapsis/maintenant/internal/heartbeat"
	_ "github.com/kolapsis/maintenant/internal/kubernetes"
	"github.com/kolapsis/maintenant/internal/license"
	pbmcp "github.com/kolapsis/maintenant/internal/mcp"
	mcpoauth "github.com/kolapsis/maintenant/internal/mcp/oauth"
	"github.com/kolapsis/maintenant/internal/ratelimit"
	"github.com/kolapsis/maintenant/internal/resource"
	"github.com/kolapsis/maintenant/internal/security"
	pbruntime "github.com/kolapsis/maintenant/internal/runtime"
	"github.com/kolapsis/maintenant/internal/status"
	"github.com/kolapsis/maintenant/internal/store/sqlite"
	"github.com/kolapsis/maintenant/internal/update"
	"github.com/kolapsis/maintenant/internal/webhook"
	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"
	gomcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/modelcontextprotocol/go-sdk/oauthex"
)

var (
	version      = "dev"
	commit       = "unknown"
	buildDate    = "unknown"
	publicKeyB64 = ""
)

func main() {
	mcpStdio := len(os.Args) > 1 && os.Args[1] == "--mcp-stdio"

	logLevel := slog.LevelInfo
	logOutput := os.Stdout
	if mcpStdio {
		// In stdio mode, logs go to stderr to keep stdout clean for MCP protocol.
		logOutput = os.Stderr
	}
	logger := slog.New(slog.NewJSONHandler(logOutput, &slog.HandlerOptions{
		Level: logLevel,
	}))
	logger.Info("maintenant starting", "version", version, "commit", commit, "build_date", buildDate)
	v1.SetBuildVersion(version)
	v1.SetOrganisationName(envOrDefault("MAINTENANT_ORGANISATION_NAME", "Maintenant"))

	// Configuration from environment
	addr := envOrDefault("MAINTENANT_ADDR", "127.0.0.1:8080")
	dbPath := envOrDefault("MAINTENANT_DB", "./maintenant.db")

	// K8s namespace config (used when runtime is Kubernetes)
	k8sNamespaces := os.Getenv("MAINTENANT_K8S_NAMESPACES")
	k8sExcludeNamespaces := os.Getenv("MAINTENANT_K8S_EXCLUDE_NAMESPACES")
	if k8sNamespaces != "" {
		logger.Info("K8s namespace allowlist configured", "namespaces", k8sNamespaces)
	}
	if k8sExcludeNamespaces != "" {
		logger.Info("K8s namespace blocklist configured", "exclude_namespaces", k8sExcludeNamespaces)
	}

	// Graceful shutdown context
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// --- Database ---
	db, err := sqlite.Open(dbPath, logger)
	if err != nil {
		logger.Error("failed to open database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := sqlite.Migrate(db.ReadDB(), logger); err != nil {
		logger.Error("failed to run migrations", "error", err)
		os.Exit(1)
	}

	db.StartWriter(ctx)

	store := sqlite.NewContainerStore(db)
	epStore := sqlite.NewEndpointStore(db)
	hbStore := sqlite.NewHeartbeatStore(db)
	certStore := sqlite.NewCertificateStore(db)
	resStore := sqlite.NewResourceStore(db)
	alertStore := sqlite.NewAlertStore(db)
	channelStore := sqlite.NewChannelStore(db)
	silenceStore := sqlite.NewSilenceStore(db)
	statusCompStore := sqlite.NewStatusComponentStore(db)
	incidentStore := sqlite.NewIncidentStore(db)
	maintenanceStore := sqlite.NewMaintenanceStore(db)
	subscriberStore := sqlite.NewSubscriberStore(db)
	webhookStore := sqlite.NewWebhookStore(db)
	updateStore := sqlite.NewUpdateStore(db)

	// --- License manager ---
	license.InitPublicKey(publicKeyB64)
	licenseKey := os.Getenv("MAINTENANT_LICENSE_KEY")
	var licenseMgr *license.LicenseManager
	if licenseKey != "" {
		dataDir := filepath.Dir(dbPath)
		var err error
		licenseMgr, err = license.NewLicenseManager(licenseKey, dataDir, version, logger)
		if err != nil {
			logger.Warn("license manager initialization failed, running as Community Edition", "error", err)
		} else {
			extension.CurrentEdition = func() extension.Edition {
				if licenseMgr.IsProEnabled() {
					return extension.Enterprise
				}
				return extension.Community
			}
			licenseMgr.Start(ctx)
			defer licenseMgr.Stop()
			logger.Info("license manager started")
		}
	}

	// --- Runtime detection ---
	rt, err := pbruntime.Detect(ctx, logger)
	if err != nil {
		logger.Error("failed to detect container runtime", "error", err)
		os.Exit(1)
	}
	defer rt.Close()

	// Connect to runtime (with retry)
	if err := rt.Connect(ctx); err != nil {
		logger.Error("failed to connect to runtime", "runtime", rt.Name(), "error", err)
		os.Exit(1)
	}

	// --- Services ---
	svc := container.NewService(store, logger)
	if lf, ok := rt.(container.LogFetcher); ok {
		svc.SetLogFetcher(lf)
	}
	svc.SetRestartChecker(alert.NewRestartDetector(store, logger))
	svc.SetDiscoverer(rt)
	uptimeCalc := container.NewUptimeCalculator(store)

	// --- Security insights ---
	securitySvc := security.NewService(logger)

	// --- Resource monitoring ---
	resSvc := resource.NewService(resStore, rt, svc, logger)

	// --- Certificate monitoring ---
	certSvc := certificate.NewService(certStore, logger)

	// --- Endpoint monitoring ---
	var epSvc *endpoint.Service

	// Wire check result callback (including certificate auto-detection)
	checkEngine := endpoint.NewCheckEngine(func(endpointID int64, result endpoint.CheckResult) {
		epSvc.ProcessCheckResult(ctx, endpointID, result)

		// Auto-detect TLS certificates from HTTPS endpoint checks
		if len(result.TLSPeerCertificates) > 0 {
			ep, err := epSvc.GetEndpoint(ctx, endpointID)
			if err == nil && ep != nil && certificate.IsHTTPS(ep.Target) {
				certSvc.ProcessAutoDetectedCerts(ctx, endpointID, ep.Target, result.TLSPeerCertificates)
			}
		}
	}, logger)
	epSvc = endpoint.NewService(epStore, checkEngine, logger)
	alertDetector := alert.NewEndpointAlertDetector()
	epSvc.Start(ctx)

	// --- Heartbeat monitoring ---
	hbSvc := heartbeat.NewService(hbStore, logger, nil)
	hbSvc.SetBaseURL(envOrDefault("MAINTENANT_BASE_URL", "http://"+addr))
	hbSvc.StartDeadlineChecker(ctx)

	// --- SMTP configuration ---
	smtpHost := os.Getenv("MAINTENANT_SMTP_HOST")
	var smtpSender *alert.SMTPSender
	if smtpHost != "" {
		smtpSender = alert.NewSMTPSender(alert.SMTPConfig{
			Host:     smtpHost,
			Port:     envOrDefault("MAINTENANT_SMTP_PORT", "587"),
			Username: os.Getenv("MAINTENANT_SMTP_USERNAME"),
			Password: os.Getenv("MAINTENANT_SMTP_PASSWORD"),
			From:     envOrDefault("MAINTENANT_SMTP_FROM", "maintenant@localhost"),
		})
		logger.Info("SMTP sender configured", "host", smtpHost)
	}

	// --- Alert engine ---
	alertEngine := alert.NewEngine(alertStore, channelStore, silenceStore, logger)
	notifier := alert.NewNotifier(channelStore, logger)
	if smtpSender != nil {
		notifier.SetSMTPSender(smtpSender)
	}
	alertEngine.SetNotifier(notifier)

	// --- HTTP server ---
	broker := v1.NewSSEBroker(logger)

	// Wire SSE broadcaster to alert engine
	alertEngine.SetBroadcaster(alert.NewSSEBroadcasterFunc(func(eventType string, data interface{}) {
		broker.Broadcast(v1.SSEEvent{Type: eventType, Data: data})
	}))

	alertEngine.Start(ctx)
	notifier.Start(ctx)

	// --- Public Status Page ---
	statusBroker := v1.NewSSEBroker(logger)
	statusSvc := status.NewService(statusCompStore, logger)
	// containerStatus derives the status page status for a single container.
	containerStatus := func(c *container.Container) string {
		switch c.State {
		case container.StateRunning:
			if c.HealthStatus != nil && *c.HealthStatus == container.HealthUnhealthy {
				return status.StatusDegraded
			}
			return status.StatusOperational
		case container.StateCompleted:
			// Exited with code 0 (migration, seed, init job) — not an error.
			return status.StatusOperational
		default:
			return status.StatusMajorOutage
		}
	}

	// endpointStatus derives the status page status for a single endpoint.
	endpointStatus := func(ep *endpoint.Endpoint) string {
		switch ep.Status {
		case endpoint.StatusUp:
			return status.StatusOperational
		case endpoint.StatusDown:
			return status.StatusMajorOutage
		default:
			return status.StatusOperational
		}
	}

	// heartbeatStatus derives the status page status for a single heartbeat.
	heartbeatStatus := func(hb *heartbeat.Heartbeat) string {
		switch hb.Status {
		case heartbeat.StatusUp:
			return status.StatusOperational
		case heartbeat.StatusDown:
			return status.StatusMajorOutage
		default:
			return status.StatusDegraded
		}
	}

	// certificateStatus derives the status page status for a single certificate monitor.
	certificateStatus := func(cert *certificate.CertMonitor) string {
		switch cert.Status {
		case certificate.StatusValid:
			return status.StatusOperational
		case certificate.StatusExpiring:
			return status.StatusDegraded
		default:
			return status.StatusMajorOutage
		}
	}

	// worstStatus returns the most severe status between two values.
	worstStatus := func(a, b string) string {
		if status.Severity(a) >= status.Severity(b) {
			return a
		}
		return b
	}

	statusSvc.SetMonitorStatusProvider(func(ctx context.Context, monitorType string, monitorID int64) string {
		// When monitorID is 0, aggregate the worst status across all monitors of the type.
		switch monitorType {
		case "container":
			if monitorID != 0 {
				c, err := svc.GetContainer(ctx, monitorID)
				if err != nil || c == nil {
					return status.StatusOperational
				}
				return containerStatus(c)
			}
			containers, err := svc.ListContainers(ctx, container.ListContainersOpts{})
			if err != nil {
				return status.StatusOperational
			}
			worst := status.StatusOperational
			for _, c := range containers {
				worst = worstStatus(worst, containerStatus(c))
			}
			return worst
		case "endpoint":
			if monitorID != 0 {
				ep, err := epSvc.GetEndpoint(ctx, monitorID)
				if err != nil || ep == nil {
					return status.StatusOperational
				}
				return endpointStatus(ep)
			}
			endpoints, err := epSvc.ListEndpoints(ctx, endpoint.ListEndpointsOpts{})
			if err != nil {
				return status.StatusOperational
			}
			worst := status.StatusOperational
			for _, ep := range endpoints {
				worst = worstStatus(worst, endpointStatus(ep))
			}
			return worst
		case "heartbeat":
			if monitorID != 0 {
				hb, err := hbSvc.GetHeartbeat(ctx, monitorID)
				if err != nil || hb == nil {
					return status.StatusOperational
				}
				return heartbeatStatus(hb)
			}
			heartbeats, err := hbSvc.ListHeartbeats(ctx, heartbeat.ListHeartbeatsOpts{})
			if err != nil {
				return status.StatusOperational
			}
			worst := status.StatusOperational
			for _, hb := range heartbeats {
				worst = worstStatus(worst, heartbeatStatus(hb))
			}
			return worst
		case "certificate":
			if monitorID != 0 {
				cert, err := certSvc.GetMonitor(ctx, monitorID)
				if err != nil || cert == nil {
					return status.StatusOperational
				}
				return certificateStatus(cert)
			}
			certs, err := certSvc.ListMonitors(ctx, certificate.ListCertificatesOpts{})
			if err != nil {
				return status.StatusOperational
			}
			worst := status.StatusOperational
			for _, cert := range certs {
				worst = worstStatus(worst, certificateStatus(cert))
			}
			return worst
		}
		return status.StatusOperational
	})
	statusSvc.SetBroadcaster(func(eventType string, data interface{}) {
		statusBroker.Broadcast(v1.SSEEvent{Type: eventType, Data: data})
	})
	statusSvc.SetIncidentStore(incidentStore)
	statusSvc.SetMaintenanceStore(maintenanceStore)

	// Subscriber service for email notifications
	baseURL := envOrDefault("MAINTENANT_BASE_URL", "http://"+addr)
	subscriberSvc := status.NewSubscriberService(subscriberStore, nil, baseURL, logger)
	statusSvc.SetSubscriberService(subscriberSvc)

	// Maintenance scheduler
	maintScheduler := status.NewMaintenanceScheduler(maintenanceStore, statusCompStore, incidentStore, statusSvc, logger)

	statusHandler := status.NewHandler(statusSvc, statusBroker, logger)

	router := v1.NewRouter(broker, rt, svc, uptimeCalc, epSvc, hbSvc, certSvc, resSvc, logger, v1.AlertOpts{
		AlertStore:   alertStore,
		ChannelStore: channelStore,
		SilenceStore: silenceStore,
		Notifier:     notifier,
	}, v1.APIConfig{
		WebhookStore: webhookStore,
	}, v1.StatusAdminOpts{
		Components:  statusCompStore,
		Incidents:   incidentStore,
		Subscribers: subscriberStore,
		Maintenance: maintenanceStore,
		StatusSvc:   statusSvc,
		Broker:      statusBroker,
	})

	// --- License status endpoint ---
	router.RegisterLicenseRoutes(licenseMgr)

	// --- UI redesign endpoints ---
	uptimeDailyStore := sqlite.NewUptimeDailyStore(db)
	router.RegisterUIRoutes(v1.UIConfig{
		UptimeDaily:      uptimeDailyStore,
		LogStreamer:      rt,
		ContainerSvc:     svc,
		ResourceTopSvc:   resSvc,
		SparklineFetcher: epStore,
	})

	// --- Webhook dispatcher ---
	webhookDispatcher := webhook.NewDispatcher(webhookStore, notifier, logger)
	webhookObserverCh := make(chan v1.SSEEvent, 64)
	broker.AddObserver(webhookObserverCh)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case evt, ok := <-webhookObserverCh:
				if !ok {
					return
				}
				webhookDispatcher.HandleEvent(ctx, evt.Type, evt.Data)
			}
		}
	}()

	// --- Wire alert event forwarding from all monitoring services ---
	alertCh := alertEngine.EventChannel()

	// sendAlert forwards an alert event to the alert engine and status page auto-incidents.
	sendAlert := func(evt alert.Event) {
		alertCh <- evt
		statusSvc.HandleAlertEvent(ctx, evt)
	}

	// Container restart and health alerts
	svc.SetEventCallback(func(eventType string, data interface{}) {
		broker.Broadcast(v1.SSEEvent{Type: eventType, Data: data})

		// Propagate container state/health changes to the public status page.
		if eventType == "container.state_changed" || eventType == "container.health_changed" {
			if m, ok := data.(map[string]interface{}); ok {
				statusSvc.NotifyMonitorChanged(ctx, "container", toInt64(m["id"]))
			}
		}

		switch eventType {
		case "container.restart_alert":
			if ra, ok := data.(*alert.RestartAlert); ok && ra != nil {
				severity := alert.SeverityWarning
				if ra.RestartCount >= ra.Threshold*alert.CriticalRestartMultiplier {
					severity = alert.SeverityCritical
				}
				sendAlert(alert.Event{
					Source:     alert.SourceContainer,
					AlertType:  "restart_loop",
					Severity:   severity,
					Message:    fmt.Sprintf("Container %s exceeded restart threshold (%d/%d)", ra.ContainerName, ra.RestartCount, ra.Threshold),
					EntityType: "container",
					EntityID:   ra.ContainerID,
					EntityName: ra.ContainerName,
					Details: map[string]any{
						"restart_count": ra.RestartCount,
						"threshold":     ra.Threshold,
					},
					Timestamp: ra.Timestamp,
				})
			}
		case "container.restart_recovery":
			if m, ok := data.(map[string]interface{}); ok {
				sendAlert(alert.Event{
					Source:     alert.SourceContainer,
					AlertType:  "restart_loop",
					Severity:   alert.SeverityInfo,
					IsRecover:  true,
					Message:    fmt.Sprintf("Container %s restart rate returned to normal", toString(m["container_name"])),
					EntityType: "container",
					EntityID:   toInt64(m["container_id"]),
					EntityName: toString(m["container_name"]),
					Timestamp:  time.Now(),
				})
			}
		case "container.archived":
			if m, ok := data.(map[string]interface{}); ok {
				alertEngine.ResolveByEntity(ctx, "container", toInt64(m["id"]))
			}
		case "container.health_changed":
			m, ok := data.(map[string]interface{})
			if !ok {
				return
			}
			prev, _ := m["previous_health"].(*container.HealthStatus)
			newH, _ := m["health_status"].(container.HealthStatus)
			if prev != nil && *prev == container.HealthHealthy && newH == container.HealthUnhealthy {
				sendAlert(alert.Event{
					Source:     alert.SourceContainer,
					AlertType:  "health_unhealthy",
					Severity:   alert.SeverityWarning,
					Message:    "Container became unhealthy",
					EntityType: "container",
					EntityID:   toInt64(m["id"]),
					Details:    m,
					Timestamp:  time.Now(),
				})
			} else if prev != nil && *prev == container.HealthUnhealthy && newH == container.HealthHealthy {
				sendAlert(alert.Event{
					Source:     alert.SourceContainer,
					AlertType:  "health_unhealthy",
					Severity:   alert.SeverityInfo,
					IsRecover:  true,
					Message:    "Container recovered to healthy",
					EntityType: "container",
					EntityID:   toInt64(m["id"]),
					Details:    m,
					Timestamp:  time.Now(),
				})
			}
		}
	})

	// Endpoint alerts
	epSvc.SetAlertCallback(func(ep *endpoint.Endpoint, result endpoint.CheckResult) (string, interface{}) {
		// Propagate endpoint state changes to the public status page.
		statusSvc.NotifyMonitorChanged(ctx, "endpoint", ep.ID)

		a := alertDetector.EvaluateCheckResult(ep, result)
		if a == nil {
			return "", nil
		}
		if a.Type == "alert" {
			sendAlert(alert.Event{
				Source:     alert.SourceEndpoint,
				AlertType:  "consecutive_failure",
				Severity:   alert.SeverityCritical,
				Message:    fmt.Sprintf("Endpoint %s failed %d consecutive checks", a.Target, a.Failures),
				EntityType: "endpoint",
				EntityID:   a.EndpointID,
				EntityName: a.ContainerName,
				Details: map[string]any{
					"target":     a.Target,
					"failures":   a.Failures,
					"threshold":  a.Threshold,
					"last_error": a.LastError,
				},
				Timestamp: a.Timestamp,
			})
			return "endpoint.alert", map[string]interface{}{
				"endpoint_id":          a.EndpointID,
				"container_name":       a.ContainerName,
				"target":               a.Target,
				"consecutive_failures": a.Failures,
				"threshold":            a.Threshold,
				"last_error":           a.LastError,
				"timestamp":            a.Timestamp,
			}
		}
		sendAlert(alert.Event{
			Source:     alert.SourceEndpoint,
			AlertType:  "consecutive_failure",
			Severity:   alert.SeverityInfo,
			IsRecover:  true,
			Message:    fmt.Sprintf("Endpoint %s recovered after %d consecutive successes", a.Target, a.Successes),
			EntityType: "endpoint",
			EntityID:   a.EndpointID,
			EntityName: a.ContainerName,
			Details: map[string]any{
				"target":    a.Target,
				"successes": a.Successes,
				"threshold": a.Threshold,
			},
			Timestamp: a.Timestamp,
		})
		return "endpoint.recovery", map[string]interface{}{
			"endpoint_id":           a.EndpointID,
			"container_name":        a.ContainerName,
			"target":                a.Target,
			"consecutive_successes": a.Successes,
			"threshold":             a.Threshold,
			"timestamp":             a.Timestamp,
		}
	})

	// Heartbeat alerts
	hbSvc.SetAlertCallback(func(h *heartbeat.Heartbeat, alertType string, details map[string]interface{}) {
		// Propagate heartbeat state changes to the public status page.
		statusSvc.NotifyMonitorChanged(ctx, "heartbeat", h.ID)

		isRecover := alertType == "recovery"
		severity := alert.SeverityCritical
		msg := fmt.Sprintf("Heartbeat '%s' missed deadline", h.Name)
		if isRecover {
			severity = alert.SeverityInfo
			msg = fmt.Sprintf("Heartbeat '%s' recovered", h.Name)
		}
		hbAlertType := "deadline_missed"
		if t, ok := details["alert_type"].(string); ok {
			hbAlertType = t
		}
		sendAlert(alert.Event{
			Source:     alert.SourceHeartbeat,
			AlertType:  hbAlertType,
			Severity:   severity,
			IsRecover:  isRecover,
			Message:    msg,
			EntityType: "heartbeat",
			EntityID:   h.ID,
			EntityName: h.Name,
			Details:    details,
			Timestamp:  time.Now(),
		})
	})

	// Certificate alerts
	certSvc.SetEventCallback(func(eventType string, data interface{}) {
		broker.Broadcast(v1.SSEEvent{Type: eventType, Data: data})
		m, ok := data.(map[string]interface{})
		if !ok {
			return
		}

		// Propagate certificate state changes to the public status page.
		if eventType == "certificate.alert" || eventType == "certificate.recovery" {
			statusSvc.NotifyMonitorChanged(ctx, "certificate", toInt64(m["monitor_id"]))
		}

		switch eventType {
		case "certificate.alert":
			certAlertType, _ := m["alert_type"].(string)
			sendAlert(alert.Event{
				Source:     alert.SourceCertificate,
				AlertType:  certAlertType,
				Severity:   alert.SeverityCritical,
				Message:    fmt.Sprintf("Certificate alert (%s) for %v:%v", certAlertType, m["hostname"], m["port"]),
				EntityType: "certificate",
				EntityID:   toInt64(m["monitor_id"]),
				EntityName: toString(m["hostname"]),
				Details:    m,
				Timestamp:  time.Now(),
			})
		case "certificate.recovery":
			sendAlert(alert.Event{
				Source:     alert.SourceCertificate,
				AlertType:  "expiring",
				Severity:   alert.SeverityInfo,
				IsRecover:  true,
				Message:    fmt.Sprintf("Certificate renewed for %v", m["hostname"]),
				EntityType: "certificate",
				EntityID:   toInt64(m["monitor_id"]),
				EntityName: toString(m["hostname"]),
				Details:    m,
				Timestamp:  time.Now(),
			})
		}
	})

	// Resource alerts
	resSvc.SetEventCallback(func(eventType string, data interface{}) {
		broker.Broadcast(v1.SSEEvent{Type: eventType, Data: data})
		m, ok := data.(map[string]interface{})
		if !ok {
			return
		}
		switch eventType {
		case "resource.alert":
			resAlertType, _ := m["alert_type"].(string)
			sendAlert(alert.Event{
				Source:     alert.SourceResource,
				AlertType:  resAlertType + "_threshold",
				Severity:   alert.SeverityWarning,
				Message:    fmt.Sprintf("Resource %s threshold exceeded for container %v", resAlertType, m["container_name"]),
				EntityType: "container",
				EntityID:   toInt64(m["container_id"]),
				EntityName: toString(m["container_name"]),
				Details:    m,
				Timestamp:  time.Now(),
			})
		case "resource.recovery":
			recoveredType, _ := m["recovered_type"].(string)
			sendAlert(alert.Event{
				Source:     alert.SourceResource,
				AlertType:  recoveredType + "_threshold",
				Severity:   alert.SeverityInfo,
				IsRecover:  true,
				Message:    fmt.Sprintf("Resource usage returned to normal for container %v", m["container_name"]),
				EntityType: "container",
				EntityID:   toInt64(m["container_id"]),
				EntityName: toString(m["container_name"]),
				Details:    m,
				Timestamp:  time.Now(),
			})
		}
	})

	// Security insight alert and SSE callbacks
	securitySvc.SetAlertCallback(func(containerID int64, containerName string, insights []security.Insight, isRecover bool) {
		if isRecover {
			sendAlert(alert.Event{
				Source:     alert.SourceSecurity,
				AlertType:  alert.AlertTypeDangerousConfig,
				Severity:   alert.SeverityInfo,
				IsRecover:  true,
				Message:    fmt.Sprintf("All security issues resolved for container %s", containerName),
				EntityType: "container",
				EntityID:   containerID,
				EntityName: containerName,
				Details:    map[string]any{},
				Timestamp:  time.Now(),
			})
			return
		}
		hs := security.HighestSeverity(insights)
		sendAlert(alert.Event{
			Source:     alert.SourceSecurity,
			AlertType:  alert.AlertTypeDangerousConfig,
			Severity:   mapSecuritySeverity(hs),
			Message:    security.FormatAlertMessage(insights),
			EntityType: "container",
			EntityID:   containerID,
			EntityName: containerName,
			Details: map[string]any{
				"insight_count":    fmt.Sprintf("%d", len(insights)),
				"highest_severity": hs,
			},
			Timestamp: time.Now(),
		})
	})
	securitySvc.SetEventCallback(func(eventType string, data any) {
		broker.Broadcast(v1.SSEEvent{Type: eventType, Data: data})
	})

	// --- Rate limiter for public routes ---
	rl := ratelimit.New(10, 20) // 10 req/s per IP, burst 20
	go rl.Start(ctx)

	// --- Top-level mux combining admin router, public status page, and SPA ---
	topMux := http.NewServeMux()

	// MCP Streamable HTTP handler (mcpServer is assigned later, before srv.ListenAndServe)
	var mcpServer *gomcp.Server
	mcpEnabled := envOrDefault("MAINTENANT_MCP", "false")
	if mcpEnabled == "true" {
		mcpHTTPHandler := gomcp.NewStreamableHTTPHandler(func(_ *http.Request) *gomcp.Server {
			return mcpServer
		}, nil)
		var mcpHandler http.Handler = mcpHTTPHandler

		mcpClientID := os.Getenv("MAINTENANT_MCP_CLIENT_ID")
		mcpClientSecret := os.Getenv("MAINTENANT_MCP_CLIENT_SECRET")
		if mcpClientID != "" && mcpClientSecret != "" {
			// OAuth2 server for MCP authentication
			mcpOAuthStore := sqlite.NewMCPOAuthStore(db)
			oauthSrv := mcpoauth.NewOAuthServer(mcpoauth.Config{
				ClientID:     mcpClientID,
				ClientSecret: mcpClientSecret,
				IssuerURL:    baseURL,
			}, mcpOAuthStore, logger.With("component", "mcp-oauth"))

			// Register OAuth routes
			topMux.HandleFunc("/.well-known/oauth-authorization-server", oauthSrv.HandleAuthServerMetadata)
			topMux.HandleFunc("/oauth/authorize", oauthSrv.HandleAuthorize)
			topMux.HandleFunc("/oauth/token", oauthSrv.HandleToken)

			// Protected resource metadata via SDK handler
			topMux.Handle("/.well-known/oauth-protected-resource",
				mcpauth.ProtectedResourceMetadataHandler(&oauthex.ProtectedResourceMetadata{
					Resource:               baseURL + "/mcp",
					AuthorizationServers:   []string{baseURL},
					BearerMethodsSupported: []string{"header"},
					ResourceName:           "maintenant MCP",
				}))

			// SDK bearer token middleware with SQLite token verification
			resourceMetadataURL := baseURL + "/.well-known/oauth-protected-resource"
			tokenVerifier := mcpoauth.NewTokenVerifier(mcpOAuthStore)
			authMiddleware := mcpauth.RequireBearerToken(tokenVerifier, &mcpauth.RequireBearerTokenOptions{
				ResourceMetadataURL: resourceMetadataURL,
			})
			mcpHandler = authMiddleware(mcpHTTPHandler)

			// Start token cleanup goroutine
			go mcpoauth.StartCleanup(ctx, mcpOAuthStore, logger.With("component", "mcp-oauth-cleanup"))

			logger.Info("MCP server enabled with OAuth2 auth", "client_id", mcpClientID)
		} else {
			logger.Info("MCP server enabled without auth")
		}
		mcpHandler = rl.Middleware(mcpHandler)
		topMux.Handle("/mcp", mcpHandler)
		topMux.Handle("/mcp/", mcpHandler)
	}

	statusHandler.Register(topMux, rl.Middleware)

	topMux.Handle("/api/", router.Handler())
	topMux.Handle("/ping/", rl.Middleware(router.Handler()))
	topMux.Handle("/", spaHandler(router.Handler(), logger))

	srv := &http.Server{
		Addr:         addr,
		Handler:      withRequestTimeout(topMux, 10*time.Second),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 0, // Disabled globally: SSE streams require unbounded writes; non-SSE routes use per-request timeouts via withRequestTimeout.
		IdleTimeout:  120 * time.Second,
	}

	// --- Startup: Reconciliation ---
	logger.Info("running startup container reconciliation")
	if err := svc.Reconcile(ctx, rt); err != nil {
		logger.Error("startup reconciliation failed", "error", err)
		// Non-fatal: continue with existing state
	}

	// Prune orphan container alerts — resolve active alerts whose container no
	// longer exists (e.g. destroyed between maintenant restarts).
	if activeAlerts, err := alertStore.ListActiveAlerts(ctx); err == nil {
		for _, a := range activeAlerts {
			if a.EntityType != "container" {
				continue
			}
			c, err := svc.GetContainer(ctx, a.EntityID)
			if err != nil || c == nil {
				alertEngine.ResolveByEntity(ctx, "container", a.EntityID)
				logger.Info("pruned orphan container alert", "alert_id", a.ID, "entity_id", a.EntityID)
			}
		}
	}

	// Discover endpoint labels and security insights from all containers (Docker-specific)
	if dr, ok := rt.(*docker.Runtime); ok {
		logger.Info("syncing endpoint labels from discovered containers")
		if results, err := dr.DiscoverAllWithLabels(ctx); err == nil {
			// Build ExternalID → DB ID lookup from reconciled containers
			dbContainers, _ := svc.ListContainers(ctx, container.ListContainersOpts{IncludeIgnored: true})
			dbByExtID := make(map[string]*container.Container, len(dbContainers))
			for _, c := range dbContainers {
				dbByExtID[c.ExternalID] = c
			}

			now := time.Now()
			for _, r := range results {
				epSvc.SyncEndpoints(ctx, r.Container.Name, r.Container.ExternalID, r.Labels,
					r.Container.OrchestrationGroup, r.Container.OrchestrationUnit)
				certSvc.SyncFromLabels(ctx, r.Container.ExternalID, r.Labels)

				// Security insight analysis — resolve DB ID from reconciled containers
				dbC := dbByExtID[r.Container.ExternalID]
				if r.SecurityConfig != nil && dbC != nil && dbC.ID > 0 {
					bindings := make([]security.PortBinding, 0, len(r.SecurityConfig.PortBindings))
					for _, pb := range r.SecurityConfig.PortBindings {
						bindings = append(bindings, security.PortBinding{
							HostIP:   pb.HostIP,
							HostPort: pb.HostPort,
							Port:     pb.ContainerPort,
							Protocol: pb.Protocol,
						})
					}
					insights := security.AnalyzeDocker(dbC.ID, dbC.Name, security.DockerSecurityConfig{
						Privileged:  r.SecurityConfig.Privileged,
						NetworkMode: r.SecurityConfig.NetworkMode,
						Bindings:    bindings,
					}, now)
					securitySvc.UpdateContainer(dbC.ID, dbC.Name, insights)
				}
			}
			logger.Info("endpoint discovery complete", "active_checks", checkEngine.ActiveCount())
		} else {
			logger.Error("endpoint label discovery failed", "error", err)
		}
	}

	// --- Background: Event stream ---
	eventCh := rt.StreamEvents(ctx)
	go func() {
		for evt := range eventCh {
			svc.ProcessEvent(ctx, container.ContainerEvent{
				Action:       evt.Action,
				ExternalID:   evt.ExternalID,
				Name:         evt.Name,
				ExitCode:     evt.ExitCode,
				HealthStatus: evt.HealthStatus,
				ErrorDetail:  evt.ErrorDetail,
				Timestamp:    evt.Timestamp,
				Labels:       evt.Labels,
			})

			// Endpoint lifecycle events
			switch evt.Action {
			case "start":
				name := evt.Name
				if len(name) > 0 && name[0] == '/' {
					name = name[1:]
				}
				epSvc.HandleContainerStart(ctx, name, evt.ExternalID, evt.Labels,
					evt.Labels["com.docker.compose.project"],
					evt.Labels["com.docker.compose.service"])
				certSvc.SyncFromLabels(ctx, evt.ExternalID, evt.Labels)

				// Re-scan security insights for this container
				if dr, ok := rt.(*docker.Runtime); ok {
					go scanContainerSecurity(ctx, dr, svc, securitySvc, evt.ExternalID, logger)
				}
			case "stop", "die", "kill":
				epSvc.HandleContainerStop(ctx, evt.ExternalID)
			case "destroy":
				epSvc.HandleContainerDestroy(ctx, evt.ExternalID)
				certSvc.HandleContainerDestroy(ctx, evt.ExternalID)
			}
		}
	}()

	// --- Update intelligence ---
	registryClient := update.NewRegistryClient()
	updateScanner := update.NewScanner(registryClient, updateStore, logger)
	containerAdapter := update.NewContainerServiceAdapter(svc)
	updateSvc := update.NewService(updateStore, updateScanner, containerAdapter, logger)

	// Enrichment pipeline: CVE scanning, changelog resolution, risk scoring (Pro only)
	if extension.CurrentEdition() == extension.Enterprise {
		cveClient := update.NewCVEClient(updateStore, logger.With("component", "cve"))
		changelogResolver := update.NewChangelogResolver(registryClient, logger.With("component", "changelog"))
		riskEngine := update.NewRiskEngine()
		ecosystemResolver := update.NewEcosystemResolver(registryClient, logger.With("component", "ecosystem"))
		enricher := update.NewProEnricher(updateStore, cveClient, changelogResolver, riskEngine, ecosystemResolver, logger.With("component", "enricher"))
		updateSvc.SetEnricher(enricher)
		logger.Info("update enrichment pipeline enabled (Pro)")
	}

	updateSvc.SetEventCallback(func(eventType string, data interface{}) {
		broker.Broadcast(v1.SSEEvent{Type: eventType, Data: data})

		if eventType == event.UpdateDetected {
			if m, ok := data.(map[string]interface{}); ok {
				severity := alert.SeverityInfo
				if rs, ok := m["risk_score"].(int); ok {
					if rs >= 81 {
						severity = alert.SeverityCritical
					} else if rs >= 61 {
						severity = alert.SeverityWarning
					}
				}

				details := map[string]any{
					"image":       m["image"],
					"current_tag": m["current_tag"],
					"latest_tag":  m["latest_tag"],
					"update_type": m["update_type"],
				}

				if extension.CurrentEdition() == extension.Enterprise {
					if cmd, ok := m["update_command"]; ok {
						details["update_command"] = cmd
					}
					if cmd, ok := m["rollback_command"]; ok {
						details["rollback_command"] = cmd
					}
					if url, ok := m["changelog_url"]; ok {
						details["changelog_url"] = url
					}
					if bc, ok := m["has_breaking_changes"]; ok {
						details["has_breaking_changes"] = bc
					}
				}

				containerName, _ := m["container_name"].(string)
				latestTag, _ := m["latest_tag"].(string)
				sendAlert(alert.Event{
					Source:     "update",
					AlertType:  "update_available",
					Severity:   severity,
					Message:    fmt.Sprintf("Update available for %s: %s", containerName, latestTag),
					EntityType: "container",
					EntityName: containerName,
					Details:    details,
					Timestamp:  time.Now(),
				})
			}
		}
	})

	router.RegisterUpdateRoutes(updateSvc, updateStore, containerAdapter)
	router.RegisterSecurityRoutes(securitySvc, svc)

	// --- Security posture scoring (Enterprise) ---
	ackStore := sqlite.NewAcknowledgmentStore(db)
	certPostureAdapter := &certPostureReaderAdapter{certSvc: certSvc}
	cvePostureAdapter := &cvePostureReaderAdapter{store: updateStore}
	updatePostureAdapter := &updatePostureReaderAdapter{store: updateStore}
	scorer := security.NewScorer(certPostureAdapter, cvePostureAdapter, updatePostureAdapter, securitySvc, ackStore)

	// Posture threshold alerting
	if thresholdStr := os.Getenv("MAINTENANT_SECURITY_SCORE_THRESHOLD"); thresholdStr != "" {
		if threshold, err := strconv.Atoi(thresholdStr); err == nil && threshold > 0 {
			scorer.SetThreshold(threshold)
			logger.Info("security posture threshold configured", "threshold", threshold)
		}
	}
	scorer.SetPostureAlertCallback(func(score int, previousScore int, color string, isBreach bool) {
		severity := alert.SeverityWarning
		if score < scorer.Threshold()-20 {
			severity = alert.SeverityCritical
		}
		msg := fmt.Sprintf("Infrastructure security score dropped to %d (threshold: %d)", score, scorer.Threshold())
		if !isBreach {
			severity = alert.SeverityInfo
			msg = fmt.Sprintf("Infrastructure security score recovered to %d (threshold: %d)", score, scorer.Threshold())
		}
		sendAlert(alert.Event{
			Source:     alert.SourceSecurity,
			AlertType:  alert.AlertTypePostureThreshold,
			Severity:   severity,
			IsRecover:  !isBreach,
			Message:    msg,
			EntityType: "infrastructure",
			EntityID:   0,
			EntityName: "infrastructure",
			Details: map[string]any{
				"score":          score,
				"previous_score": previousScore,
				"color":          color,
			},
			Timestamp: time.Now(),
		})
	})
	scorer.SetPostureEventCallback(func(eventType string, data any) {
		broker.Broadcast(v1.SSEEvent{Type: eventType, Data: data})
	})

	router.RegisterPostureRoutes(scorer, svc, ackStore)

	go updateSvc.Start(ctx)

	// --- MCP Server ---
	mcpSvc := &pbmcp.Services{
		Containers:   svc,
		Endpoints:    epSvc,
		Heartbeats:   hbSvc,
		Certificates: certSvc,
		Resources:    resSvc,
		Alerts:       alertStore,
		Updates:      updateSvc,
		Incidents:    extension.NoopIncidentManager{},
		Maintenance:  extension.NoopMaintenanceScheduler{},
		Runtime:      rt,
		LogFetcher:   rt,
		Version:      version,
		Logger:       logger.With("component", "mcp"),
	}
	mcpServer = pbmcp.NewServer(mcpSvc)

	// MCP stdio mode: run MCP server over stdin/stdout, then exit.
	if mcpStdio {
		logger.Info("starting MCP server in stdio mode")
		if err := mcpServer.Run(ctx, &gomcp.StdioTransport{}); err != nil {
			logger.Error("MCP stdio server error", "error", err)
			os.Exit(1)
		}
		return
	}

	// --- Resource collector ---
	go resSvc.Start(ctx)

	// --- Certificate scheduler ---
	go certSvc.Start(ctx)

	// --- Maintenance scheduler ---
	go maintScheduler.Start(ctx)

	// --- Subscriber cleanup ---
	go subscriberSvc.Start(ctx)

	// --- Background: Retention cleanup ---
	sqlite.StartRetentionCleanupWithOpts(ctx, store, db, logger, sqlite.RetentionOpts{
		EndpointStore:    epStore,
		HeartbeatStore:   hbStore,
		CertificateStore: certStore,
		ResourceStore:    resStore,
	})

	// Alert retention cleanup (90 days)
	go func() {
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				before := time.Now().Add(-90 * 24 * time.Hour)
				deleted, err := alertStore.DeleteAlertsOlderThan(ctx, before)
				if err != nil {
					logger.Error("alert retention cleanup failed", "error", err)
				} else if deleted > 0 {
					logger.Info("alert retention cleanup", "deleted", deleted)
				}
			}
		}
	}()

	// Update intelligence retention cleanup (30 days)
	go func() {
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				before := time.Now().Add(-30 * 24 * time.Hour)
				deleted, err := updateStore.CleanupExpired(ctx, before)
				if err != nil {
					logger.Error("update retention cleanup failed", "error", err)
				} else if deleted > 0 {
					logger.Info("update retention cleanup", "deleted", deleted)
				}
			}
		}
	}()

	// --- Start HTTP server ---
	go func() {
		logger.Info("starting HTTP server", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("HTTP server error", "error", err)
			os.Exit(1)
		}
	}()

	// --- Wait for shutdown ---
	<-ctx.Done()
	logger.Info("shutting down maintenant")

	// Stop endpoint check engine
	epSvc.Stop()
	logger.Info("endpoint check engine stopped")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("HTTP server shutdown error", "error", err)
	}

	logger.Info("maintenant stopped")
}

func envOrDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func toInt64(v interface{}) int64 {
	switch n := v.(type) {
	case int64:
		return n
	case int:
		return int64(n)
	case float64:
		return int64(n)
	default:
		return 0
	}
}

func toString(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}

// scanContainerSecurity inspects a single container and updates its security insights.
func scanContainerSecurity(ctx context.Context, dr *docker.Runtime, containerSvc *container.Service, secSvc *security.Service, externalID string, logger *slog.Logger) {
	results, err := dr.DiscoverAllWithLabels(ctx)
	if err != nil {
		logger.Warn("security: failed to scan container", "external_id", externalID, "error", err)
		return
	}

	now := time.Now()
	for _, r := range results {
		if r.Container.ExternalID != externalID || r.SecurityConfig == nil {
			continue
		}
		c, err := containerSvc.GetContainer(ctx, r.Container.ID)
		if err != nil || c == nil {
			// Container may not be in DB yet after reconciliation; try by external ID
			stored, _ := containerSvc.ListContainers(ctx, container.ListContainersOpts{IncludeIgnored: true})
			for _, sc := range stored {
				if sc.ExternalID == externalID {
					c = sc
					break
				}
			}
		}
		if c == nil {
			return
		}

		bindings := make([]security.PortBinding, 0, len(r.SecurityConfig.PortBindings))
		for _, pb := range r.SecurityConfig.PortBindings {
			bindings = append(bindings, security.PortBinding{
				HostIP:   pb.HostIP,
				HostPort: pb.HostPort,
				Port:     pb.ContainerPort,
				Protocol: pb.Protocol,
			})
		}
		insights := security.AnalyzeDocker(c.ID, c.Name, security.DockerSecurityConfig{
			Privileged:  r.SecurityConfig.Privileged,
			NetworkMode: r.SecurityConfig.NetworkMode,
			Bindings:    bindings,
		}, now)
		secSvc.UpdateContainer(c.ID, c.Name, insights)
		return
	}
}

// --- Posture reader adapters ---

// certPostureReaderAdapter adapts the certificate service for posture scoring.
type certPostureReaderAdapter struct {
	certSvc *certificate.Service
}

func (a *certPostureReaderAdapter) ListCertificatesForContainer(ctx context.Context, containerExternalID string) ([]security.CertificateInfo, error) {
	monitors, err := a.certSvc.ListMonitors(ctx, certificate.ListCertificatesOpts{})
	if err != nil {
		return nil, fmt.Errorf("list cert monitors: %w", err)
	}

	var result []security.CertificateInfo
	for _, m := range monitors {
		if !m.Active || m.ExternalID != containerExternalID {
			continue
		}
		info := security.CertificateInfo{
			Status: string(m.Status),
		}
		cr, err := a.certSvc.GetLatestCheckResult(ctx, m.ID)
		if err == nil && cr != nil {
			info.DaysRemaining = cr.DaysRemaining()
		}
		result = append(result, info)
	}
	return result, nil
}

// cvePostureReaderAdapter adapts the update store for CVE scoring.
type cvePostureReaderAdapter struct {
	store update.UpdateStore
}

func (a *cvePostureReaderAdapter) ListCVEsForContainer(ctx context.Context, containerExternalID string) ([]security.CVEInfo, error) {
	cves, err := a.store.ListContainerCVEs(ctx, containerExternalID)
	if err != nil {
		return nil, fmt.Errorf("list container cves: %w", err)
	}
	result := make([]security.CVEInfo, len(cves))
	for i, c := range cves {
		result[i] = security.CVEInfo{
			CVEID:    c.CVEID,
			Severity: string(c.Severity),
		}
	}
	return result, nil
}

// updatePostureReaderAdapter adapts the update store for update/image-age scoring.
type updatePostureReaderAdapter struct {
	store update.UpdateStore
}

func (a *updatePostureReaderAdapter) ListUpdatesForContainer(ctx context.Context, containerExternalID string) ([]security.UpdateInfo, error) {
	updates, err := a.store.ListImageUpdates(ctx, update.ListImageUpdatesOpts{})
	if err != nil {
		return nil, fmt.Errorf("list image updates: %w", err)
	}
	var result []security.UpdateInfo
	for _, u := range updates {
		if u.ContainerID != containerExternalID {
			continue
		}
		result = append(result, security.UpdateInfo{
			UpdateType:  string(u.UpdateType),
			PublishedAt: u.PublishedAt,
		})
	}
	return result, nil
}

// mapSecuritySeverity maps security insight severity to alert severity.
func mapSecuritySeverity(s string) string {
	switch s {
	case security.SeverityCritical:
		return alert.SeverityCritical
	case security.SeverityHigh:
		return alert.SeverityWarning
	case security.SeverityMedium:
		return alert.SeverityInfo
	default:
		return alert.SeverityInfo
	}
}

// spaHandler returns an http.Handler that serves the embedded SPA frontend.
// API and ping routes are delegated to the API handler; everything else is
// served from the embedded filesystem, with a fallback to index.html for
// client-side routing.
func spaHandler(apiHandler http.Handler, logger *slog.Logger) http.Handler {
	distFS, err := fs.Sub(web.FS, "dist")
	if err != nil {
		logger.Warn("SPA assets not embedded, frontend will not be served")
		return apiHandler
	}

	fileServer := http.FileServer(http.FS(distFS))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		// API and ping routes → delegate to API router
		if strings.HasPrefix(path, "/api/") || strings.HasPrefix(path, "/ping/") {
			apiHandler.ServeHTTP(w, r)
			return
		}

		// Try to serve static file
		f, err := fs.Stat(distFS, strings.TrimPrefix(path, "/"))
		if err == nil && !f.IsDir() {
			// Immutable cache for hashed assets
			if strings.HasPrefix(path, "/assets/") {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			}
			fileServer.ServeHTTP(w, r)
			return
		}

		// SPA fallback: serve index.html for client-side routing
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	})
}

// isStreamingPath reports whether path corresponds to an SSE or streaming endpoint.
func isStreamingPath(path string) bool {
	// Exact matches first.
	if path == "/api/v1/containers/events" || path == "/status/events" {
		return true
	}
	// MCP Streamable HTTP uses SSE for server-to-client messages.
	if path == "/mcp" || strings.HasPrefix(path, "/mcp/") {
		return true
	}
	// Log streaming: /api/v1/containers/{id}/logs/stream
	if strings.HasPrefix(path, "/api/v1/containers/") && strings.HasSuffix(path, "/logs/stream") {
		return true
	}
	return false
}

// withRequestTimeout wraps non-streaming handlers with http.TimeoutHandler so
// that ordinary REST requests are bounded even though the server-level
// WriteTimeout is disabled (required for SSE).
func withRequestTimeout(h http.Handler, timeout time.Duration) http.Handler {
	wrapped := http.TimeoutHandler(h, timeout, "request timeout")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isStreamingPath(r.URL.Path) {
			h.ServeHTTP(w, r)
			return
		}
		wrapped.ServeHTTP(w, r)
	})
}
