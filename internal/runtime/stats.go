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

package runtime

import "time"

// RawStats is a runtime-agnostic resource snapshot.
// Fields set to -1 indicate the metric is unavailable for this runtime.
type RawStats struct {
	CPUPercent      float64
	MemUsed         int64
	MemLimit        int64
	NetRxBytes      int64 // -1 if unavailable
	NetTxBytes      int64 // -1 if unavailable
	BlockReadBytes  int64 // -1 if unavailable
	BlockWriteBytes int64 // -1 if unavailable
	Timestamp       time.Time
}
