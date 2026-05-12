# ICAA Mailing List API

An API for managing mailing list signups for the ICAA website. Built with Go and deployed as an AWS Lambda function using AWS SAM.

## Tech Stack

- **Language**: Go 1.25+
- **Infrastructure**: AWS Lambda, API Gateway
- **Deployment**: AWS SAM (Serverless Application Model)
- **API Spec**: OpenAPI 3.0 with code generation via [oapi-codegen](https://github.com/oapi-codegen/oapi-codegen)
- **Email Provider**: MailerLite
- **CAPTCHA**: Cloudflare Turnstile
- **Observability**: OpenTelemetry tracing, structured JSON logging

## Code Structure

The project follows a Hexagonal Architecture (Ports and Adapters) pattern:

- `api/` - HTTP handlers, OpenAPI validation middleware, and generated code
- `cmd/` - Application entry point, dependency wiring
- `ptr/` - Pointer utility functions
- `spec/` - OpenAPI 3.0 specification (`api.yaml`)

## API Endpoints

| Endpoint | Auth | Description |
|---|---|---|
| `POST /mailing-list/signup` | Public (CAPTCHA) | Sign up for the ICAA mailing list |

### Request

```json
POST /mailing-list/signup
Headers:
  cf-turnstile-response: <turnstile-token>
Body:
{
  "email": "user@example.com",
  "name": "Optional Name"
}
```

### Response

```json
200 OK:
{
  "message": "Successfully signed up"
}
```

## Prerequisites

- Go 1.25+
- [AWS SAM CLI](https://docs.aws.amazon.com/serverless-application-model/latest/developerguide/install-sam-cli.html)
- Docker
- AWS CLI (configured with appropriate credentials for deployment)

## Local Development

1. **Create an `env.json` file** for local environment variables:

   ```json
   {
     "ICAAMailingList": {
       "OTEL_EXPORTER_OTLP_ENDPOINT": "jaeger:4317",
       "MAILERLITE_API_KEY": "",
       "CF_TURNSTILE_SECRET_KEY": "1x0000000000000000000000000000000AA"
     }
   }
   ```

2. **Start shared infrastructure**:
   Shared infrastructure (DynamoDB, Jaeger) is managed in `icaa.world/docker-compose.yml`.
   ```bash
   cd ../icaa.world && docker compose up -d
   ```

3. **Build and run the API locally**:

   ```bash
   make local
   ```

   This will:
   - Generate API code from the OpenAPI spec
   - Build the SAM application
   - Start the local API server with hot reloading

The local API will be available at `http://localhost:3005`.

## Building

```bash
make build
```

This generates the API code from the OpenAPI spec and builds the SAM application.

## Deployment

The API is deployed via AWS SAM. The CI/CD pipeline is configured in `.github/workflows/go.yml`.

For manual deployment:

```bash
sam deploy --guided
```

## Configuration

| Environment Variable | Description | Default |
|---|---|---|
| `MAILERLITE_API_KEY` | MailerLite API key | `""` |
| `CF_TURNSTILE_SECRET_KEY` | Cloudflare Turnstile secret key | `1x0000000000000000000000000000000AA` |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | OpenTelemetry collector endpoint | `""` |
| `HOST` | Server host | `0.0.0.0` |
| `PORT` | Server port | `8000` |

### SSM Parameters (Production)

| Parameter | Description |
|---|---|
| `/mailerlite-api-key` | MailerLite API key |
| `/cfTurnstileSecretKey` | Cloudflare Turnstile secret key |
| `/newrelic-license-key` | New Relic license key |

## License

See [LICENSE](LICENSE) for details.
