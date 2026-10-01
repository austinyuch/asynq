package asynq

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"testing"
	"testing/quick"
	"time"

	"github.com/redis/go-redis/v9"
)

// lifecycleProvider exercises the public manager admission boundary without a
// timer-driven mutable provider or a replacement scheduler implementation.
type lifecycleProvider struct {
	configs []*PeriodicTaskConfig
	err     error
}

func (p lifecycleProvider) GetConfigs() ([]*PeriodicTaskConfig, error) { return p.configs, p.err }

func lifecycleRedis(t testing.TB) *redis.Client {
	t.Helper()
	c := redis.NewClient(&redis.Options{Addr: redisAddr, DB: redisDB})
	t.Cleanup(func() { _ = c.Close() })
	return c
}

func TestLifecycleServerTransitions(t *testing.T) {
	c := lifecycleRedis(t)
	s := NewServerFromRedisClient(c, Config{Concurrency: 1, LogLevel: FatalLevel, Queues: map[string]int{"lifecycle-contract": 1}, TaskCheckInterval: 10 * time.Millisecond})
	s.Stop()
	s.Shutdown() // documented no-ops before startup must leave it usable.
	if err := s.Start(nil); err == nil {
		t.Fatal("nil handler accepted")
	}
	if err := s.Ping(); err != nil {
		t.Fatal(err)
	}
	h := HandlerFunc(func(context.Context, *Task) error { return nil })
	if err := s.Start(h); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Shutdown)
	if err := s.Start(h); err == nil {
		t.Fatal("second start accepted")
	}
	s.Stop()
	s.Stop()
	if err := s.Start(h); err == nil {
		t.Fatal("stopped server restarted")
	}
	s.Shutdown()
	s.Shutdown()
	if err := s.Start(h); !errors.Is(err, ErrServerClosed) {
		t.Fatalf("closed start: %v", err)
	}
	if err := s.Run(h); !errors.Is(err, ErrServerClosed) {
		t.Fatalf("closed run: %v", err)
	}
	if err := s.Ping(); err != nil {
		t.Fatalf("closed server ping: %v", err)
	}
	if err := c.Ping(context.Background()).Err(); err != nil {
		t.Fatalf("shared client closed: %v", err)
	}
}

func TestLifecycleSchedulerTransitions(t *testing.T) {
	c := lifecycleRedis(t)
	s := NewSchedulerFromRedisClient(c, &SchedulerOpts{LogLevel: FatalLevel})
	s.Shutdown()
	if err := s.Ping(); err != nil {
		t.Fatal(err)
	}
	if err := s.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Shutdown)
	if err := s.Start(); err == nil {
		t.Fatal("second scheduler start accepted")
	}
	s.Shutdown()
	s.Shutdown()
	if err := s.Start(); err == nil {
		t.Fatal("closed scheduler restarted")
	}
	if err := s.Run(); err == nil {
		t.Fatal("closed scheduler run accepted")
	}
	if err := s.Ping(); err != nil {
		t.Fatal(err)
	}
	if err := c.Ping(context.Background()).Err(); err != nil {
		t.Fatalf("shared scheduler client closed: %v", err)
	}
}

func TestLifecycleClosedConnectionContracts(t *testing.T) {
	c := lifecycleRedis(t)
	client := NewClientFromRedisClient(c)
	inspector := NewInspectorFromRedisClient(c)
	server := NewServerFromRedisClient(c, Config{Concurrency: 1, LogLevel: FatalLevel})
	scheduler := NewSchedulerFromRedisClient(c, &SchedulerOpts{LogLevel: FatalLevel})
	if err := client.Ping(); err != nil {
		t.Fatal(err)
	}
	if _, err := inspector.Queues(); err != nil {
		t.Fatal(err)
	}
	if err := client.Close(); err == nil {
		t.Fatal("shared client close accepted")
	}
	if err := inspector.Close(); err == nil {
		t.Fatal("shared inspector close accepted")
	}
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	for name, ping := range map[string]func() error{"client": client.Ping, "server": server.Ping, "scheduler": scheduler.Ping} {
		if err := ping(); !errors.Is(err, redis.ErrClosed) {
			t.Errorf("%s: expected closed transport cause, got %v", name, err)
		}
	}
	if _, err := inspector.Queues(); !errors.Is(err, redis.ErrClosed) {
		t.Fatalf("inspector lost closed cause: %v", err)
	}
}

func TestLifecyclePeriodicAdmission(t *testing.T) {
	c := lifecycleRedis(t)
	sentinel := errors.New("provider unavailable")
	for _, tc := range []struct {
		name     string
		provider PeriodicTaskConfigProvider
		want     error
	}{
		{"nil_config", lifecycleProvider{configs: []*PeriodicTaskConfig{nil}}, nil},
		{"nil_task", lifecycleProvider{configs: []*PeriodicTaskConfig{{Cronspec: "@hourly"}}}, nil},
		{"empty_cron", lifecycleProvider{configs: []*PeriodicTaskConfig{{Task: NewTask("contract", nil)}}}, nil},
		{"provider_error", lifecycleProvider{err: sentinel}, sentinel},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, err := NewPeriodicTaskManager(PeriodicTaskManagerOpts{PeriodicTaskConfigProvider: tc.provider, RedisUniversalClient: c, SchedulerOpts: &SchedulerOpts{LogLevel: FatalLevel}})
			if err != nil {
				t.Fatal(err)
			}
			if err = m.Start(); err == nil {
				m.Shutdown()
				t.Fatal("invalid initial configuration admitted")
			} else if tc.want != nil && !errors.Is(err, tc.want) {
				t.Fatalf("provider cause lost: %v", err)
			}
		})
	}
	if _, err := NewPeriodicTaskManager(PeriodicTaskManagerOpts{}); err == nil {
		t.Fatal("nil provider admitted")
	}
	if _, err := NewPeriodicTaskManager(PeriodicTaskManagerOpts{PeriodicTaskConfigProvider: lifecycleProvider{}}); err == nil {
		t.Fatal("missing Redis admitted")
	}
	m, err := NewPeriodicTaskManager(PeriodicTaskManagerOpts{PeriodicTaskConfigProvider: lifecycleProvider{configs: []*PeriodicTaskConfig{{Cronspec: "@hourly", Task: NewTask("contract", nil)}}}, RedisUniversalClient: c, SchedulerOpts: &SchedulerOpts{LogLevel: FatalLevel}, SyncInterval: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	if err = m.Start(); err != nil {
		t.Fatal(err)
	}
	m.Shutdown() // manager's contract permits one Shutdown; never close done twice.
}

func checkLifecycleConfig(t testing.TB, priority int8, concurrency uint8) {
	t.Helper()
	c := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"})
	defer func() { _ = c.Close() }()
	n := int(concurrency % 8)
	s := NewServerFromRedisClient(c, Config{Concurrency: n, Queues: map[string]int{"valid": int(priority), " ": 7}, LogLevel: FatalLevel})
	wantConcurrency := n
	if n == 0 {
		wantConcurrency = runtime.NumCPU()
	}
	if cap(s.processor.sema) != wantConcurrency {
		t.Fatalf("concurrency %d => %d", n, cap(s.processor.sema))
	}
	if priority > 0 {
		if len(s.processor.queueConfig) != 1 || s.processor.queueConfig["valid"] != 1 {
			t.Fatalf("single valid queue not normalized: %v", s.processor.queueConfig)
		}
	} else {
		if len(s.processor.queueConfig) != 1 || s.processor.queueConfig["default"] != 1 {
			t.Fatalf("empty admissible queue set did not default: %v", s.processor.queueConfig)
		}
	}
}

func TestLifecycleConfigProperties(t *testing.T) {
	if err := quick.Check(func(p int8, n uint8) bool { checkLifecycleConfig(t, p, n); return true }, &quick.Config{MaxCount: 40, Rand: rand.New(rand.NewSource(20261001))}); err != nil {
		t.Fatal(err)
	}
	for _, d := range []time.Duration{-time.Second, time.Nanosecond, time.Second - time.Nanosecond} {
		t.Run(fmt.Sprint(d), func(t *testing.T) {
			c := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"})
			defer func() { _ = c.Close() }()
			defer func() {
				if recover() == nil {
					t.Fatal("subsecond group grace accepted")
				}
			}()
			NewServerFromRedisClient(c, Config{GroupGracePeriod: d, LogLevel: FatalLevel})
		})
	}
}

func FuzzLifecycleConfigInvariant(f *testing.F) {
	f.Add(int8(1), uint8(1))
	f.Add(int8(-1), uint8(0))
	f.Add(int8(0), uint8(255))
	f.Fuzz(func(t *testing.T, p int8, n uint8) { checkLifecycleConfig(t, p, n) })
}

func TestLifecycleOwnedConnectionClose(t *testing.T) {
	opt := RedisClientOpt{Addr: redisAddr, DB: redisDB}
	c := NewClient(opt)
	if err := c.Ping(); err != nil {
		_ = c.Close()
		t.Fatal(err)
	}
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	if err := c.Ping(); !errors.Is(err, redis.ErrClosed) {
		t.Fatalf("owned client close did not close transport: %v", err)
	}
	i := NewInspector(opt)
	if _, err := i.Queues(); err != nil {
		_ = i.Close()
		t.Fatal(err)
	}
	if err := i.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := i.Queues(); !errors.Is(err, redis.ErrClosed) {
		t.Fatalf("owned inspector close did not close transport: %v", err)
	}
}

func TestLifecycleLogLevelAdmission(t *testing.T) {
	for _, level := range []LogLevel{DebugLevel, InfoLevel, WarnLevel, ErrorLevel, FatalLevel} {
		c := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"})
		s := NewServerFromRedisClient(c, Config{Concurrency: 1, LogLevel: level})
		if s == nil {
			t.Fatal("valid log level rejected")
		}
		_ = c.Close()
	}
	for _, level := range []LogLevel{-1, 100} {
		t.Run(fmt.Sprint(level), func(t *testing.T) {
			c := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"})
			defer func() { _ = c.Close() }()
			defer func() {
				if recover() == nil {
					t.Fatal("invalid log level accepted")
				}
			}()
			NewServerFromRedisClient(c, Config{Concurrency: 1, LogLevel: level})
		})
	}
}

// Signal delivery is confined to a fresh copy of the test executable. The
// parent process never installs signal handlers or receives a test signal.
type lifecycleSignalLogger struct {
	ready chan struct{}
	once  sync.Once
}

func (*lifecycleSignalLogger) Debug(...interface{}) {}
func (l *lifecycleSignalLogger) Info(args ...interface{}) {
	if strings.Contains(fmt.Sprint(args...), "Send signal TERM or INT") {
		l.once.Do(func() { close(l.ready) })
	}
}
func (*lifecycleSignalLogger) Warn(...interface{})  {}
func (*lifecycleSignalLogger) Error(...interface{}) {}
func (*lifecycleSignalLogger) Fatal(...interface{}) {}

func TestLifecycleRunSignals(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("bounded Linux signal subprocess contract")
	}
	if mode := os.Getenv("ASYNQ_LIFECYCLE_SIGNAL_CHILD"); mode != "" {
		logger := &lifecycleSignalLogger{ready: make(chan struct{})}
		// This child-only guard prevents default termination during the gap
		// between the public readiness log and Run's own signal.Notify. It
		// never forwards signals: Run must observe a separately delivered signal.
		guard := make(chan os.Signal, 1)
		signal.Notify(guard, syscall.SIGTERM, syscall.SIGINT)
		defer signal.Stop(guard)
		done := make(chan struct{})
		senderStopped := make(chan struct{})
		signalErrors := make(chan error, 1)
		go func() {
			defer close(senderStopped)
			select {
			case <-logger.ready:
			case <-done:
				return
			}
			ticker := time.NewTicker(20 * time.Millisecond)
			defer ticker.Stop()
			sig := syscall.SIGTERM
			if os.Getenv("ASYNQ_LIFECYCLE_SIGNAL") == "INT" {
				sig = syscall.SIGINT
			}
			for {
				select {
				case <-done:
					return
				case <-ticker.C:
					if err := syscall.Kill(os.Getpid(), sig); err != nil {
						signalErrors <- err
						return
					}
				}
			}
		}()
		defer func() { close(done); <-senderStopped }()
		c := lifecycleRedis(t)
		var err error
		switch mode {
		case "server":
			s := NewServerFromRedisClient(c, Config{Concurrency: 1, Logger: logger, LogLevel: InfoLevel, Queues: map[string]int{"lifecycle-signal": 1}})
			err = s.Run(HandlerFunc(func(context.Context, *Task) error { return nil }))
			if restart := s.Start(HandlerFunc(func(context.Context, *Task) error { return nil })); !errors.Is(restart, ErrServerClosed) {
				t.Fatalf("Run did not shutdown server: %v", restart)
			}
		case "scheduler":
			s := NewSchedulerFromRedisClient(c, &SchedulerOpts{Logger: logger, LogLevel: InfoLevel})
			err = s.Run()
			if restart := s.Start(); restart == nil {
				t.Fatal("Run did not shutdown scheduler")
			}
		case "manager":
			m, e := NewPeriodicTaskManager(PeriodicTaskManagerOpts{PeriodicTaskConfigProvider: lifecycleProvider{}, RedisUniversalClient: c, SchedulerOpts: &SchedulerOpts{Logger: logger, LogLevel: InfoLevel}})
			if e != nil {
				t.Fatal(e)
			}
			err = m.Run()
			if restart := m.s.Start(); restart == nil {
				t.Fatal("Run did not shutdown manager scheduler")
			}
		default:
			t.Fatalf("unknown subprocess mode %q", mode)
		}
		if err != nil {
			t.Fatal(err)
		}
		select {
		case err := <-signalErrors:
			t.Fatal(err)
		default:
		}
		if err := c.Ping(context.Background()).Err(); err != nil {
			t.Fatalf("Run closed shared connection: %v", err)
		}
		fmt.Println("LIFECYCLE_RUN_SIGNAL_COMPLETED")
		return
	}
	for _, mode := range []string{"server", "scheduler", "manager"} {
		for _, sig := range []string{"TERM", "INT"} {
			t.Run(mode+"_"+sig, func(t *testing.T) {
				ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
				defer cancel()
				cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestLifecycleRunSignals$", "-test.v", "-redis_addr="+redisAddr, fmt.Sprintf("-redis_db=%d", redisDB))
				childProfile := ""
				if testing.CoverMode() != "" {
					childProfile = filepath.Join(t.TempDir(), "child.cover")
					cmd.Args = append(cmd.Args, "-test.coverprofile="+childProfile)
				}
				cmd.Env = append(os.Environ(), "ASYNQ_LIFECYCLE_SIGNAL_CHILD="+mode, "ASYNQ_LIFECYCLE_SIGNAL="+sig)
				output, err := cmd.CombinedOutput()
				if err != nil || !strings.Contains(string(output), "LIFECYCLE_RUN_SIGNAL_COMPLETED") {
					t.Fatalf("subprocess Run exit=%v timeout=%v output=%s", err, ctx.Err(), output)
				}
				if destination := os.Getenv("ASYNQ_LIFECYCLE_COVERAGE_DIR"); destination != "" && childProfile != "" {
					profile, err := os.ReadFile(childProfile)
					if err != nil {
						t.Fatal(err)
					}
					if !strings.HasPrefix(string(profile), "mode: "+testing.CoverMode()+"\n") {
						t.Fatal("child coverage mode mismatch")
					}
					if err := os.MkdirAll(destination, 0700); err != nil {
						t.Fatal(err)
					}
					name := mode + "_" + sig
					if err := os.WriteFile(filepath.Join(destination, name+".cover"), profile, 0600); err != nil {
						t.Fatal(err)
					}
					binary, err := os.ReadFile(os.Args[0])
					if err != nil {
						t.Fatal(err)
					}
					source, err := os.ReadFile("lifecycle_contracts_test.go")
					if err != nil {
						t.Fatal(err)
					}
					metadata, err := json.MarshalIndent(map[string]interface{}{
						"case": name, "command": cmd.Args, "cover_mode": testing.CoverMode(),
						"test_source_sha256":          fmt.Sprintf("%x", sha256.Sum256(source)),
						"compiled_test_binary_sha256": fmt.Sprintf("%x", sha256.Sum256(binary)),
						"profile_sha256":              fmt.Sprintf("%x", sha256.Sum256(profile)),
					}, "", "  ")
					if err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(filepath.Join(destination, name+".json"), append(metadata, '\n'), 0600); err != nil {
						t.Fatal(err)
					}
				}

			})
		}
	}
}
