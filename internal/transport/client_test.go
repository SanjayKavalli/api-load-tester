package transport

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestRedirectIsReportedWithoutChangingMethod(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/target" {
			called = true
		}
		http.Redirect(w, r, "/target", http.StatusFound)
	}))
	defer server.Close()
	response := New(time.Second, 1, "").Do("POST", server.URL, nil, []byte(`{}`))
	if response.StatusCode != 302 || called {
		t.Fatal("redirect changed the tested request", response.StatusCode, called)
	}
}

func TestTransportReportsTruncation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(strings.Repeat("x", 3<<20))) }))
	defer server.Close()
	response := New(time.Second, 1, "").Do("GET", server.URL, nil, nil)
	if response.Err != nil || !response.Truncated || len(response.Body) != 2<<20 {
		t.Fatal("truncation was not surfaced", response.Err, response.Truncated, len(response.Body))
	}
}

func TestTransportResponseBodyLimitIsConfigurable(t *testing.T) {
	const limit = 32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(strings.Repeat("x", limit+10)))
	}))
	defer server.Close()

	response := NewWithResponseLimit(time.Second, 1, "", limit).Do(http.MethodGet, server.URL, nil, nil)
	if response.Err != nil || !response.Truncated || len(response.Body) != limit {
		t.Fatalf("response = (err %v, truncated %t, body %d bytes), want (nil, true, %d bytes)", response.Err, response.Truncated, len(response.Body), limit)
	}
}

func TestDoContextCancelsInFlightRequest(t *testing.T) {
	started := make(chan struct{})
	canceled := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-r.Context().Done()
		close(canceled)
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := make(chan Response, 1)
	go func() {
		result <- New(time.Minute, 1, "").DoContext(ctx, http.MethodGet, server.URL, nil, nil)
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("request did not reach server")
	}
	cancel()
	select {
	case response := <-result:
		if !errors.Is(response.Err, context.Canceled) {
			t.Fatalf("DoContext error = %v, want context cancellation", response.Err)
		}
	case <-time.After(time.Second):
		t.Fatal("in-flight request did not stop after cancellation")
	}
	select {
	case <-canceled:
	case <-time.After(time.Second):
		t.Fatal("server did not observe request cancellation")
	}
}

func TestDoIncludesResponseBodyAndContentType(t *testing.T) {
	const responseBody = `{"is_error":false,"message":"ok"}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(responseBody))
	}))
	defer server.Close()

	client := New(time.Second, 1, "")
	response := client.Do(http.MethodGet, server.URL, nil, nil)
	if response.Err != nil {
		t.Fatalf("Do() returned error: %v", response.Err)
	}
	if response.StatusCode != http.StatusCreated {
		t.Errorf("status = %d, want %d", response.StatusCode, http.StatusCreated)
	}
	if response.ContentType != "application/json" {
		t.Errorf("content type = %q, want application/json", response.ContentType)
	}
	if string(response.Body) != responseBody {
		t.Errorf("body = %q, want %q", response.Body, responseBody)
	}
}

func TestDoNormalizesGlobalBearerToken(t *testing.T) {
	prefix := "Bear" + "er "
	for _, token := range []string{"test-token", prefix + "test-token"} {
		t.Run(token, func(t *testing.T) {
			var authorization string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				authorization = r.Header.Get("Authorization")
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			client := New(time.Second, 1, token)
			response := client.Do(http.MethodGet, server.URL, nil, nil)
			if response.Err != nil {
				t.Fatalf("Do() returned error: %v", response.Err)
			}
			if authorization != prefix+"test-token" {
				t.Errorf("Authorization = %q, want normalized token", authorization)
			}
		})
	}
}
