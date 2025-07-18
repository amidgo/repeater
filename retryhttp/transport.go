package retryhttp

import (
	"context"
	"crypto/tls"
	"net/http"
	"net/url"
	"regexp"

	"github.com/amidgo/retry"
)

var (
	// A regular expression to match the error returned by net/http when the
	// configured number of redirects is exhausted. This error isn't typed
	// specifically so we resort to matching on the error string.
	redirectsErrorRe = regexp.MustCompile(`stopped after \d+ redirects\z`)

	// A regular expression to match the error returned by net/http when the
	// scheme specified in the URL is invalid. This error isn't typed
	// specifically so we resort to matching on the error string.
	missingProtocolScheme = regexp.MustCompile(`missing protocol scheme`)
	// A regular expression to match the error returned by net/http when the
	// scheme specified in the URL is invalid. This error isn't typed
	// specifically so we resort to matching on the error string.
	schemeErrorRe = regexp.MustCompile(`unsupported protocol scheme`)

	// A regular expression to match the error returned by net/http when a
	// request header or value is invalid. This error isn't typed
	// specifically so we resort to matching on the error string.
	invalidHeaderErrorRe = regexp.MustCompile(`invalid header`)
)

type Transport struct {
	policy         retry.Policy
	transport      http.RoundTripper
	handleResponse func(ctx context.Context, resp *http.Response, err error) retry.Result
}

var _ http.RoundTripper = (*Transport)(nil)

func NewTransport(policy retry.Policy, transport http.RoundTripper, handleResponse func(context.Context, *http.Response, error) retry.Result) *Transport {
	return &Transport{
		policy:         policy,
		transport:      transport,
		handleResponse: handleResponse,
	}
}

func (c *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	rf := DefaultHandleResponse
	if c.handleResponse != nil {
		rf = c.handleResponse
	}

	var (
		resp *http.Response
		err  error
	)

	retryErr := c.policy.RetryContext(
		req.Context(),
		func(ctx context.Context) retry.Result {
			resp, err = c.transport.RoundTrip(req)

			return rf(ctx, resp, err)
		},
	)
	if retryErr != nil {
		return nil, retryErr
	}

	return resp, nil
}

func DefaultHandleResponse(_ context.Context, resp *http.Response, err error) retry.Result {
	if err != nil {
		if v, ok := err.(*url.Error); ok {
			// Don't retry if the error was due to too many redirects.
			if redirectsErrorRe.MatchString(v.Error()) {
				return retry.Abort(err)
			}

			if missingProtocolScheme.MatchString(v.Error()) {
				return retry.Abort(err)
			}

			// Don't retry if the error was due to an invalid protocol scheme.
			if schemeErrorRe.MatchString(v.Error()) {
				return retry.Abort(err)
			}

			// Don't retry if the error was due to an invalid header.
			if invalidHeaderErrorRe.MatchString(v.Error()) {
				return retry.Abort(err)
			}

			if isCertError(v.Err) {
				return retry.Abort(err)
			}
		}

		// The error is likely recoverable so retry.
		return retry.Recover(err)
	}

	// 429 Too Many Requests is recoverable. Sometimes the server puts
	// a Retry-After response header to indicate when the server is
	// available to start processing request from client.
	if resp.StatusCode == http.StatusTooManyRequests {
		return retry.Continue()
	}

	// Check the response code. We retry on 500-range responses to allow
	// the server time to recover, as 500's are typically not permanent
	// errors and may relate to outages on the server side. This will catch
	// invalid response codes as well, like 0 and 999.
	if resp.StatusCode == 0 || (resp.StatusCode >= 500 && resp.StatusCode != http.StatusNotImplemented) {
		return retry.Continue()
	}

	return retry.Finish()
}

func isCertError(err error) bool {
	_, ok := err.(*tls.CertificateVerificationError)
	return ok
}
