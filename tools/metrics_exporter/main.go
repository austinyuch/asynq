package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
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

func main() {
	flag.Parse()
	// Using NewPedanticRegistry here to test the implementation of Collectors and Metrics.
	reg := prometheus.NewPedanticRegistry()

	inspector := asynq.NewInspector(asynq.RedisClientOpt{
		Addr:     flagRedisAddr,
		DB:       flagRedisDB,
		Password: flagRedisPassword,
		Username: flagRedisUsername,
	})

	reg.MustRegister(
		metrics.NewQueueMetricsCollector(inspector),
		// Add the standard process and go metrics to the registry
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		collectors.NewGoCollector(),
	)

	http.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
	log.Printf("exporter server is listening on port: %d\n", flagPort)

	// http.ListenAndServe applies no timeouts, which leaves the exporter open to
	// slow-header (Slowloris) clients holding connections indefinitely. Scrape
	// requests are short, so modest timeouts are safe here.
	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", flagPort),
		Handler:           http.DefaultServeMux,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	log.Fatal(srv.ListenAndServe())
}
