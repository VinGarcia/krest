package krest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"time"
)

// Client contains methods for making rest requests
// these methods accept any struct that can be marshaled into JSON
// but the response is returned in Bytes, since not all APIs follow
// rest strictly.
type Client struct {
	http        http.Client
	middlewares []Middleware
}

// New instantiates a new rest client
func New(timeout time.Duration, middlewares ...Middleware) Client {
	return Client{
		http: http.Client{
			Timeout: timeout,
		},
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
	var i = -1 // (It will be incremented to 0 on first use)

	// Each time this next() function is called
	// the next middleware is executed until there are no
	// middlewares left, then we run makeRequest()
	var nextMiddleware NextMiddleware
	nextMiddleware = func(
		ctx context.Context,
		method string,
		url string,
		data RequestData,
	) (Response, error) {
		i++
		if i < len(c.middlewares) {
			return c.middlewares[i](ctx, method, url, data, nextMiddleware)
		}

		return c.makeRequest(ctx, method, url, data)
	}

	return nextMiddleware(ctx, method, url, data)
}

func (c Client) makeRequest(
	ctx context.Context,
	method string,
	url string,
	data RequestData,
) (_ Response, err error) {
	data.SetDefaultsIfNecessary()

	var requestBody io.Reader
	switch body := data.Body.(type) {
	case nil:
		requestBody = nil
	case io.Reader:
		if data.MaxRetries > 1 {
			return Response{}, fmt.Errorf("can't retry a request whose body is an io.Reader!")
		}

		requestBody = body
	case []byte:
		requestBody = bytes.NewReader(body)
	case string:
		requestBody = strings.NewReader(body)
	case MultipartData:
		if data.MaxRetries > 1 {
			return Response{}, fmt.Errorf("can't retry a request whose body depends on io.Reader's!")
		}

		form, contentType, err := newMultipartStream(body)
		if err != nil {
			return Response{}, fmt.Errorf("error building multipart data: %v", err)
		}
		data.Headers["Content-Type"] = contentType
		requestBody = form
	default:
		inputBodyJSON, err := json.Marshal(data.Body)
		if err != nil {
			return Response{}, err
		}
		requestBody = bytes.NewReader(inputBodyJSON)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, requestBody)
	if err != nil {
		return Response{}, err
	}

	for k, v := range data.Headers {
		req.Header.Set(k, v)
	}

	var resp *http.Response
	Retry(ctx, data.BaseRetryDelay, data.MaxRetryDelay, data.MaxRetries, func() bool {
		resp, err = c.http.Do(req)
		return data.RetryRule(resp, err)
	})
	if err != nil {
		return Response{}, err
	}

	header := map[string]string{}
	for k, v := range resp.Header {
		if len(v) == 0 {
			continue
		}
		header[k] = v[0]
	}

	isStatusSuccess := (resp.StatusCode >= 200 && resp.StatusCode < 300)

	var body []byte
	bodyReader := io.ReadCloser(resp.Body)
	if !data.Stream || !isStatusSuccess {
		body, err = ioutil.ReadAll(resp.Body)
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
		Header:     header,
		StatusCode: resp.StatusCode,
	}, err
}
