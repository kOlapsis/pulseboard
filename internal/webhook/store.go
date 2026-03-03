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

package webhook

import "context"

// WebhookSubscriptionStore defines the persistence interface for webhook subscriptions.
type WebhookSubscriptionStore interface {
	List(ctx context.Context) ([]*WebhookSubscription, error)
	GetByID(ctx context.Context, id string) (*WebhookSubscription, error)
	Create(ctx context.Context, sub *WebhookSubscription) error
	Delete(ctx context.Context, id string) error
	UpdateDeliveryStatus(ctx context.Context, id string, status string, failureCount int) error
	ListActive(ctx context.Context) ([]*WebhookSubscription, error)
}
