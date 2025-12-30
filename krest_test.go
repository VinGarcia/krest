package krest

import (
	"context"
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
	type testCases struct {
		description string
		timeout     time.Duration
	}

	for _, test := range []testCases{
		{
			description: "With timeout",
			timeout:     1 * time.Millisecond,
		},
	} {
		t.Run(test.description, func(t *testing.T) {
			client := New(test.timeout)
			tt.AssertEqual(t, client.timeout, 1*time.Millisecond)
		})
	}
}

func TestKrestClient(t *testing.T) {
	ctx := context.Background()

	t.Run("public methods", func(t *testing.T) {
		type testCases struct {
			description string
			method      string

			expectedErr        []string
			expectedResp       string
			expectedStatusCode int
		}

		for _, test := range []testCases{
			{
				description:        "GET: request is successful",
				method:             "GET",
				expectedResp:       "Hello, client",
				expectedStatusCode: http.StatusOK,
			},
			{
				description:        "GET: bad request",
				method:             "GET",
				expectedErr:        []string{"unexpected status code", "400", "Hello, client"},
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
				expectedErr:        []string{"unexpected status code", "400", "Hello, client"},
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
				expectedErr:        []string{"unexpected status code", "400", "Hello, client"},
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
				expectedErr:        []string{"unexpected status code", "400", "Hello, client"},
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
				expectedErr:        []string{"unexpected status code", "400", "Hello, client"},
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
				expectedErr:        []string{"unexpected status code", "400", "Hello, client"},
				expectedResp:       "Hello, client",
				expectedStatusCode: http.StatusBadRequest,
			},
		} {
			t.Run(test.description, func(t *testing.T) {
				svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(test.expectedStatusCode)
					_, _ = fmt.Fprint(w, test.expectedResp)
				}))
				defer svr.Close()

				client := Client{
					timeout: 1 * time.Second,
				}

				res, err := client.Do(ctx, test.method, svr.URL, RequestData{
					Headers: map[string]any{
						"accept": "application/json",
					},
				})
				if err != nil {
					tt.AssertErrContains(t, err, test.expectedErr...)
				}

				body, err := io.ReadAll(res.ReadCloser)
				tt.AssertNoErr(t, err)

				tt.AssertEqual(t, string(body), test.expectedResp)
				tt.AssertEqual(t, res.StatusCode, test.expectedStatusCode)
			})
		}
	})

	t.Run("makeRequest", func(t *testing.T) {
		type testCases struct {
			description        string
			requestData        RequestData
			responseStatusCode int
			responseHeaders    map[string]string

			expectedRequestHeaders  url.Values
			expectedResponseHeaders map[string]string
			expectedRequestBody     string
			expectErrToContain      []string
		}

		for _, test := range []testCases{
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
		} {
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

				client := Client{
					timeout: 1 * time.Second,
				}

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
					tt.AssertEqual(t, res.Header.Get(key), value)
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

			client := Client{
				middlewares: middlewares,
				timeout:     1 * time.Second,
			}

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

			client := Client{
				middlewares: middlewares,
				timeout:     1 * time.Second,
			}

			_, _ = client.makeRequestWithMiddlewares(ctx, "POST", svr.URL, RequestData{})

			tt.AssertEqual(t, passedOn, []string{
				"firstMiddleware",
			})
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
	type testCase struct {
		desc               string
		body               interface{}
		expectedPayload    string
		expectErrToContain []string
	}

	for _, test := range []testCase{
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
	} {
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

			client := Client{
				timeout: 1 * time.Second,
			}

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
