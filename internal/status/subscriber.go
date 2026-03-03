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

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"time"
)

// SubscriberService manages email subscriptions for status updates.
type SubscriberService struct {
	store  SubscriberStore
	smtp   *SmtpClient
	logger *slog.Logger

	baseURL string
}

// NewSubscriberService creates a new subscriber service.
func NewSubscriberService(store SubscriberStore, smtp *SmtpClient, baseURL string, logger *slog.Logger) *SubscriberService {
	return &SubscriberService{
		store:   store,
		smtp:    smtp,
		logger:  logger,
		baseURL: baseURL,
	}
}

func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// Subscribe creates a new subscriber with double opt-in.
func (s *SubscriberService) Subscribe(ctx context.Context, email string) error {
	confirmToken, err := generateToken()
	if err != nil {
		return fmt.Errorf("generate confirm token: %w", err)
	}
	unsubToken, err := generateToken()
	if err != nil {
		return fmt.Errorf("generate unsub token: %w", err)
	}

	expires := time.Now().Add(24 * time.Hour)
	sub := &StatusSubscriber{
		Email:          email,
		Confirmed:      false,
		ConfirmToken:   &confirmToken,
		ConfirmExpires: &expires,
		UnsubToken:     unsubToken,
	}

	if _, err := s.store.CreateSubscriber(ctx, sub); err != nil {
		return fmt.Errorf("create subscriber: %w", err)
	}

	if s.smtp != nil {
		confirmURL := fmt.Sprintf("%s/status/confirm?token=%s", s.baseURL, confirmToken)
		body := fmt.Sprintf(`<html><body>
<h2>Confirm your subscription</h2>
<p>Click the link below to confirm your status page subscription:</p>
<p><a href="%s">Confirm Subscription</a></p>
<p>This link expires in 24 hours.</p>
</body></html>`, confirmURL)
		if err := s.smtp.Send(email, "Confirm your status page subscription", body); err != nil {
			s.logger.Error("failed to send confirmation email", "error", err, "email", email)
		}
	}

	return nil
}

// Confirm validates a confirmation token and activates the subscription.
func (s *SubscriberService) Confirm(ctx context.Context, token string) error {
	sub, err := s.store.GetSubscriberByToken(ctx, token)
	if err != nil || sub == nil {
		return fmt.Errorf("invalid or expired token")
	}
	if sub.ConfirmExpires != nil && sub.ConfirmExpires.Before(time.Now()) {
		return fmt.Errorf("confirmation token expired")
	}
	return s.store.ConfirmSubscriber(ctx, sub.ID)
}

// Unsubscribe removes a subscriber by their unsubscribe token.
func (s *SubscriberService) Unsubscribe(ctx context.Context, token string) error {
	sub, err := s.store.GetSubscriberByUnsubToken(ctx, token)
	if err != nil || sub == nil {
		return fmt.Errorf("invalid unsubscribe token")
	}
	return s.store.DeleteSubscriber(ctx, sub.ID)
}

// NotifyAll sends an incident notification to all confirmed subscribers.
func (s *SubscriberService) NotifyAll(ctx context.Context, subject, message string) {
	if s.smtp == nil {
		return
	}

	subs, err := s.store.ListConfirmedSubscribers(ctx)
	if err != nil {
		s.logger.Error("failed to list subscribers for notification", "error", err)
		return
	}

	for _, sub := range subs {
		unsubURL := fmt.Sprintf("%s/status/unsubscribe?token=%s", s.baseURL, sub.UnsubToken)
		body := fmt.Sprintf(`<html><body>
<h2>Status Update</h2>
<p>%s</p>
<hr>
<p><small><a href="%s">Unsubscribe</a></small></p>
</body></html>`, message, unsubURL)

		if err := s.smtp.Send(sub.Email, subject, body); err != nil {
			s.logger.Error("failed to send notification", "error", err, "email", sub.Email)
		}
	}
}

// CleanExpired removes unconfirmed subscribers older than 24 hours.
func (s *SubscriberService) CleanExpired(ctx context.Context) (int64, error) {
	return s.store.CleanExpiredUnconfirmed(ctx)
}

// StartCleanupTicker runs periodic cleanup of expired unconfirmed subscribers.
func (s *SubscriberService) StartCleanupTicker(ctx context.Context) {
	ticker := time.NewTicker(24 * time.Hour)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				deleted, err := s.CleanExpired(ctx)
				if err != nil {
					s.logger.Error("subscriber cleanup failed", "error", err)
				} else if deleted > 0 {
					s.logger.Info("cleaned expired subscribers", "deleted", deleted)
				}
			}
		}
	}()
}
