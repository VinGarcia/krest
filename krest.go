package krest

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

const defaultMaxTransports = 10

// transportCache holds reusable HTTP transports to avoid creating a new one per request.
//
// A default transport is always available for requests without a custom TLSConfig (the common case).
// For requests with a custom TLSConfig, transports are cached by pointer identity and evicted
// using a ring buffer when the cache is full.
type transportCache struct {
	defaultTransport *http.Transport

	mu         sync.Mutex
	maxSize    int
	transports map[*tls.Config]*http.Transport
	ring       []*tls.Config
	ringIdx    int
}

func (tc *transportCache) get(tlsConfig *tls.Config) *http.Transport {
	if tlsConfig == nil {
		return tc.defaultTransport
	}

	tc.mu.Lock()
	defer tc.mu.Unlock()

	// Lazy init: only allocate the map and ring on first use
	if tc.transports == nil {
		tc.transports = make(map[*tls.Config]*http.Transport, tc.maxSize)
		tc.ring = make([]*tls.Config, tc.maxSize)
	}

	if t, ok := tc.transports[tlsConfig]; ok {
		return t
	}

	// Evict the oldest entry if the ring slot is occupied
	if old := tc.ring[tc.ringIdx]; old != nil {
		if t, ok := tc.transports[old]; ok {
			t.CloseIdleConnections()
			delete(tc.transports, old)
		}
	}

	t := newTransport(tlsConfig)
	tc.transports[tlsConfig] = t
	tc.ring[tc.ringIdx] = tlsConfig
	tc.ringIdx = (tc.ringIdx + 1) % tc.maxSize

	return t
}

func newTransport(tlsConfig *tls.Config) *http.Transport {
	return &http.Transport{
		TLSClientConfig: tlsConfig,

		// These configs below match the values for http.DefaultTransport:
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
	}
}

// Client contains methods for making rest requests.
// These methods accept any struct that can be marshaled into JSON
// but the response is returned in Bytes, since not all APIs follow
// rest strictly.
type Client struct {
	timeout     time.Duration
	middlewares []Middleware
	transports  *transportCache
}

// New instantiates a new rest client
func New(timeout time.Duration, middlewares ...Middleware) *Client {
	return &Client{
		timeout:     timeout,
		middlewares: middlewares,
		transports: &transportCache{
			defaultTransport: newTransport(nil),
			maxSize:          defaultMaxTransports,
		},
	}
}

// SetMaxTransports sets the maximum number of cached transports for custom TLS configurations.
// This does not affect the default transport used for requests without a TLSConfig.
// If maxTransports is 0 or negative, the default value of 10 is used.
func (c *Client) SetMaxTransports(maxTransports int) {
	if maxTransports <= 0 {
		maxTransports = defaultMaxTransports
	}
	c.transports.mu.Lock()
	defer c.transports.mu.Unlock()

	c.transports.maxSize = maxTransports
	// Reset the cache so the new size takes effect cleanly
	for _, cfg := range c.transports.ring {
		if cfg != nil {
			if t, ok := c.transports.transports[cfg]; ok {
				t.CloseIdleConnections()
			}
		}
	}
	c.transports.transports = nil
	c.transports.ring = nil
	c.transports.ringIdx = 0
}

// AddMiddleware adds one or more new middlewares to this instance
func (c *Client) AddMiddleware(middlewares ...Middleware) {
	c.middlewares = append(c.middlewares, middlewares...)
}

// Get will make a GET request to the input URL
// and return the results
func (c *Client) Get(ctx context.Context, url string, data RequestData) (Response, error) {
	return c.makeRequestWithMiddlewares(ctx, "GET", url, data)
}

// Post will make a POST request to the input URL
// and return the results
func (c *Client) Post(ctx context.Context, url string, data RequestData) (Response, error) {
	return c.makeRequestWithMiddlewares(ctx, "POST", url, data)
}

// Put will make a PUT request to the input URL
// and return the results
func (c *Client) Put(ctx context.Context, url string, data RequestData) (Response, error) {
	return c.makeRequestWithMiddlewares(ctx, "PUT", url, data)
}

// Patch will make a PATCH request to the input URL
// and return the results
func (c *Client) Patch(ctx context.Context, url string, data RequestData) (Response, error) {
	return c.makeRequestWithMiddlewares(ctx, "PATCH", url, data)
}

// Delete will make a DELETE request to the input URL
// and return the results
func (c *Client) Delete(ctx context.Context, url string, data RequestData) (Response, error) {
	return c.makeRequestWithMiddlewares(ctx, "DELETE", url, data)
}

// Options will make a OPTIONS request to the input URL
// and return the results
func (c *Client) Options(ctx context.Context, url string, data RequestData) (Response, error) {
	return c.makeRequestWithMiddlewares(ctx, "OPTIONS", url, data)
}

// Do is only useful if you need to change the method programatically, otherwise prefer
// the other public functions like Get() and Post().
func (c *Client) Do(ctx context.Context, method string, url string, data RequestData) (Response, error) {
	switch strings.ToUpper(method) {
	case "GET":
		return c.Get(ctx, url, data)
	case "POST":
		return c.Post(ctx, url, data)
	case "PUT":
		return c.Put(ctx, url, data)
	case "PATCH":
		return c.Patch(ctx, url, data)
	case "DELETE":
		return c.Delete(ctx, url, data)
	case "OPTIONS":
		return c.Options(ctx, url, data)
	default:
		return Response{}, fmt.Errorf("unsupported request method: %q", method)
	}
}

func (c *Client) makeRequestWithMiddlewares(
	ctx context.Context,
	method string,
	url string,
	data RequestData,
) (Response, error) {
	// Start from back to front where the last middleware is c.makeRequest:
	middlewareChain := c.makeRequest
	for i := len(c.middlewares) - 1; i >= 0; i-- {
		i := i

		// Save a copy of the current head of the chain
		// so the closure below works correctly:
		var nextMiddleware = middlewareChain

		middlewareChain = func(
			ctx context.Context,
			method string,
			url string,
			data RequestData,
		) (Response, error) {
			return c.middlewares[i](ctx, method, url, data, nextMiddleware)
		}
	}

	return middlewareChain(ctx, method, url, data)
}

func (c *Client) makeRequest(
	ctx context.Context,
	method string,
	url string,
	data RequestData,
) (_ Response, err error) {
	data.SetDefaultsIfNecessary()

	var bytesPayload []byte
	var requestBody io.Reader
	switch body := data.Body.(type) {
	case nil:
		requestBody = nil
	case io.Reader:
		if data.MaxRetries > 1 {
			return Response{}, fmt.Errorf("can't retry a request whose body is an io.Reader")
		}

		requestBody = body
	case []byte:
		bytesPayload = body
	case string:
		bytesPayload = []byte(body)
	case map[string]io.Reader:
		if data.MaxRetries > 1 {
			return Response{}, fmt.Errorf("can't retry a request whose body depends on io.Reader's")
		}

		form, contentType, err := newMultipartStream(MultipartData(body))
		if err != nil {
			return Response{}, fmt.Errorf("error building multipart data: %v", err)
		}
		data.Headers["Content-Type"] = contentType
		requestBody = form
	default:
		bytesPayload, err = json.Marshal(data.Body)
		if err != nil {
			return Response{}, err
		}
	}

	httpClient := http.Client{
		Timeout:   c.timeout,
		Transport: c.transports.get(data.TLSConfig),
		// Don't follow redirects by default:
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	if data.FollowRedirects {
		// Restore the default http.Client behavior of
		// following redirects up to 10 times:
		httpClient.CheckRedirect = nil
	}

	var resp *http.Response
	Retry(ctx, data.BaseRetryDelay, data.MaxRetryDelay, data.MaxRetries, func() bool {
		if bytesPayload != nil {
			requestBody = bytes.NewReader(bytesPayload)
		}

		var req *http.Request
		req, err = http.NewRequestWithContext(ctx, method, url, requestBody)
		if err != nil {
			return true
		}

		for k, value := range data.Headers {
			switch v := value.(type) {
			case string:
				req.Header.Set(k, v)
			case []string:
				req.Header[k] = v
			default:
				err = fmt.Errorf("header of invalid type received for key '%s': %T", k, v)
				return false
			}
		}

		resp, err = httpClient.Do(req)
		return data.RetryRule(resp, err)
	})
	if err != nil {
		return Response{}, err
	}

	isStatusSuccess := (resp.StatusCode >= 200 && resp.StatusCode < 300)

	var body []byte
	bodyReader := io.ReadCloser(resp.Body)
	if !data.Stream || !isStatusSuccess {
		body, err = io.ReadAll(resp.Body)
		err = errors.Join(err, resp.Body.Close())
		bodyReader = io.NopCloser(bytes.NewReader(body))
	}

	if err == nil && !isStatusSuccess {
		err = fmt.Errorf(
			"%s %s: unexpected status code: %d, payload: %s",
			method, url, resp.StatusCode, string(body),
		)
	}

	return Response{
		ReadCloser: bodyReader,
		Body:       body,
		Headers:    resp.Header,
		StatusCode: resp.StatusCode,
	}, err
}
