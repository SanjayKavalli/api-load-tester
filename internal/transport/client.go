package transport

import (
	"io"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	HTTP  *http.Client
	Token string
}

func New(timeout time.Duration, maxInFlight int, token string) *Client {
	return &Client{
		Token: token,
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
	start := time.Now()
	req, err := http.NewRequest(method, url, strings.NewReader(string(body)))
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

	// Bound response memory usage to 2 MiB per response.
	const limit = 2 << 20
	responseBody, readErr := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	truncated := len(responseBody) > limit
	if truncated {
		responseBody = responseBody[:limit]
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
