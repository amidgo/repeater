package retryhttp_test

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"sync/atomic"
	"testing"

	_ "embed"

	"github.com/amidgo/httpmock"
	"github.com/amidgo/retry"
	"github.com/amidgo/retry/retryhttp"
)

type HandlerMock func(t *testing.T) func(ctx context.Context, resp *http.Response, err error) retry.Result

type transportTest struct {
	Name             string
	Request          *http.Request
	Policy           retry.Policy
	Calls            httpmock.Calls
	Handler          func(t *testing.T) func(ctx context.Context, resp *http.Response, err error) retry.Result
	ExpectedResponse *httpmock.Response
	ExpectedError    error
}

func (tt *transportTest) Test(t *testing.T) {
	transport := retryhttp.NewTransport(
		tt.Policy,
		httpmock.NewTransport(t, tt.Calls, httpmock.HandleCallCompareInput),
		tt.Handler(t),
	)

	resp, err := transport.RoundTrip(tt.Request)

	if !reflect.DeepEqual(resp, expectedResponse(t, tt.ExpectedResponse)) {
		t.Fatalf("compare response, responses not equal\n\nexpected:\n%+v\n\nactual:\n%+v", tt.ExpectedResponse, resp)
	}

	assertResultError(t, tt.ExpectedError, err)
}

func expectedResponse(t *testing.T, expectedResponse *httpmock.Response) *http.Response {
	if expectedResponse == nil {
		return nil
	}

	rec := httptest.NewRecorder()

	err := httpmock.WriteResponse(rec, *expectedResponse)
	if err != nil {
		t.Fatalf("httpmock.WriteResponse, unexpected error: %+v", err)

		return &http.Response{}
	}

	return rec.Result()
}

func assertResultError(t *testing.T, expectedErr, resultErr error) {
	if expectedErr == resultErr {
		return
	}

	expectedErrs := extractErrors(expectedErr)
	resultErrs := extractErrors(resultErr)

	if len(expectedErrs) != len(resultErrs) {
		t.Fatalf("wrong result err\n\nexpected:\n%+v\n\nactual:\n%+v", expectedErr, resultErr)

		return
	}

	for i := range expectedErrs {
		if expectedErrs[i] != resultErrs[i] {
			t.Fatalf("wrong result err, check %d element of errs\n\nexpected:\n%+v\n\nactual:\n%+v", i+1, expectedErrs[i], resultErrs[i])

			return
		}
	}
}

func extractErrors(expectedErr error) []error {
	switch expectedErr := expectedErr.(type) {
	case interface{ Unwrap() []error }:
		return expectedErr.Unwrap()
	case interface{ Unwrap() error }:
		return []error{expectedErr.Unwrap()}
	default:
		return []error{expectedErr}
	}
}
func joinHandlerMocks(handlers ...HandlerMock) HandlerMock {
	callsCounter := atomic.Int64{}

	return func(t *testing.T) func(context.Context, *http.Response, error) retry.Result {
		t.Cleanup(
			func() {
				callsOccurred := int(callsCounter.Load())

				if callsOccurred > len(handlers) {
					t.Fatalf("too many calls occurred to HandlerMock: %d calls, expected only %d calls", callsOccurred, len(handlers))
				}

				if callsOccurred < len(handlers) {
					t.Fatalf("not all expected calls occurred to HandlerMock: %d calls, expected %d calls", callsOccurred, len(handlers))
				}
			},
		)

		return func(ctx context.Context, resp *http.Response, err error) retry.Result {
			callNumber := int(callsCounter.Add(1))

			if callNumber > len(handlers) {
				t.Errorf("joinHandlerMocks, received too many calls, %d maximum, %d actual call", len(handlers), callNumber)

				return retry.Finish()
			}

			handler := handlers[callNumber-1]

			return handler(t)(ctx, resp, err)
		}
	}
}

func newHandlerMockNilError(expectedResp httpmock.Response, result retry.Result) HandlerMock {
	return func(t *testing.T) func(context.Context, *http.Response, error) retry.Result {
		return func(_ context.Context, resp *http.Response, err error) retry.Result {
			if err != nil {
				t.Error("newHandlerMockNilError, received unexpected nil error")
			}

			rec := httptest.NewRecorder()

			writeResponseErr := httpmock.WriteResponse(rec, expectedResp)
			if err != nil {
				t.Fatalf("newHandlerMockNilError, httpmock.WriteResponse unexpected error: %+v", writeResponseErr)

				return result
			}

			if !reflect.DeepEqual(rec.Result(), resp) {
				t.Errorf("newHandlerMockNilError, compare response, response not equal\n\nexpected:\n%+v\n\nactual:\n%+v", expectedResp, resp)
			}

			return result
		}
	}
}

func newHandlerMockNilResponse(expectedErr error, result retry.Result) HandlerMock {
	return func(t *testing.T) func(context.Context, *http.Response, error) retry.Result {
		return func(_ context.Context, resp *http.Response, err error) retry.Result {
			if resp != nil {
				t.Error("unexpected non nil response")
			}

			if !errors.Is(err, expectedErr) {
				t.Errorf("newHandlerMockNilResponse, compare error, error not equal\n\nexpected:\n%+v\n\nactual:\n%+v", expectedErr, err)
			}

			return result
		}
	}
}

func Test_Transport(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/any/target", http.NoBody)
	input := httpmock.Input{
		Method: http.MethodGet,
		URL:    &url.URL{Path: "/any/target"},
	}

	tests := []*transportTest{
		{
			Name:    "success on first try",
			Request: req,
			Calls: httpmock.SequenceCalls(
				httpmock.Call{
					Input:    input,
					Response: httpmock.Response{StatusCode: http.StatusOK},
				},
			),
			Handler: newHandlerMockNilError(
				httpmock.Response{
					StatusCode: http.StatusOK,
				},
				retry.Finish(),
			),
			ExpectedResponse: &httpmock.Response{
				StatusCode: http.StatusOK,
			},
			ExpectedError: nil,
		},
		{
			Name:    "many calls",
			Request: req,
			Policy:  retry.New(retry.Plain(0), 2),
			Calls: httpmock.SequenceCalls(
				httpmock.Call{
					Input:    input,
					Response: httpmock.Response{StatusCode: http.StatusInternalServerError},
				},
				httpmock.Call{
					Input:   input,
					DoError: io.ErrUnexpectedEOF,
				},
				httpmock.Call{
					Input:    input,
					Response: httpmock.Response{StatusCode: http.StatusOK},
				},
			),
			Handler: joinHandlerMocks(
				newHandlerMockNilError(
					httpmock.Response{
						StatusCode: http.StatusInternalServerError,
					},
					retry.Continue(),
				),
				newHandlerMockNilResponse(
					io.ErrUnexpectedEOF,
					retry.Recover(io.ErrUnexpectedEOF),
				),
				newHandlerMockNilError(
					httpmock.Response{
						StatusCode: http.StatusOK,
					},
					retry.Finish(),
				),
			),
			ExpectedResponse: &httpmock.Response{
				StatusCode: http.StatusOK,
			},
			ExpectedError: nil,
		},
		{
			Name:    "recover error after retry count exceeded",
			Request: req,
			Policy:  retry.New(retry.Plain(0), 1),
			Calls: httpmock.SequenceCalls(
				httpmock.Call{
					Input: input,
					Response: httpmock.Response{
						StatusCode: http.StatusInternalServerError,
					},
				},
				httpmock.Call{
					Input:   input,
					DoError: io.ErrUnexpectedEOF,
				},
			),
			Handler: joinHandlerMocks(
				newHandlerMockNilError(
					httpmock.Response{
						StatusCode: http.StatusInternalServerError,
					},
					retry.Continue(),
				),
				newHandlerMockNilResponse(
					io.ErrUnexpectedEOF,
					retry.Recover(io.ErrUnexpectedEOF),
				),
			),
			ExpectedResponse: nil,
			ExpectedError:    errors.Join(retry.ErrRetryCountExceeded, io.ErrUnexpectedEOF),
		},
	}

	for _, tst := range tests {
		t.Run(tst.Name, tst.Test)
	}
}

func Test_DefaultHandleResponse_redirect(t *testing.T) {
	cl := &http.Client{
		Transport: httpmock.NewHandlerTransport(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, "/", http.StatusFound)
			}),
		),
	}

	resp, err := cl.Get("/")
	if err == nil {
		t.Fatal("unexpected nil error")
	}

	result := retryhttp.DefaultHandleResponse(t.Context(), resp, err)

	expectedResult := retry.Abort(err)

	equal, message := expectedResult.Eq(result)
	if !equal {
		t.Fatal(message)
	}
}

func Test_DefaultHandleResponse_missingProtocolScheme(t *testing.T) {
	cl := &http.Client{
		Transport: httpmock.NewHandlerTransport(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}),
		),
	}

	resp, err := cl.Get("://github.com/hashicorp/go-retryablehttp/blob/main/client_test.go")
	if err == nil {
		t.Fatalf("unexpected nil error, resp: %+v", resp)
	}

	result := retryhttp.DefaultHandleResponse(t.Context(), resp, err)

	expectedResult := retry.Abort(err)

	equal, message := expectedResult.Eq(result)
	if !equal {
		t.Fatal(message)
	}
}

func Test_DefaultHandleResponse_unsupportedProtocolScheme(t *testing.T) {
	cl := &http.Client{
		Transport: http.DefaultTransport,
	}

	resp, err := cl.Get("ftp://github.com/hashicorp/go-retryablehttp/blob/main/client_test.go")
	if err == nil {
		t.Fatalf("unexpected nil error, resp: %+v", resp)
	}

	result := retryhttp.DefaultHandleResponse(t.Context(), resp, err)

	expectedResult := retry.Abort(err)

	equal, message := expectedResult.Eq(result)
	if !equal {
		t.Fatal(message)
	}
}

func Test_DefaultHandleResponse_invalidHeaderKey(t *testing.T) {
	cl := &http.Client{}

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://github.com/any/target", http.NoBody)
	if err != nil {
		t.Fatalf("make request, unexpected error: %+v", err)
	}

	req.Header.Set("Header-Name-\033", "header value")

	resp, err := cl.Do(req)
	if err == nil {
		t.Fatal("unexpected non nil error")

		return
	}

	result := retryhttp.DefaultHandleResponse(t.Context(), resp, err)

	expectedResult := retry.Abort(err)

	equal, message := expectedResult.Eq(result)
	if !equal {
		t.Fatal(message)
	}
}

func Test_DefaultHandleResponse_invalidHeaderValue(t *testing.T) {
	cl := &http.Client{}

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://github.com/any/target", http.NoBody)
	if err != nil {
		t.Fatalf("make request, unexpected error: %+v", err)
	}

	req.Header.Set("Header-Name", "bad header value \033")

	resp, err := cl.Do(req)
	if err == nil {
		t.Fatal("unexpected nil error")

		return
	}

	result := retryhttp.DefaultHandleResponse(t.Context(), resp, err)

	expectedResult := retry.Abort(err)

	equal, message := expectedResult.Eq(result)
	if !equal {
		t.Fatal(message)
	}
}

var (
	//go:embed testdata/cert.pem
	LocalhostCert []byte
	//go:embed testdata/key.pem
	LocalhostKey []byte
)

func Test_DefaultHandleResponse_tlsCertificateVerificationError(t *testing.T) {
	cert, err := tls.X509KeyPair(LocalhostCert, LocalhostKey)
	if err != nil {
		panic(fmt.Sprintf("httptest: NewTLSServer: %v", err))
	}

	certificates := []tls.Certificate{cert}

	certificate, err := x509.ParseCertificate(certificates[0].Certificate[0])
	if err != nil {
		panic(fmt.Sprintf("httptest: NewTLSServer: %v", err))
	}

	certpool := x509.NewCertPool()
	certpool.AddCert(certificate)

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs: certpool,
			},
		},
	}

	resp, err := client.Get("https://github.com/amidgo/retry")
	if err == nil {
		t.Fatal("unexpected nil error")
	}

	urlErr, ok := err.(*url.Error)
	if !ok {
		t.Fatalf("unexpected type of returned error: %T, expected: %T", err, (*url.Error)(nil))
	}

	_, ok = urlErr.Err.(*tls.CertificateVerificationError)
	if !ok {
		t.Fatalf("unexpected type of returned error: %T, expected %T", urlErr.Err, (*tls.CertificateVerificationError)(nil))
	}

	result := retryhttp.DefaultHandleResponse(t.Context(), resp, err)

	expectedResult := retry.Abort(err)

	equal, message := expectedResult.Eq(result)
	if !equal {
		t.Fatal(message)
	}
}

func Test_DefaultHandleResponse_retryStatuses(t *testing.T) {
	codes := []int{
		http.StatusTooManyRequests,
		http.StatusInternalServerError,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout,
		http.StatusHTTPVersionNotSupported,
		http.StatusVariantAlsoNegotiates,
		http.StatusInsufficientStorage,
		http.StatusLoopDetected,
		http.StatusNotExtended,
		http.StatusNetworkAuthenticationRequired,
	}

	for _, statusCode := range codes {
		t.Run(fmt.Sprintf("status code %d", statusCode), func(t *testing.T) {
			client := &http.Client{
				Transport: httpmock.NewHandlerTransport(
					http.HandlerFunc(
						func(w http.ResponseWriter, r *http.Request) {
							w.WriteHeader(statusCode)
						},
					),
				),
			}

			resp, err := client.Get("http://resource.com/")
			if err != nil {
				t.Fatalf("unexpeced non nil error: %s", err)

				return
			}

			result := retryhttp.DefaultHandleResponse(t.Context(), resp, err)

			expectedResult := retry.Continue()

			equal, message := expectedResult.Eq(result)
			if !equal {
				t.Fatal(message)
			}
		})
	}
}

func Test_DefaultHandleResponse_finishStatuses(t *testing.T) {
	codes := []int{
		http.StatusContinue,
		http.StatusSwitchingProtocols,
		http.StatusProcessing,
		http.StatusEarlyHints,
		http.StatusOK,
		http.StatusCreated,
		http.StatusAccepted,
		http.StatusNonAuthoritativeInfo,
		http.StatusNoContent,
		http.StatusResetContent,
		http.StatusPartialContent,
		http.StatusMultiStatus,
		http.StatusAlreadyReported,
		http.StatusIMUsed,
		http.StatusMultipleChoices,
		http.StatusMovedPermanently,
		http.StatusFound,
		http.StatusSeeOther,
		http.StatusNotModified,
		http.StatusUseProxy,
		http.StatusTemporaryRedirect,
		http.StatusPermanentRedirect,
		http.StatusBadRequest,
		http.StatusUnauthorized,
		http.StatusPaymentRequired,
		http.StatusForbidden,
		http.StatusNotFound,
		http.StatusMethodNotAllowed,
		http.StatusNotAcceptable,
		http.StatusProxyAuthRequired,
		http.StatusRequestTimeout,
		http.StatusConflict,
		http.StatusGone,
		http.StatusLengthRequired,
		http.StatusPreconditionFailed,
		http.StatusRequestEntityTooLarge,
		http.StatusRequestURITooLong,
		http.StatusUnsupportedMediaType,
		http.StatusRequestedRangeNotSatisfiable,
		http.StatusExpectationFailed,
		http.StatusTeapot,
		http.StatusMisdirectedRequest,
		http.StatusUnprocessableEntity,
		http.StatusLocked,
		http.StatusFailedDependency,
		http.StatusTooEarly,
		http.StatusUpgradeRequired,
		http.StatusPreconditionRequired,
		http.StatusRequestHeaderFieldsTooLarge,
		http.StatusUnavailableForLegalReasons,
		http.StatusNotImplemented,
	}

	for _, statusCode := range codes {
		t.Run(fmt.Sprintf("status code %d", statusCode), func(t *testing.T) {
			client := &http.Client{
				Transport: httpmock.NewHandlerTransport(
					http.HandlerFunc(
						func(w http.ResponseWriter, r *http.Request) {
							w.WriteHeader(statusCode)
						},
					),
				),
			}

			resp, err := client.Get("http://resource.com/")
			if err != nil {
				t.Fatalf("unexpeced non nil error: %s", err)

				return
			}

			result := retryhttp.DefaultHandleResponse(t.Context(), resp, err)

			expectedResult := retry.Finish()

			equal, message := expectedResult.Eq(result)
			if !equal {
				t.Fatal(message)
			}
		})
	}
}

func Test_DefaultHandleResponse_DoError(t *testing.T) {
	expectedErr := io.ErrShortWrite

	client := &http.Client{
		Transport: returnErrorTransport{Err: expectedErr},
	}

	resp, err := client.Get("http://resource.com/")

	urlErr, ok := err.(*url.Error)
	if !ok {
		t.Fatalf("unexpected type of returned error\n\nexpected:\n%T\n\nactual:\n%T", urlErr, err)
	}

	if urlErr.Err != expectedErr {
		t.Fatalf("unexpected error\n\nexpected:\n%s\n\nactual:\n%s", expectedErr, urlErr.Err)
	}

	result := retryhttp.DefaultHandleResponse(t.Context(), resp, err)

	expectedResult := retry.Recover(urlErr)

	equal, message := expectedResult.Eq(result)
	if !equal {
		t.Fatal(message)
	}
}

type returnErrorTransport struct{ Err error }

func (r returnErrorTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, r.Err
}
