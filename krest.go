package krest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client contains methods for making rest requests
// these methods accept any struct that can be marshaled into JSON
// but the response is returned in Bytes, since not all APIs follow
// rest strictly.
type Client struct {
	timeout     time.Duration
	middlewares []Middleware
}

// New instantiates a new rest client
func New(timeout time.Duration, middlewares ...Middleware) Client {
	return Client{
		timeout:     timeout,
		middlewares: middlewares,
	}
}

// AddMiddleware adds one or more new middlewares to this instance
func (c *Client) AddMiddleware(middlewares ...Middleware) {
	c.middlewares = append(c.middlewares, middlewares...)
}

// Get will make a GET request to the input URL
// and return the results
func (c Client) Get(ctx context.Context, url string, data RequestData) (Response, error) {
	return c.makeRequestWithMiddlewares(ctx, "GET", url, data)
}

// Post will make a POST request to the input URL
// and return the results
func (c Client) Post(ctx context.Context, url string, data RequestData) (Response, error) {
	return c.makeRequestWithMiddlewares(ctx, "POST", url, data)
}

// Put will make a PUT request to the input URL
// and return the results
func (c Client) Put(ctx context.Context, url string, data RequestData) (Response, error) {
	return c.makeRequestWithMiddlewares(ctx, "PUT", url, data)
}

// Patch will make a PATCH request to the input URL
// and return the results
func (c Client) Patch(ctx context.Context, url string, data RequestData) (Response, error) {
	return c.makeRequestWithMiddlewares(ctx, "PATCH", url, data)
}

// Delete will make a DELETE request to the input URL
// and return the results
func (c Client) Delete(ctx context.Context, url string, data RequestData) (Response, error) {
	return c.makeRequestWithMiddlewares(ctx, "DELETE", url, data)
}

func (c Client) makeRequestWithMiddlewares(
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

func (c Client) makeRequest(
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
		Timeout: c.timeout,
		Transport: &http.Transport{
			TLSClientConfig: data.TLSConfig,
		},
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
		resp.Body.Close()
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
		Header:     resp.Header,
		StatusCode: resp.StatusCode,
	}, err
}
