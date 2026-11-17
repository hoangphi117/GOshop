// Package metrics exposes the per-process Prometheus registry.
//
// Every service mounts GET /metrics on its own gin router
// (metrics.Handler(nil)), which is all a single scrape target needs. A second
// listener exists for the processes Prometheus cannot reach on their app port:
// cmd/worker serves no HTTP at all, so without ListenAndServe its /healthz
// counters and consumer lag would be unpublishable. It is also the reason the
// compose files publish a metrics port: an unpublished port is invisible to the
// docker service discovery in system/prometheus/prometheus.yml.
//
// The endpoint is unauthenticated and MUST NOT be reachable from the internet:
// it carries internal topic names and consumer group names. The edge only
// proxies explicit paths, and this one is not among them.
package metrics

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
)

// Handler returns the scrape handler for a registry. Pass nil for the default
// registry, which is what promauto-registered collectors (shared/pkg/kafka)
// write to. promhttp does not accept a nil Gatherer, so nil is normalised here
// rather than at every call site.
func Handler(reg prometheus.Gatherer) gin.HandlerFunc {
	if reg == nil {
		reg = prometheus.DefaultGatherer
	}
	return gin.WrapH(promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
}

// Register mounts GET /metrics on r. Call it exactly once per router: gin
// panics on a duplicate route registration.
func Register(r *gin.Engine) {
	r.GET("/metrics", Handler(prometheus.DefaultGatherer))
}

// ListenAndServe starts a /metrics listener on METRICS_PORT (default 9464).
// It returns nil when the env var is empty, so a process that does not want the
// port stays silent; flag the difference if you need it to be fatal.
//
// The server is killed by ctx; the returned error channel reports fatal errors
// only (a normal shutdown is not an error).
func ListenAndServe(ctx context.Context, logger zerolog.Logger) (<-chan error, error) {
	port := os.Getenv("METRICS_PORT")
	if port == "" {
		return nil, nil
	}

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(prometheus.DefaultGatherer, promhttp.HandlerOpts{}))

	ln, err := net.Listen("tcp", ":"+port)
	if err != nil {
		return nil, fmt.Errorf("metrics: listen on :%s: %w", port, err)
	}

	srv := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	errc := make(chan error, 1)
	go func() {
		logger.Info().Str("addr", ln.Addr().String()).Msg("metrics listening")
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errc <- err
		}
		close(errc)
	}()

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	return errc, nil
}
