package main

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/International-Combat-Archery-Alliance/mailing-list-api/api"
	"github.com/International-Combat-Archery-Alliance/telemetry"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"go.opentelemetry.io/otel/codes"
)

const (
	newRelicLicenseEnvVar  = "NEW_RELIC_LICENSE_KEY"
	newRelicLicenseSSMPath = "/newrelic-license-key"

	mailerLiteAPIKeySSMPath     = "/mailerlite-api-key"
	cfTurnstileSecretKeySSMPath = "/cfTurnstileSecretKey"

	mailerLiteAPIKeyEnvVar     = "MAILERLITE_API_KEY"
	cfTurnstileSecretKeyEnvVar = "CF_TURNSTILE_SECRET_KEY"
)

var (
	awsCfg     aws.Config
	awsCfgErr  error
	awsCfgOnce sync.Once
)

func loadAWSConfig(ctx context.Context) (aws.Config, error) {
	awsCfgOnce.Do(func() {
		ctx, span := tracer.Start(ctx, "load-aws-config")
		defer span.End()

		cfg, err := config.LoadDefaultConfig(ctx)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			awsCfgErr = fmt.Errorf("unable to load AWS SDK config: %w", err)
			return
		}
		telemetry.InstrumentAWSConfig(&cfg)
		awsCfg = cfg
	})
	return awsCfg, awsCfgErr
}

func getSSMParameter(ctx context.Context, name string) (string, error) {
	cfg, err := loadAWSConfig(ctx)
	if err != nil {
		return "", err
	}

	client := ssm.NewFromConfig(cfg)
	result, err := client.GetParameter(ctx, &ssm.GetParameterInput{
		Name:           aws.String(name),
		WithDecryption: aws.Bool(true),
	})
	if err != nil {
		return "", fmt.Errorf("failed to get parameter %q: %w", name, err)
	}

	return aws.ToString(result.Parameter.Value), nil
}

type AppConfig struct {
	MailerLiteAPIKey    string
	TurnstileSecretKey  string
}

func fetchAppConfig(ctx context.Context, env api.Environment) (AppConfig, error) {
	if env == api.LOCAL {
		return AppConfig{
			MailerLiteAPIKey:   getEnvOrDefault(mailerLiteAPIKeyEnvVar, ""),
			TurnstileSecretKey: getEnvOrDefault(cfTurnstileSecretKeyEnvVar, "1x0000000000000000000000000000000AA"),
		}, nil
	}

	mailerLiteAPIKey, err := getSSMParameter(ctx, mailerLiteAPIKeySSMPath)
	if err != nil {
		return AppConfig{}, fmt.Errorf("failed to get mailerlite API key: %w", err)
	}

	turnstileSecretKey, err := getSSMParameter(ctx, cfTurnstileSecretKeySSMPath)
	if err != nil {
		return AppConfig{}, fmt.Errorf("failed to get turnstile secret key: %w", err)
	}

	return AppConfig{
		MailerLiteAPIKey:   mailerLiteAPIKey,
		TurnstileSecretKey: turnstileSecretKey,
	}, nil
}

func getNewRelicLicenseKey(ctx context.Context, env api.Environment) (string, error) {
	if env == api.LOCAL {
		return os.Getenv(newRelicLicenseEnvVar), nil
	}
	return getSSMParameter(ctx, newRelicLicenseSSMPath)
}

func getAPIEnvironment() api.Environment {
	if isLocal() {
		return api.LOCAL
	}
	return api.PROD
}

func isLocal() bool {
	return getEnvOrDefault("AWS_SAM_LOCAL", "false") == "true"
}

func getEnvOrDefault(key, defaultVal string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return defaultVal
}
