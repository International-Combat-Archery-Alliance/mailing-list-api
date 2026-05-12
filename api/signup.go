package api

import (
	"context"
	"errors"
	"log/slog"

	"github.com/International-Combat-Archery-Alliance/email"
	"github.com/International-Combat-Archery-Alliance/mailing-list-api/ptr"
)

func (a *API) SignupForMailingList(ctx context.Context, request SignupForMailingListRequestObject) (SignupForMailingListResponseObject, error) {
	ctx, span := a.tracer.Start(ctx, "SignupForMailingList")
	defer span.End()

	logger := a.getLoggerOrBaseLogger(ctx)

	validatedData, err := a.captchaValidator.Validate(ctx, request.Params.CfTurnstileResponse, "")
	if err != nil {
		span.RecordError(err)
		logger.Warn("Invalid captcha", slog.String("error", err.Error()))
		return SignupForMailingList400JSONResponse{
			Code:    CaptchaInvalid,
			Message: "Invalid captcha",
		}, nil
	}

	if a.env == PROD && validatedData.Hostname() != "icaa.world" {
		logger.Warn("Invalid captcha hostname", slog.String("givenHostname", validatedData.Hostname()))
		return SignupForMailingList400JSONResponse{
			Code:    CaptchaInvalid,
			Message: "Invalid hostname, must come from icaa.world",
		}, nil
	}

	var name string
	if request.Body.Name != nil {
		name = *request.Body.Name
	}

	err = a.subscriberManager.AddSubscriberToGroup(ctx, string(request.Body.Email), name, a.groupID)
	if err != nil {
		span.RecordError(err)
		logger.Error("failed to add subscriber to group", slog.String("error", err.Error()))

		var emailErr *email.Error
		if errors.As(err, &emailErr) {
			switch emailErr.Reason {
			case email.REASON_INVALID_EMAIL:
				return SignupForMailingList422JSONResponse{
					Code:    InvalidEmail,
					Message: "Invalid email address",
				}, nil
			case email.REASON_RATE_LIMITED:
				return SignupForMailingList429JSONResponse{
					Code:    RateLimited,
					Message: "Too many requests, please try again later",
				}, nil
			case email.REASON_SERVICE_ERROR:
				return SignupForMailingList500JSONResponse{
					Code:    ServiceError,
					Message: "Mailing list service is temporarily unavailable",
				}, nil
			}
		}

		return SignupForMailingList500JSONResponse{
			Code:    InternalError,
			Message: "An unexpected error occurred",
		}, nil
	}

	logger.Info("subscriber added to mailing list", slog.String("email", string(request.Body.Email)))

	return SignupForMailingList200JSONResponse{
		Message: ptr.String("Successfully signed up"),
	}, nil
}
