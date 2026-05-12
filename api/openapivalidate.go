package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/International-Combat-Archery-Alliance/middleware"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	nethttpMiddleware "github.com/oapi-codegen/nethttp-middleware"
)

func (a *API) openapiValidateMiddleware(swagger *openapi3.T) middleware.MiddlewareFunc {
	return nethttpMiddleware.OapiRequestValidatorWithOptions(swagger, &nethttpMiddleware.Options{
		Options: openapi3filter.Options{
			AuthenticationFunc: func(ctx context.Context, ai *openapi3filter.AuthenticationInput) error {
				return nil
			},
		},
		ErrorHandlerWithOpts: func(ctx context.Context, err error, w http.ResponseWriter, r *http.Request, opts nethttpMiddleware.ErrorHandlerOpts) {
			logger := a.getLoggerOrBaseLogger(ctx)

			var e Error

			var requestErr *openapi3filter.RequestError
			var secErr *openapi3filter.SecurityRequirementsError
			if errors.As(err, &requestErr) {
				e = Error{
					Message: err.Error(),
					Code:    InputValidationError,
				}
			} else if errors.As(err, &secErr) {
				e = Error{
					Message: err.Error(),
					Code:    InputValidationError,
				}
			} else {
				e = Error{
					Message: err.Error(),
					Code:    InternalError,
				}
			}
			jsonBody, err := json.Marshal(&e)
			if err != nil {
				logger.Error("failed to marshal input validation error resp", "error", err)
				jsonBody = []byte("{\"message\": \"input is invalid\", \"code\": \"InputValidationError\"}")
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(opts.StatusCode)
			w.Write(jsonBody)
		},
	})
}
