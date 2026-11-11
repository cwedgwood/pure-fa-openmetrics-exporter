package client

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// Collectors registered on one prometheus.Registry share an FAClient and run
// concurrently, so a 401 in one collector refreshes the session while others
// are sending requests.
func TestConcurrentRefreshSession(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/api_version":
			w.Write([]byte(`{"version":["2.0"]}`))
		case "/api/2.0/login":
			w.Header().Set("x-auth-token", "test-token")
			w.Write([]byte(`{}`))
		case "/api/2.0/ports":
			if r.Header.Get("x-auth-token") != "test-token" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			w.Write([]byte(`{"items":[{"name":"test-port"}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	c := NewRestClient(strings.TrimPrefix(server.URL, "https://"), "test-api-token", "latest", "test", "test-request-id", false, false)
	if c.Error != nil {
		t.Fatal(c.Error)
	}

	var wg sync.WaitGroup
	start := make(chan struct{})
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			for range 20 {
				c.RefreshSession()
				if ports := c.GetPorts(); len(ports.Items) != 1 {
					t.Errorf("expected one port, got %d", len(ports.Items))
				}
			}
		}()
	}
	close(start)
	wg.Wait()
	c.Close()
}
