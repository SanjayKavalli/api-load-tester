package transport

import (
	"context"
	"io"
	"net/http"
	"strings"
	"time"
)

const DefaultMaxResponseBodyBytes = 2 << 20

type Client struct {
	HTTP                 *http.Client
	Token                string
	maxResponseBodyBytes int
}

func New(timeout time.Duration, maxInFlight int, token string) *Client {
	return NewWithResponseLimit(timeout, maxInFlight, token, DefaultMaxResponseBodyBytes)
}

func NewWithResponseLimit(timeout time.Duration, maxInFlight int, token string, maxResponseBodyBytes int) *Client {
	if maxResponseBodyBytes < 0 {
		maxResponseBodyBytes = 0
	}
	return &Client{
		Token:                token,
		maxResponseBodyBytes: maxResponseBodyBytes,
		HTTP: &http.Client{
			Timeout:       timeout,
			CheckRedirect: func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse },
			Transport: &http.Transport{
				MaxIdleConns:        maxInFlight,
				MaxIdleConnsPerHost: maxInFlight,
				MaxConnsPerHost:     maxInFlight,
				IdleConnTimeout:     90 * time.Second,
			},
		},
	}
}

type Response struct {
	StatusCode        int
	ContentType       string
	Body              []byte
	Duration          time.Duration
	Err               error
	Truncated         bool
	DeclaredBodyBytes int64
}

func (c *Client) Do(method, url string, headers map[string]string, body []byte) Response {
	return c.DoContext(context.Background(), method, url, headers, body)
}

func (c *Client) DoContext(ctx context.Context, method, url string, headers map[string]string, body []byte) Response {
	start := time.Now()
	req, err := http.NewRequestWithContext(ctx, method, url, strings.NewReader(string(body)))
	if err != nil {
		return Response{Duration: time.Since(start), Err: err}
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}
	if req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.Token != "" && req.Header.Get("Authorization") == "" {
		token := strings.TrimSpace(c.Token)
		if !strings.HasPrefix(strings.ToLower(token), "bearer ") {
			token = "Bearer " + token
		}
		req.Header.Set("Authorization", token)
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return Response{Duration: time.Since(start), Err: err}
	}
	defer resp.Body.Close()

	responseBody, readErr := io.ReadAll(io.LimitReader(resp.Body, int64(c.maxResponseBodyBytes)+1))
	truncated := len(responseBody) > c.maxResponseBodyBytes
	if truncated {
		responseBody = responseBody[:c.maxResponseBodyBytes]
	}

	return Response{
		StatusCode:        resp.StatusCode,
		ContentType:       resp.Header.Get("Content-Type"),
		Body:              responseBody,
		Duration:          time.Since(start),
		Err:               readErr,
		Truncated:         truncated,
		DeclaredBodyBytes: resp.ContentLength,
	}
}
