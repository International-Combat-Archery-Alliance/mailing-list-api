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

	mailerLiteAPIKeySSMPath     = "/mailerLiteApiKey"
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

func getSSMParameters(ctx context.Context, names []string) (map[string]string, error) {
	cfg, err := loadAWSConfig(ctx)
	if err != nil {
		return nil, err
	}

	client := ssm.NewFromConfig(cfg)
	result, err := client.GetParameters(ctx, &ssm.GetParametersInput{
		Names:          names,
		WithDecryption: aws.Bool(true),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get parameters: %w", err)
	}

	params := make(map[string]string, len(result.Parameters))
	for _, p := range result.Parameters {
		params[aws.ToString(p.Name)] = aws.ToString(p.Value)
	}

	if len(result.InvalidParameters) > 0 {
		return params, fmt.Errorf("invalid parameters: %v", result.InvalidParameters)
	}

	return params, nil
}

type AppConfig struct {
	MailerLiteAPIKey   string
	TurnstileSecretKey string
}

func fetchAppConfig(env api.Environment, ssmParams map[string]string) (AppConfig, error) {
	if env == api.LOCAL {
		return AppConfig{
			MailerLiteAPIKey:   getEnvOrDefault(mailerLiteAPIKeyEnvVar, ""),
			TurnstileSecretKey: getEnvOrDefault(cfTurnstileSecretKeyEnvVar, "1x0000000000000000000000000000000AA"),
		}, nil
	}

	mailerLiteAPIKey, ok := ssmParams[mailerLiteAPIKeySSMPath]
	if !ok {
		return AppConfig{}, fmt.Errorf("missing SSM parameter %q", mailerLiteAPIKeySSMPath)
	}

	turnstileSecretKey, ok := ssmParams[cfTurnstileSecretKeySSMPath]
	if !ok {
		return AppConfig{}, fmt.Errorf("missing SSM parameter %q", cfTurnstileSecretKeySSMPath)
	}

	return AppConfig{
		MailerLiteAPIKey:   mailerLiteAPIKey,
		TurnstileSecretKey: turnstileSecretKey,
	}, nil
}

func getNewRelicLicenseKey(env api.Environment, ssmParams map[string]string) (string, error) {
	if env == api.LOCAL {
		return os.Getenv(newRelicLicenseEnvVar), nil
	}

	key, ok := ssmParams[newRelicLicenseSSMPath]
	if !ok {
		return "", fmt.Errorf("missing SSM parameter %q", newRelicLicenseSSMPath)
	}
	return key, nil
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
