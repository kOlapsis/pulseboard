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

package runtime

// HealthInfo holds runtime-agnostic health check information.
type HealthInfo struct {
	HasHealthCheck bool
	Status         string // "healthy", "unhealthy", "starting", "none"
	FailingStreak  int
	LastOutput     string
}
