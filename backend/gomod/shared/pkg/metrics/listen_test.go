package metrics_test

import (
	"context"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"

	"goshop/shared/pkg/metrics"
)

// The listener is what makes the worker scrapeable at all, and the compose
// files publish 9464 unconditionally, so "bind nothing and stay silent" would
// show up as a permanently down target rather than a startup failure. Assert
// both halves: empty env = no listener, set env = a real /metrics.
func TestListenAndServe_EmptyEnvIsNoop(t *testing.T) {
	t.Setenv("METRICS_PORT", "")

	errc, err := metrics.ListenAndServe(context.Background(), zerolog.Nop())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if errc != nil {
		t.Fatalf("errc = %v, want nil", errc)
	}
}

func TestListenAndServe_ServesMetricsAndStopsOnCancel(t *testing.T) {
	port := freePort(t)
	t.Setenv("METRICS_PORT", port)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errc, err := metrics.ListenAndServe(ctx, zerolog.Nop())
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	if errc == nil {
		t.Fatal("errc = nil, want a channel")
	}

	body := get(t, "http://127.0.0.1:"+port+"/metrics")
	if !strings.Contains(body, "go_goroutines") {
		t.Fatalf("/metrics body missing default collectors")
	}

	cancel()
	select {
	case err, ok := <-errc:
		if ok && err != nil {
			t.Fatalf("shutdown reported an error: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("listener did not stop within 10s of cancel")
	}

	if conn, err := net.DialTimeout("tcp", "127.0.0.1:"+port, 200*time.Millisecond); err == nil {
		_ = conn.Close()
		t.Fatal("port still accepting connections after cancel")
	}
}

func freePort(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("pick port: %v", err)
	}
	defer func() { _ = ln.Close() }()
	_, port, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		t.Fatalf("split: %v", err)
	}
	return port
}

func get(t *testing.T, url string) string {
	t.Helper()
	var lastErr error
	for i := 0; i < 50; i++ {
		resp, err := http.Get(url) //nolint:noctx // test helper, bounded by the loop
		if err == nil {
			defer func() { _ = resp.Body.Close() }()
			buf := make([]byte, 1<<16)
			n, _ := resp.Body.Read(buf)
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("status = %d, want 200", resp.StatusCode)
			}
			return string(buf[:n])
		}
		lastErr = err
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("GET %s never succeeded: %v", url, lastErr)
	return ""
}
