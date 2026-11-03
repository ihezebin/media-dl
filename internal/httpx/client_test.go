package httpx

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestExplicitProxy(t *testing.T) {
	var hits atomic.Int32
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.URL.Host != "media.invalid" {
			t.Errorf("unexpected destination %s", r.URL.Host)
		}
		_, _ = io.WriteString(w, "proxied")
	}))
	defer proxy.Close()
	c, err := New(Options{Proxy: proxy.URL})
	if err != nil {
		t.Fatal(err)
	}
	body, _, err := c.GetString("http://media.invalid/video", nil)
	if err != nil || body != "proxied" || hits.Load() != 1 {
		t.Fatalf("proxy not used: %q %v", body, err)
	}
}

func TestExtractionBudgetStopsRequestsAndRetries(t *testing.T) {
	for _, direct := range []bool{false, true} {
		t.Run(map[bool]string{false: "retry", true: "direct HTTP client"}[direct], func(t *testing.T) {
			var hits atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				hits.Add(1)
				if direct {
					<-r.Context().Done()
					return
				}
				w.WriteHeader(503)
			}))
			defer server.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()
			c, err := New(Options{Context: ctx})
			if err != nil {
				t.Fatal(err)
			}
			start := time.Now()
			if direct {
				_, err = c.HTTP().Get(server.URL)
			} else {
				_, err = c.Get(server.URL, nil)
			}
			if err == nil || time.Since(start) > time.Second || hits.Load() != 1 {
				t.Fatalf("budget ignored: %v, hits=%d", err, hits.Load())
			}
		})
	}
}

func TestCallerCancellationStopsResponseBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}))
	defer server.Close()
	c, err := New(Options{})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", server.URL, nil)
	resp, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	cancel()
	if _, err := io.ReadAll(resp.Body); err == nil {
		t.Fatal("response body ignored cancellation")
	}
}
