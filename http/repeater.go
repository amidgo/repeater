package http

import (
	"context"
	"net/http"
	"net/url"
	"regexp"

	"github.com/amidgo/repeater"
)

var (
	// A regular expression to match the error returned by net/http when the
	// configured number of redirects is exhausted. This error isn't typed
	// specifically so we resort to matching on the error string.
	redirectsErrorRe = regexp.MustCompile(`stopped after \d+ redirects\z`)

	// A regular expression to match the error returned by net/http when the
	// scheme specified in the URL is invalid. This error isn't typed
	// specifically so we resort to matching on the error string.
	schemeErrorRe = regexp.MustCompile(`unsupported protocol scheme`)

	// A regular expression to match the error returned by net/http when a
	// request header or value is invalid. This error isn't typed
	// specifically so we resort to matching on the error string.
	invalidHeaderErrorRe = regexp.MustCompile(`invalid header`)

	// A regular expression to match the error returned by net/http when the
	// TLS certificate is not trusted. This error isn't typed
	// specifically so we resort to matching on the error string.
	notTrustedErrorRe = regexp.MustCompile(`certificate is not trusted`)
)

type Repeater struct {
	client   *http.Client
	repeater *repeater.Repeater
}

func (r *Repeater) Client() *http.Client {
	return r.client
}

func (r *Repeater) Do(req *http.Request, retryCount uint64) (resp *http.Response, err error) {
	_ = r.repeater.RepeatContext(
		req.Context(),
		func(ctx context.Context) (finished bool) {
			resp, err = r.client.Do(req)

			return shouldFinishRetry(resp, err)
		},
		retryCount,
	)

	return resp, err
}

func shouldFinishRetry(resp *http.Response, err error) bool {
	if err != nil {
		if v, ok := err.(*url.Error); ok {
			// Don't retry if the error was due to too many redirects.
			if redirectsErrorRe.MatchString(v.Error()) {
				return true
			}

			// Don't retry if the error was due to an invalid protocol scheme.
			if schemeErrorRe.MatchString(v.Error()) {
				return true
			}

			// Don't retry if the error was due to an invalid header.
			if invalidHeaderErrorRe.MatchString(v.Error()) {
				return true
			}

			// Don't retry if the error was due to TLS cert verification failure.
			if notTrustedErrorRe.MatchString(v.Error()) {
				return true
			}

			if isCertError(v.Err) {
				return true
			}
		}

		// The error is likely recoverable so retry.
		return false
	}

	// 429 Too Many Requests is recoverable. Sometimes the server puts
	// a Retry-After response header to indicate when the server is
	// available to start processing request from client.
	if resp.StatusCode == http.StatusTooManyRequests {
		return false
	}

	// Check the response code. We retry on 500-range responses to allow
	// the server time to recover, as 500's are typically not permanent
	// errors and may relate to outages on the server side. This will catch
	// invalid response codes as well, like 0 and 999.
	if resp.StatusCode == 0 || (resp.StatusCode >= 500 && resp.StatusCode != http.StatusNotImplemented) {
		return false
	}

	return true
}
