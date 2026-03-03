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

package status

import "context"

// ComponentStore defines the persistence interface for component groups and status components.
type ComponentStore interface {
	// Groups
	ListGroups(ctx context.Context) ([]ComponentGroup, error)
	GetGroup(ctx context.Context, id int64) (*ComponentGroup, error)
	CreateGroup(ctx context.Context, g *ComponentGroup) (int64, error)
	UpdateGroup(ctx context.Context, g *ComponentGroup) error
	DeleteGroup(ctx context.Context, id int64) error

	// Components
	ListComponents(ctx context.Context) ([]StatusComponent, error)
	ListVisibleComponents(ctx context.Context) ([]StatusComponent, error)
	GetComponent(ctx context.Context, id int64) (*StatusComponent, error)
	GetComponentByMonitor(ctx context.Context, monitorType string, monitorID int64) (*StatusComponent, error)
	ListGlobalComponents(ctx context.Context, monitorType string) ([]StatusComponent, error)
	CreateComponent(ctx context.Context, c *StatusComponent) (int64, error)
	UpdateComponent(ctx context.Context, c *StatusComponent) error
	DeleteComponent(ctx context.Context, id int64) error
}
