package certificate

import (
	"context"
	"time"
)

// CertificateStore defines the persistence interface for certificate monitoring data.
type CertificateStore interface {
	// Monitor CRUD
	CreateMonitor(ctx context.Context, m *CertMonitor) (int64, error)
	GetMonitorByID(ctx context.Context, id int64) (*CertMonitor, error)
	GetMonitorByHostPort(ctx context.Context, hostname string, port int) (*CertMonitor, error)
	GetMonitorByEndpointID(ctx context.Context, endpointID int64) (*CertMonitor, error)
	ListMonitors(ctx context.Context, opts ListCertificatesOpts) ([]*CertMonitor, error)
	UpdateMonitor(ctx context.Context, m *CertMonitor) error
	SoftDeleteMonitor(ctx context.Context, id int64) error

	// Check results
	InsertCheckResult(ctx context.Context, result *CertCheckResult) (int64, error)
	GetLatestCheckResult(ctx context.Context, monitorID int64) (*CertCheckResult, error)
	ListCheckResults(ctx context.Context, monitorID int64, opts ListChecksOpts) ([]*CertCheckResult, int, error)

	// Chain entries
	InsertChainEntries(ctx context.Context, entries []*CertChainEntry) error
	GetChainEntries(ctx context.Context, checkResultID int64) ([]*CertChainEntry, error)

	// Label-discovered monitors
	ListMonitorsByExternalID(ctx context.Context, externalID string) ([]*CertMonitor, error)
	DeactivateMonitor(ctx context.Context, id int64) error

	// Scheduler
	ListDueScheduledMonitors(ctx context.Context, now time.Time) ([]*CertMonitor, error)

	// Retention
	DeleteCheckResultsBefore(ctx context.Context, before time.Time, batchSize int) (int64, error)
}
