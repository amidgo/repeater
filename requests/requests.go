package requests

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/amidgo/retry"
	"github.com/google/uuid"
)

type (
	CreatedRequest struct {
		RequestID uuid.UUID
		Content   []byte
		CreatedAt time.Time
	}

	Request struct {
		RequestID uuid.UUID
		Content   []byte
	}

	AbortedRequest struct {
		RequestID uuid.UUID
		AbortedAt time.Time
		Err       error
	}

	CompletedRequest struct {
		RequestID   uuid.UUID
		CompletedAt time.Time
	}

	Attempt struct {
		RequestID uuid.UUID
		Err       error
		CreatedAt time.Time
	}

	Pagination struct {
		Offset uint64
		Limit  uint64
	}

	Config struct {
		WorkerCount int
	}

	Storage interface {
		CreateRequest(ctx context.Context, createdRequest CreatedRequest) error

		GetPendingRequests(ctx context.Context, pagination Pagination) ([]Request, error)

		LastRequestAttemptNumber(ctx context.Context, requestID uuid.UUID) (uint64, error)
		InsertRequestAttempt(ctx context.Context, attempt Attempt) (uint64, error)

		MarkRequestAsAborted(ctx context.Context, abortedRequest AbortedRequest) error
		MarkRequestAsCompleted(ctx context.Context, completedRequest CompletedRequest) error
	}
)

type retryOptions struct {
	logger      *slog.Logger
	maxAttempts *uint64
}

func makeRetryOptions(opts []Option) retryOptions {
	options := retryOptions{}

	for _, op := range opts {
		op(&options)
	}

	return options
}

type Option func(opts *retryOptions)

func WithLogger(logger *slog.Logger) Option {
	return func(opts *retryOptions) {
		opts.logger = logger
	}
}

func retryOptionsLogger(retryOptions retryOptions) *slog.Logger {
	if retryOptions.logger == nil {
		return slog.Default()
	}

	return retryOptions.logger
}

func retryCountExceeded(retryOptions retryOptions, attemptNumber uint64) bool {
	if retryOptions.maxAttempts == nil {
		return false
	}

	return attemptNumber > *retryOptions.maxAttempts
}

type Policy struct {
	storage Storage
	policy  retry.Policy
	options retryOptions
}

func requestIDAttr(requestID uuid.UUID) slog.Attr {
	return slog.String("requestID", requestID.String())
}

func errorAttr(err error) slog.Attr {
	return slog.String("err", err.Error())
}

func completedRequestAttr(completedRequest CompletedRequest) slog.Attr {
	return slog.Any("completedRequest", completedRequest)
}

func abortedRequestAttr(abortedRequest AbortedRequest) slog.Attr {
	return slog.Any("abortedRequest", abortedRequest)
}

func attemptAttr(attempt Attempt) slog.Attr {
	return slog.Any("attempt", attempt)
}

func lastAttemptNumberAttr(lastAttemptNumber uint64) slog.Attr {
	return slog.Uint64("lastAttemptNumber", lastAttemptNumber)
}

func attemptNumberAttr(attemptNumber uint64) slog.Attr {
	return slog.Uint64("attemptNumber", attemptNumber)
}

func maxAttemptsAttr(maxAttempts *uint64) slog.Attr {
	return slog.Any("maxAttempts", maxAttempts)
}

func opError(opName string, err error) error {
	return fmt.Errorf(opName+": %w", err)
}

func opAttr(opName string) slog.Attr {
	return slog.String("op", opName)
}

const (
	failedMsg             = "failed"
	startMsg              = "started"
	finishMsg             = "finished"
	retryCountExceededMsg = "retry count exceeded"
)

func New(
	storage Storage,
	policy retry.Policy,
	opts ...Option,
) Policy {
	return Policy{
		storage: storage,
		policy:  policy,
		options: makeRetryOptions(opts),
	}
}

func (p *Policy) Retry(
	ctx context.Context,
	requestID uuid.UUID,
	rfctx retry.FuncContext,
) error {
	log := retryOptionsLogger(p.options)
	log = log.With(
		requestIDAttr(requestID),
		maxAttemptsAttr(p.options.maxAttempts),
	)

	lastAttemptNumber, err := p.lastAttemptNumber(ctx, log, requestID)
	if err != nil {
		return err
	}

	if retryCountExceeded(p.options, lastAttemptNumber) {
		log.ErrorContext(ctx,
			retryCountExceededMsg,
			opAttr("checkRetryExceeded"),
			errorAttr(retry.ErrRetryCountExceeded),
			attemptNumberAttr(lastAttemptNumber),
		)

		return p.markRequestAsAborted(ctx, log,
			AbortedRequest{
				RequestID: requestID,
				Err:       retry.ErrRetryCountExceeded,
				AbortedAt: time.Now().UTC(),
			},
		)
	}

	retryErr := p.retry(ctx, log, requestID, rfctx)
	switch retryErr {
	case nil:
		return p.markRequestAsCompleted(ctx, log,
			CompletedRequest{
				RequestID:   requestID,
				CompletedAt: time.Now().UTC(),
			},
		)
	default:
		return p.markRequestAsAborted(ctx, log,
			AbortedRequest{
				RequestID: requestID,
				Err:       retryErr,
				AbortedAt: time.Now().UTC(),
			},
		)
	}
}

func (p *Policy) lastAttemptNumber(
	ctx context.Context,
	log *slog.Logger,
	requestID uuid.UUID,
) (uint64, error) {
	const lastAttemptNumberOp = "storage.LastAttemptNumber"

	log = log.With(
		opAttr(lastAttemptNumberOp),
	)

	log.InfoContext(ctx, startMsg)

	lastAttemptNumber, err := p.storage.LastRequestAttemptNumber(ctx, requestID)
	if err != nil {
		log.ErrorContext(ctx,
			failedMsg,
			errorAttr(err),
		)

		return 0, opError(lastAttemptNumberOp, err)
	}

	log.InfoContext(ctx, finishMsg, lastAttemptNumberAttr(lastAttemptNumber))

	return lastAttemptNumber, nil
}

func (p *Policy) retry(
	ctx context.Context,
	log *slog.Logger,
	requestID uuid.UUID,
	rfctx retry.FuncContext,
) error {
	return p.policy.RetryContext(ctx,
		func(ctx context.Context) retry.Result {
			result := rfctx(ctx)

			attempt := Attempt{
				RequestID: requestID,
				Err:       result.Err(),
				CreatedAt: time.Now().UTC(),
			}

			const insertRequestAttemptOp = "storage.InsertRequestAttempt"

			insertRequestAttemptLogger := log.With(
				attemptAttr(attempt),
				opAttr(insertRequestAttemptOp),
			)

			insertRequestAttemptLogger.InfoContext(ctx, startMsg)

			attemptNumber, err := p.storage.InsertRequestAttempt(ctx, attempt)
			if err != nil {
				insertRequestAttemptLogger.ErrorContext(ctx,
					failedMsg,
					errorAttr(err),
				)

				return result
			}

			insertRequestAttemptLogger = insertRequestAttemptLogger.With(
				attemptNumberAttr(attemptNumber),
			)

			if retryCountExceeded(p.options, attemptNumber) {
				insertRequestAttemptLogger.ErrorContext(ctx,
					retryCountExceededMsg,
					errorAttr(retry.ErrRetryCountExceeded),
				)

				return retry.Abort(retry.ErrRetryCountExceeded)
			}

			insertRequestAttemptLogger.InfoContext(ctx, finishMsg)

			return result
		},
	)
}

func (p *Policy) markRequestAsCompleted(
	ctx context.Context,
	log *slog.Logger,
	completedRequest CompletedRequest,
) error {
	const markRequestAsCompletedOp = "storage.MarkRequestAsCompleted"

	log = log.With(
		opAttr(markRequestAsCompletedOp),
		completedRequestAttr(completedRequest),
	)

	log.InfoContext(ctx, startMsg)

	err := p.storage.MarkRequestAsCompleted(ctx, completedRequest)
	if err != nil {
		log.ErrorContext(ctx,
			failedMsg,
			errorAttr(err),
		)

		return opError(markRequestAsCompletedOp, err)
	}

	log.InfoContext(ctx, finishMsg)

	return nil
}

func (p *Policy) markRequestAsAborted(
	ctx context.Context,
	log *slog.Logger,
	abortedRequest AbortedRequest,
) error {
	const markRequestAsAbortedOp = "storage.MarkRequestAsAborted"

	log = log.With(
		opAttr(markRequestAsAbortedOp),
		abortedRequestAttr(abortedRequest),
	)

	log.InfoContext(ctx, startMsg)

	err := p.storage.MarkRequestAsAborted(ctx, abortedRequest)
	if err != nil {
		log.ErrorContext(ctx,
			failedMsg,
			errorAttr(err),
		)

		return opError(markRequestAsAbortedOp, err)
	}

	log.InfoContext(ctx, finishMsg)

	return nil
}
