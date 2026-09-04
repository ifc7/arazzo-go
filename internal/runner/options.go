package runner

import (
	"context"
	"log/slog"
	"net/http"
	"time"
)

// Option configures a Runner.
type Option func(*options)

type options struct {
	httpClient   *http.Client
	baseURL      string
	timeout      time.Duration
	logger       *slog.Logger
	sleepFunc    func(ctx context.Context, d time.Duration) error
	skipValidate bool
}

func defaultOptions() options {
	return options{
		timeout: 60 * time.Second,
		logger:  slog.Default(),
	}
}

func (o *options) apply(opts []Option) {
	for _, opt := range opts {
		if opt != nil {
			opt(o)
		}
	}
}

// WithHTTPClient sets the HTTP client used for workflow steps and source fetches.
// If the client has no cookie jar, the runner clones it and attaches one.
func WithHTTPClient(client *http.Client) Option {
	return func(o *options) {
		o.httpClient = client
	}
}

// WithBaseURL sets the default API base URL. ExecuteWorkflow overrides this
// when inputs contain a non-empty "baseUrl" string (as login.arazzo.yaml does).
func WithBaseURL(baseURL string) Option {
	return func(o *options) {
		o.baseURL = baseURL
	}
}

// WithTimeout sets the HTTP client timeout when the runner creates the client.
func WithTimeout(d time.Duration) Option {
	return func(o *options) {
		o.timeout = d
	}
}

// WithLogger sets the logger used for request traces. Secret fields are redacted.
func WithLogger(logger *slog.Logger) Option {
	return func(o *options) {
		if logger != nil {
			o.logger = logger
		}
	}
}

// WithFastRetries skips waiting between Arazzo retryAfter delays. Intended for tests.
func WithFastRetries() Option {
	return func(o *options) {
		o.sleepFunc = func(ctx context.Context, d time.Duration) error {
			return ctx.Err()
		}
	}
}

// WithSkipValidate skips structural validation of the Arazzo document.
func WithSkipValidate() Option {
	return func(o *options) {
		o.skipValidate = true
	}
}
