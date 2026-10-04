package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/austinyuch/asynq"
	"github.com/austinyuch/asynq/x/metrics"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Declare command-line flags.
// These variables are binded to flags in init().
var (
	flagRedisAddr     string
	flagRedisDB       int
	flagRedisPassword string
	flagRedisUsername string
	flagPort          int
)

func init() {
	flag.StringVar(&flagRedisAddr, "redis-addr", "127.0.0.1:6379", "host:port of redis server to connect to")
	flag.IntVar(&flagRedisDB, "redis-db", 0, "redis DB number to use")
	flag.StringVar(&flagRedisPassword, "redis-password", "", "password used to connect to redis server")
	flag.StringVar(&flagRedisUsername, "redis-username", "", "username used to connect to redis server")
	flag.IntVar(&flagPort, "port", 9876, "port to use for the HTTP server")
}

// Each instance owns its registry, HTTP server and Redis transport.
// An instance is served once; create a new one for another run.
type exporter struct {
	server          *http.Server
	inspector       *asynq.Inspector
	shutdownTimeout time.Duration
}

func newExporter(redisOpt asynq.RedisClientOpt, addr string) *exporter {
	reg := prometheus.NewPedanticRegistry()
	inspector := asynq.NewInspector(redisOpt)
	reg.MustRegister(
		metrics.NewQueueMetricsCollector(inspector),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		collectors.NewGoCollector(),
	)
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
	return &exporter{
		inspector:       inspector,
		shutdownTimeout: 5 * time.Second,
		server: &http.Server{
			Addr:              addr,
			Handler:           mux,
			ReadHeaderTimeout: 10 * time.Second,
			ReadTimeout:       30 * time.Second,
			WriteTimeout:      60 * time.Second,
			IdleTimeout:       120 * time.Second,
		},
	}
}

func (e *exporter) run(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		_ = e.inspector.Close()
		return err
	}
	listener, err := net.Listen("tcp", e.server.Addr)
	if err != nil {
		_ = e.inspector.Close()
		return fmt.Errorf("listen metrics: %w", err)
	}
	log.Printf("exporter server is listening on %s", listener.Addr())
	return e.serve(ctx, listener)
}

// serve takes ownership of the listener and Inspector. Successful Shutdown is
// the active-handler completion barrier; Serve returning alone is not one.
func (e *exporter) serve(ctx context.Context, listener net.Listener) error {
	defer e.inspector.Close()
	defer listener.Close()
	if err := ctx.Err(); err != nil {
		return err
	}
	served := make(chan error, 1)
	go func() { served <- e.server.Serve(listener) }()
	var serveErr error
	serveReturned := false
	select {
	case serveErr = <-served:
		serveReturned = true
	case <-ctx.Done():
	}
	// Serve can return while accepted handlers are still active. Apply the
	// same completion barrier to cancellation and unexpected accept errors.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), e.shutdownTimeout)
	shutdownErr := e.server.Shutdown(shutdownCtx)
	cancel()
	if shutdownErr != nil {
		_ = e.server.Close()
	}
	if !serveReturned {
		serveErr = <-served
	}
	if errors.Is(serveErr, http.ErrServerClosed) {
		serveErr = nil
	} else {
		serveErr = fmt.Errorf("serve metrics: %w", serveErr)
	}
	if shutdownErr != nil {
		// Close terminates connections, not every handler goroutine. The
		// source is closed by defer, and the timeout remains observable.
		shutdownErr = fmt.Errorf("shutdown metrics: %w", shutdownErr)
	}
	return errors.Join(serveErr, shutdownErr)
}

func main() {
	flag.Parse()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	e := newExporter(asynq.RedisClientOpt{
		Addr:     flagRedisAddr,
		DB:       flagRedisDB,
		Password: flagRedisPassword,
		Username: flagRedisUsername,
	}, fmt.Sprintf(":%d", flagPort))
	if err := e.run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		log.Print(err)
		os.Exit(1)
	}
}
