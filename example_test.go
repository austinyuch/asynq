// Copyright 2020 Kentaro Hibino. All rights reserved.
// Use of this source code is governed by a MIT license
// that can be found in the LICENSE file.

package asynq_test

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/austinyuch/asynq"
)

func ExampleServer_Run() {
	srv := asynq.NewServer(
		asynq.RedisClientOpt{Addr: ":6379"},
		asynq.Config{Concurrency: 20},
	)

	h := asynq.NewServeMux()
	// ... Register handlers

	// Run blocks and waits for os signal to terminate the program.
	if err := srv.Run(h); err != nil {
		log.Fatal(err)
	}
}

func ExampleScheduler() {
	scheduler := asynq.NewScheduler(
		asynq.RedisClientOpt{Addr: ":6379"},
		&asynq.SchedulerOpts{Location: time.Local},
	)

	if _, err := scheduler.Register("* * * * *", asynq.NewTask("task1", nil)); err != nil {
		log.Fatal(err)
	}
	if _, err := scheduler.Register("@every 30s", asynq.NewTask("task2", nil)); err != nil {
		log.Fatal(err)
	}

	// Run blocks and waits for os signal to terminate the program.
	if err := scheduler.Run(); err != nil {
		log.Fatal(err)
	}
}

func ExampleParseRedisURI() {
	rconn, err := asynq.ParseRedisURI("redis://localhost:6379/10")
	if err != nil {
		log.Fatal(err)
	}
	r, ok := rconn.(asynq.RedisClientOpt)
	if !ok {
		log.Fatal("unexpected type")
	}
	fmt.Println(r.Addr)
	fmt.Println(r.DB)
	// Output:
	// localhost:6379
	// 10
}

func ExampleResultWriter() {
	// ResultWriter is only accessible in Handler.
	h := func(ctx context.Context, task *asynq.Task) error {
		// .. do task processing work

		res := []byte("task result data")
		n, err := task.ResultWriter().Write(res) // implements io.Writer
		if err != nil {
			return fmt.Errorf("failed to write task result: %w", err)
		}
		log.Printf(" %d bytes written", n)
		return nil
	}

	_ = h
}
