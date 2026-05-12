package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/International-Combat-Archery-Alliance/captcha/cfturnstile"
	"github.com/International-Combat-Archery-Alliance/email"
	"github.com/International-Combat-Archery-Alliance/email/mailerlite"
	"github.com/International-Combat-Archery-Alliance/mailing-list-api/api"
)

func createSubscriberManager(logger *slog.Logger, envEnvironment api.Environment, mailerLiteAPIKey string) email.SubscriberManager {
	if envEnvironment == api.LOCAL {
		return &subscriberLogger{logger: logger}
	}
	return mailerlite.NewMailerLiteManager(mailerLiteAPIKey)
}

var _ email.SubscriberManager = &subscriberLogger{}

type subscriberLogger struct {
	logger *slog.Logger
}

func (s *subscriberLogger) CreateGroup(ctx context.Context, name string) (string, error) {
	s.logger.Info("mailerlite group that would be created", slog.String("name", name))
	return "local-group-id", nil
}

func (s *subscriberLogger) FindOrCreateGroup(ctx context.Context, name string) (string, error) {
	s.logger.Info("mailerlite group that would be found or created", slog.String("name", name))
	return "local-group-id", nil
}

func (s *subscriberLogger) AddSubscriberToGroup(ctx context.Context, email, name, groupID string) error {
	s.logger.Info("mailerlite subscriber that would be added", slog.String("email", email), slog.String("name", name), slog.String("groupID", groupID))
	return nil
}

func createTurnstileValidator(httpClient *http.Client, secretKey string) (*cfturnstile.Validator, error) {
	if httpClient == nil {
		return nil, fmt.Errorf("http client is required")
	}
	return cfturnstile.NewValidator(httpClient, secretKey), nil
}
