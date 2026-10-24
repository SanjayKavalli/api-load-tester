package transport

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

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
