//go:build linux || dragonfly || freebsd || netbsd || openbsd || darwin

// Copyright 2020 Kentaro Hibino. All rights reserved.
// Use of this source code is governed by a MIT license
// that can be found in the LICENSE file.

package asynq

import (
	"go.uber.org/goleak"
	"syscall"
	"testing"
	"time"
)

func TestServerRun(t *testing.T) {
	// https://github.com/go-redis/redis/issues/1029
	ignorePoolReaper := goleak.IgnoreTopFunction("github.com/redis/go-redis/v9/internal/pool.(*ConnPool).reaper")
	ignoreCircuitBreakerCleanup := goleak.IgnoreTopFunction("github.com/redis/go-redis/v9/maintnotifications.(*CircuitBreakerManager).cleanupLoop")
	defer goleak.VerifyNone(t, ignorePoolReaper, ignoreCircuitBreakerCleanup)

	srv := NewServer(getRedisConnOpt(t), Config{LogLevel: testLogLevel})

	done := make(chan struct{})
	// Make sure server exits when receiving TERM signal.
	go func() {
		time.Sleep(2 * time.Second)
		_ = syscall.Kill(syscall.Getpid(), syscall.SIGTERM)
		done <- struct{}{}
	}()

	go func() {
		select {
		case <-time.After(10 * time.Second):
			panic("server did not stop after receiving TERM signal")
		case <-done:
		}
	}()

	mux := NewServeMux()
	if err := srv.Run(mux); err != nil {
		t.Fatal(err)
	}
}
