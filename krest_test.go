package krest

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	tt "github.com/vingarcia/krest/internal/testtools"
)

func TestNew(t *testing.T) {
	tests := []struct {
		desc    string
		timeout time.Duration
	}{
		{
			desc:    "With timeout",
			timeout: 1 * time.Millisecond,
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			client := New(test.timeout)
			tt.AssertEqual(t, client.timeout, 1*time.Millisecond)
		})
	}
}

func TestKrestClient(t *testing.T) {
	ctx := context.Background()

	t.Run("public methods", func(t *testing.T) {
		tests := []struct {
			description string
			method      string
			requestData RequestData

			expectErrToContain []string
			expectedResp       string
			expectedStatusCode int
		}{
			{
				description:        "GET: request is successful",
				method:             "GET",
				expectedResp:       "Hello, client",
				expectedStatusCode: http.StatusOK,
			},
			{
				description:        "GET: bad request",
				method:             "GET",
				expectErrToContain: []string{"unexpected status code", "400", "Hello, client"},
				expectedResp:       "Hello, client",
				expectedStatusCode: http.StatusBadRequest,
			},
			{
				description:        "POST: request is successful",
				method:             "POST",
				expectedResp:       "Hello, client",
				expectedStatusCode: http.StatusOK,
			},
			{
				description:        "POST: bad request",
				method:             "POST",
				expectErrToContain: []string{"unexpected status code", "400", "Hello, client"},
				expectedResp:       "Hello, client",
				expectedStatusCode: http.StatusBadRequest,
			},
			{
				description:        "PUT: request is successful",
				method:             "PUT",
				expectedResp:       "Hello, client",
				expectedStatusCode: http.StatusOK,
			},
			{
				description:        "PUT: bad request",
				method:             "PUT",
				expectErrToContain: []string{"unexpected status code", "400", "Hello, client"},
				expectedResp:       "Hello, client",
				expectedStatusCode: http.StatusBadRequest,
			},
			{
				description:        "PATCH: request is successful",
				method:             "PATCH",
				expectedResp:       "Hello, client",
				expectedStatusCode: http.StatusOK,
			},
			{
				description:        "PATCH: bad request",
				method:             "PATCH",
				expectErrToContain: []string{"unexpected status code", "400", "Hello, client"},
				expectedResp:       "Hello, client",
				expectedStatusCode: http.StatusBadRequest,
			},
			{
				description:        "DELETE: request is successful",
				method:             "DELETE",
				expectedResp:       "Hello, client",
				expectedStatusCode: http.StatusOK,
			},
			{
				description:        "DELETE: bad request",
				method:             "DELETE",
				expectErrToContain: []string{"unexpected status code", "400", "Hello, client"},
				expectedResp:       "Hello, client",
				expectedStatusCode: http.StatusBadRequest,
			},
			{
				description:        "OPTIONS: request is successful",
				method:             "OPTIONS",
				expectedResp:       "Hello, client",
				expectedStatusCode: http.StatusOK,
			},
			{
				description:        "OPTIONS: bad request",
				method:             "OPTIONS",
				expectErrToContain: []string{"unexpected status code", "400", "Hello, client"},
				expectedResp:       "Hello, client",
				expectedStatusCode: http.StatusBadRequest,
			},
		}

		for _, test := range tests {
			t.Run(test.description, func(t *testing.T) {
				svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(test.expectedStatusCode)
					_, _ = fmt.Fprint(w, test.expectedResp)
				}))
				defer svr.Close()

				client := New(1 * time.Second)

				res, err := client.Do(ctx, test.method, svr.URL, test.requestData)
				if test.expectErrToContain != nil {
					tt.AssertErrContains(t, err, test.expectErrToContain...)
				} else {
					tt.AssertNoErr(t, err)
				}

				body, err := io.ReadAll(res.ReadCloser)
				tt.AssertNoErr(t, err)

				tt.AssertEqual(t, string(body), test.expectedResp)
				tt.AssertEqual(t, res.StatusCode, test.expectedStatusCode)
			})
		}
	})

	t.Run("makeRequest", func(t *testing.T) {
		tests := []struct {
			description        string
			requestData        RequestData
			responseStatusCode int
			responseHeaders    map[string]string

			expectedRequestHeaders  url.Values
			expectedResponseHeaders map[string]string
			expectedRequestBody     string
			expectErrToContain      []string
		}{
			{
				description:         "should work with a nil body",
				requestData:         RequestData{},
				expectedRequestBody: "",
				responseStatusCode:  http.StatusOK,
			},
			{
				description: "should work with bodies of type string",
				requestData: RequestData{
					Body: "Hello, client",
				},
				expectedRequestBody: "Hello, client",
				responseStatusCode:  http.StatusOK,
			},
			{
				description: "should work with of type []byte",
				requestData: RequestData{
					Body: []byte("Hello, client"),
				},

				expectedRequestBody: "Hello, client",
				responseStatusCode:  http.StatusOK,
			},
			{
				description: "should work with bodies of type io.Readers",
				requestData: RequestData{
					Body: strings.NewReader("Hello, client"),
				},

				expectedRequestBody: "Hello, client",
				responseStatusCode:  http.StatusOK,
			},
			{
				description: "should marshal bodies of type map as JSON",
				requestData: RequestData{
					Body: map[string]interface{}{
						"fakeAttr": "fakeValue",
					},
				},

				expectedRequestBody: `{"fakeAttr":"fakeValue"}`,
				responseStatusCode:  http.StatusOK,
			},
			{
				description: "should marshal bodies of type struct as JSON",
				requestData: RequestData{
					Body: struct {
						FakeAttr string `json:"fakeAttr"`
					}{FakeAttr: "fakeValue"},
				},

				expectedRequestBody: `{"fakeAttr":"fakeValue"}`,
				responseStatusCode:  http.StatusOK,
			},

			{
				description: "should send headers correctly",
				requestData: RequestData{
					Headers: map[string]any{
						"fakeHeaderKey": "fakeHeaderValue",
						"fakeMultiHeaderKey": []string{
							"fakeHeaderValue1", "fakeHeaderValue2",
						},
					},
				},

				expectedRequestHeaders: url.Values{
					// url.Values keys are formatted like this:
					"Fakeheaderkey": []string{"fakeHeaderValue"},
					"Fakemultiheaderkey": []string{
						"fakeHeaderValue1", "fakeHeaderValue2",
					},
				},
				responseStatusCode: http.StatusOK,
			},
			{
				description: "should report errors when invalid header values are passed",
				requestData: RequestData{
					Headers: map[string]any{
						"fakeHeaderKey": []any{"notAValidHeaderValueType"},
					},
				},

				expectErrToContain: []string{"invalid", "header", "fakeHeaderKey"},
			},
			{
				description: "should parse response headers correctly",
				requestData: RequestData{},
				responseHeaders: map[string]string{
					"fakeHeaderKey": "fakeHeaderValue",
				},

				expectedResponseHeaders: map[string]string{
					"fakeHeaderKey": "fakeHeaderValue",
				},
				responseStatusCode: http.StatusOK,
			},
		}

		for _, test := range tests {
			t.Run(test.description, func(t *testing.T) {
				var requestBody []byte
				var requestHeaders http.Header
				svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					var err error
					requestBody, err = io.ReadAll(r.Body)
					tt.AssertNoErr(t, err)

					requestHeaders = r.Header

					for key, value := range test.responseHeaders {
						w.Header().Set(key, value)
					}
					if test.responseStatusCode < 100 {
						test.responseStatusCode = 500
					}
					w.WriteHeader(test.responseStatusCode)
				}))
				defer svr.Close()

				client := New(1 * time.Second)

				res, err := client.makeRequest(ctx, "POST", svr.URL, test.requestData)
				if test.expectErrToContain != nil {
					tt.AssertErrContains(t, err, test.expectErrToContain...)
				}

				tt.AssertEqual(t, res.StatusCode, test.responseStatusCode)

				tt.AssertEqual(t, string(requestBody), test.expectedRequestBody)

				for key, value := range test.expectedRequestHeaders {
					tt.AssertEqual(t, requestHeaders[key], value)
				}

				for key, value := range test.expectedResponseHeaders {
					tt.AssertEqual(t, res.Headers.Get(key), value)
				}
			})
		}
	})

	t.Run("makeRequestWithMiddlewares", func(t *testing.T) {
		t.Run("should run all the middlewares in the provided order", func(t *testing.T) {
			middlewares := []Middleware{
				func(
					ctx context.Context,
					method string,
					url string,
					data RequestData,
					next NextMiddleware,
				) (Response, error) {
					data.Body = append(data.Body.([]string), "firstMiddleware")
					resp, err := next(ctx, method, url, data)
					resp.Body = append(resp.Body, []byte("+firstMiddleware")...)
					err = AppendErr(err, fmt.Errorf("err from firstMiddleware"))
					return resp, err
				},
				func(
					ctx context.Context,
					method string,
					url string,
					data RequestData,
					next NextMiddleware,
				) (Response, error) {
					data.Body = append(data.Body.([]string), "secondMiddleware")
					resp, err := next(ctx, method, url, data)
					resp.Body = append(resp.Body, []byte("+secondMiddleware")...)
					err = AppendErr(err, fmt.Errorf("err from secondMiddleware"))
					return resp, err
				},
			}

			var requestBody []byte
			svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var err error
				requestBody, err = io.ReadAll(r.Body)
				tt.AssertNoErr(t, err)

				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte("respFromServer"))
			}))
			defer svr.Close()

			client := New(1*time.Second, middlewares...)

			res, err := client.makeRequestWithMiddlewares(ctx, "POST", svr.URL, RequestData{
				Body: []string{
					"bodyFromRequest",
				},
			})
			tt.AssertErrContains(t, err, "firstMiddleware", "secondMiddleware", "400")
			tt.AssertContains(t, string(res.Body), "firstMiddleware", "secondMiddleware", "respFromServer")
			tt.AssertContains(t, string(requestBody), "firstMiddleware", "secondMiddleware", "bodyFromRequest")
		})

		t.Run("middlewares should be able to abort a request before sending it", func(t *testing.T) {
			var passedOn []string

			middlewares := []Middleware{
				func(
					ctx context.Context,
					method string,
					url string,
					data RequestData,
					next NextMiddleware,
				) (Response, error) {
					passedOn = append(passedOn, "firstMiddleware")
					return Response{}, fmt.Errorf("abortingRequest")
				},
				func(
					ctx context.Context,
					method string,
					url string,
					data RequestData,
					next NextMiddleware,
				) (Response, error) {
					passedOn = append(passedOn, "secondMiddleware")
					return next(ctx, method, url, data)
				},
			}

			svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				passedOn = append(passedOn, "server")
			}))
			defer svr.Close()

			client := New(1*time.Second, middlewares...)

			_, _ = client.makeRequestWithMiddlewares(ctx, "POST", svr.URL, RequestData{})

			tt.AssertEqual(t, passedOn, []string{
				"firstMiddleware",
			})
		})
	})

	t.Run("followRedirects config", func(t *testing.T) {

		t.Run("should not follow redirects by default", func(t *testing.T) {
			svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/redirected" {
					return
				}

				http.Redirect(w, r, "/redirected", http.StatusTemporaryRedirect)
			}))
			defer svr.Close()

			client := New(time.Second)

			resp, err := client.Get(ctx, svr.URL, RequestData{})
			tt.AssertErrContains(t, err, "307")
			tt.AssertEqual(t, resp.StatusCode, 307)
		})

		t.Run("should follow redirects when explicitly requested", func(t *testing.T) {
			svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/redirected" {
					return
				}

				http.Redirect(w, r, "/redirected", http.StatusTemporaryRedirect)
			}))
			defer svr.Close()

			client := New(time.Second)

			resp, err := client.Get(ctx, svr.URL, RequestData{
				FollowRedirects: true,
			})
			tt.AssertNoErr(t, err)
			tt.AssertEqual(t, resp.StatusCode, 200)
		})
	})
}

type multierr []error

func (m multierr) Error() string {
	return fmt.Sprint([]error(m))
}

func AppendErr(oldErr error, newErr error) error {
	if oldErr == nil {
		return newErr
	}

	if newErr == nil {
		return oldErr
	}

	if mErr, ok := oldErr.(multierr); ok {
		return append(mErr, newErr)
	}

	return multierr{oldErr, newErr}
}

func TestRequestRetry(t *testing.T) {
	tests := []struct {
		desc               string
		body               interface{}
		expectedPayload    string
		expectErrToContain []string
	}{
		{
			desc:            "should rebuild the payload correctly when retrying with bytes input",
			body:            []byte("fakeBytesBody"),
			expectedPayload: "fakeBytesBody",
		},
		{
			desc: "should rebuild the payload correctly when retrying with input meant to be marshalled as JSON",
			body: map[string]string{
				"fakeKey": "fakeValue",
			},
			expectedPayload: `{"fakeKey":"fakeValue"}`,
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			respCodes := []int{502, 200}
			var payload []byte

			svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var err error
				payload, err = io.ReadAll(r.Body)
				tt.AssertNoErr(t, err)

				code := respCodes[0]
				respCodes = respCodes[1:]
				w.WriteHeader(code)
				_, _ = fmt.Fprint(w, test.body)
			}))
			defer svr.Close()

			client := New(1 * time.Second)

			_, err := client.Post(context.TODO(), svr.URL, RequestData{
				Body:       test.body,
				MaxRetries: 2,
			})
			if test.expectErrToContain != nil {
				tt.AssertErrContains(t, err, test.expectErrToContain...)
			}
			tt.AssertNoErr(t, err)

			tt.AssertEqual(t, string(payload), test.expectedPayload)
		})
	}
}

// stubResponse describes one canned HTTP response returned by the test server.
type stubResponse struct {
	status     int
	retryAfter string // Retry-After header value; empty means no header
}

func TestRequestRetryAfter(t *testing.T) {
	// Remove backoff randomness so the elapsed-time assertions are stable.
	randMillis = func() time.Duration {
		return 0
	}
	defer func() {
		randMillis = DefaultRandMillis
	}()

	// An HTTP-date already in the past exercises the HTTP-date format end-to-end
	// while clamping to a zero wait, keeping the test fast.
	pastDate := time.Now().Add(-1 * time.Hour).UTC().Format(http.TimeFormat)

	tests := []struct {
		desc       string
		action     RetryAfterAction
		responses  []stubResponse
		baseDelay  time.Duration
		maxDelay   time.Duration
		ctxTimeout time.Duration // 0 means no deadline

		expectedCalls  int
		expectedStatus int
		expectErr      bool
		maxElapsed     time.Duration
	}{
		{
			desc:           "honor with a past http-date retries immediately",
			action:         RetryAfterHonor,
			responses:      []stubResponse{{status: 503, retryAfter: pastDate}, {status: 200}},
			maxDelay:       5 * time.Second,
			expectedCalls:  2,
			expectedStatus: 200,
			maxElapsed:     500 * time.Millisecond,
		},
		{
			desc:           "honor returns immediately when the header exceeds the cap",
			action:         RetryAfterHonor,
			responses:      []stubResponse{{status: 429, retryAfter: "10"}, {status: 200}},
			maxDelay:       1 * time.Second,
			expectedCalls:  1,
			expectedStatus: 429,
			expectErr:      true,
			maxElapsed:     500 * time.Millisecond,
		},
		{
			desc:           "error mode returns the response without retrying",
			action:         RetryAfterError,
			responses:      []stubResponse{{status: 429, retryAfter: "5"}, {status: 200}},
			maxDelay:       5 * time.Second,
			expectedCalls:  1,
			expectedStatus: 429,
			expectErr:      true,
			maxElapsed:     500 * time.Millisecond,
		},
		{
			desc:           "ignore mode uses normal backoff despite the header",
			action:         RetryAfterIgnore,
			responses:      []stubResponse{{status: 429, retryAfter: "10"}, {status: 200}},
			baseDelay:      1 * time.Millisecond,
			maxDelay:       1 * time.Millisecond,
			expectedCalls:  2,
			expectedStatus: 200,
			maxElapsed:     500 * time.Millisecond,
		},
		{
			desc:           "honor with a missing header uses normal backoff",
			action:         RetryAfterHonor,
			responses:      []stubResponse{{status: 429}, {status: 200}},
			baseDelay:      1 * time.Millisecond,
			maxDelay:       1 * time.Millisecond,
			expectedCalls:  2,
			expectedStatus: 200,
			maxElapsed:     500 * time.Millisecond,
		},
		{
			desc:           "honor with a malformed header uses normal backoff",
			action:         RetryAfterHonor,
			responses:      []stubResponse{{status: 429, retryAfter: "not-a-date"}, {status: 200}},
			baseDelay:      1 * time.Millisecond,
			maxDelay:       1 * time.Millisecond,
			expectedCalls:  2,
			expectedStatus: 200,
			maxElapsed:     500 * time.Millisecond,
		},
		{
			desc:           "honor is bounded by a shorter context deadline",
			action:         RetryAfterHonor,
			responses:      []stubResponse{{status: 429, retryAfter: "1"}, {status: 200}},
			maxDelay:       5 * time.Second,
			ctxTimeout:     40 * time.Millisecond,
			expectedCalls:  1,
			expectedStatus: 429,
			expectErr:      true,
			maxElapsed:     500 * time.Millisecond,
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			responses := test.responses
			calls := 0
			svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				resp := responses[0]
				responses = responses[1:]
				if resp.retryAfter != "" {
					w.Header().Set("Retry-After", resp.retryAfter)
				}
				w.WriteHeader(resp.status)
				_, _ = fmt.Fprint(w, "body")
			}))
			defer svr.Close()

			ctx := context.Background()
			if test.ctxTimeout > 0 {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, test.ctxTimeout)
				defer cancel()
			}

			client := New(5 * time.Second)
			start := time.Now()
			resp, err := client.Get(ctx, svr.URL, RequestData{
				MaxRetries:       2,
				BaseRetryDelay:   test.baseDelay,
				MaxRetryDelay:    test.maxDelay,
				RetryAfterAction: test.action,
			})
			elapsed := time.Since(start)

			tt.AssertEqual(t, calls, test.expectedCalls)
			tt.AssertEqual(t, resp.StatusCode, test.expectedStatus)
			if test.expectErr {
				tt.AssertErrContains(t, err, "unexpected status code")
			} else {
				tt.AssertNoErr(t, err)
			}

			tt.AssertTrue(t, elapsed <= test.maxElapsed,
				fmt.Sprintf("expected at most %v elapsed, got %v", test.maxElapsed, elapsed))
		})
	}
}

func TestTransportCache(t *testing.T) {
	t.Run("should reuse transport for nil TLSConfig", func(t *testing.T) {
		cache := &transportCache{
			defaultTransport: newTransport(nil),
			maxSize:          defaultMaxTransports,
		}

		t1 := cache.get(nil)
		t2 := cache.get(nil)
		tt.AssertTrue(t, t1 == t2, "transports should match")
	})

	t.Run("should reuse transport for same TLSConfig pointer", func(t *testing.T) {
		cache := &transportCache{
			defaultTransport: newTransport(nil),
			maxSize:          defaultMaxTransports,
		}

		cfg := &tls.Config{}
		t1 := cache.get(cfg)
		t2 := cache.get(cfg)
		tt.AssertTrue(t, t1 == t2, "transports should match")
	})

	t.Run("should create different transports for different TLSConfig pointers", func(t *testing.T) {
		cache := &transportCache{
			defaultTransport: newTransport(nil),
			maxSize:          defaultMaxTransports,
		}

		cfg1 := &tls.Config{}
		cfg2 := &tls.Config{}
		t1 := cache.get(cfg1)
		t2 := cache.get(cfg2)
		tt.AssertTrue(t, t1 != t2, "transports should be different")
	})

	t.Run("should evict oldest transport when cache is full", func(t *testing.T) {
		cache := &transportCache{
			defaultTransport: newTransport(nil),
			maxSize:          2,
		}

		cfg1 := &tls.Config{}
		cfg2 := &tls.Config{}
		cfg3 := &tls.Config{}

		t1 := cache.get(cfg1)
		cache.get(cfg2)

		// This should evict cfg1
		cache.get(cfg3)

		// cfg1 should now create a new transport
		t1New := cache.get(cfg1)
		tt.AssertTrue(t, t1 != t1New, "t1 should have been evicted and replaced")
	})

	t.Run("should not evict recently accessed entries that are still in ring", func(t *testing.T) {
		cache := &transportCache{
			defaultTransport: newTransport(nil),
			maxSize:          3,
		}

		cfg1 := &tls.Config{}
		cfg2 := &tls.Config{}

		t1 := cache.get(cfg1)
		cache.get(cfg2)

		// Access cfg1 again (cache hit, no new ring entry)
		t1Again := cache.get(cfg1)
		tt.AssertTrue(t, t1 == t1Again, "t1 should still be the same")
	})
}
