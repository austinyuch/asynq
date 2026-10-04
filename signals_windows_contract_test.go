//go:build windows

package asynq

import (
	"bytes"
	"fmt"
	win "golang.org/x/sys/windows"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"
	"unsafe"

	asynqlog "github.com/austinyuch/asynq/internal/log"
)

// A child returns through the testing harness so its own coverprofile closes.
// The guard prevents the default CTRL_BREAK termination before the production
// Notify registration; READY alone does not assert that registration happened.
func TestWindowsSignalChild(t *testing.T) {
	kind := os.Getenv("ASYNQ_WINDOWS_SIGNAL_CHILD")
	if kind == "" {
		return
	}
	ready := os.Getenv("ASYNQ_WINDOWS_SIGNAL_READY")
	if ready == "" {
		t.Fatal("owned readiness path required")
	}
	guard := make(chan os.Signal, 64)
	signal.Notify(guard, os.Interrupt)
	defer signal.Stop(guard)
	done := make(chan struct{})
	go func() {
		defer close(done)
		switch kind {
		case "server":
			(&Server{logger: asynqlog.NewLogger(nil)}).waitForSignals()
		case "scheduler":
			(&Scheduler{logger: asynqlog.NewLogger(nil)}).waitForSignals()
		default:
			panic("invalid bounded child kind")
		}
	}()
	select {
	case <-done:
		t.Fatal("waitForSignals returned before any owned signal")
	case <-time.After(50 * time.Millisecond):
	}
	if err := os.WriteFile(ready, []byte(strconv.Itoa(os.Getpid())), 0600); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("waitForSignals failed to return after targeted CTRL_BREAK")
	}
}

func TestWindowsNativeSignals(t *testing.T) {
	out := os.Getenv("ASYNQ_WINDOWS_SIGNAL_ARTIFACTS")
	requireCover := os.Getenv("ASYNQ_WINDOWS_SIGNAL_REQUIRE_COVER") == "1"
	if out == "" {
		if requireCover {
			t.Fatal("campaign requires retained artifact directory")
		}
		out = t.TempDir()
	}
	if requireCover && testing.CoverMode() != "atomic" {
		t.Fatal("campaign requires atomic native coverage instrumentation")
	}
	if err := os.MkdirAll(out, 0700); err != nil {
		t.Fatal(err)
	}
	kernel := syscall.NewLazyDLL("kernel32.dll")
	cp, _, _ := kernel.NewProc("GetConsoleCP").Call()
	if cp == 0 {
		allocated, _, err := kernel.NewProc("AllocConsole").Call()
		if allocated == 0 {
			t.Fatalf("owned console allocation preflight failed: %v", err)
		}
		defer kernel.NewProc("FreeConsole").Call()
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	count := 2 // ordinary consumers need no CI-specific environment or coverage
	if requested := os.Getenv("ASYNQ_WINDOWS_SIGNAL_CASES"); requested != "" {
		parsed, err := strconv.Atoi(requested)
		if err != nil || parsed < 2 || parsed > 100 {
			t.Fatal("bounded requested cases must be2..100")
		}
		count = parsed
	}
	if os.Getenv("ASYNQ_WINDOWS_SIGNAL_MUTANT") == "1" {
		count = 2
	}
	// Deterministic seeded lifecycle properties: independent owned processes,
	// alternating production surfaces, randomized pre-event delays and retry count.
	seed := uint32(20261004)
	for i := 0; i < count; i++ {
		kind := "server"
		if i%2 == 1 {
			kind = "scheduler"
		}
		seed = 1664525*seed + 1013904223
		delay := time.Duration(1+seed%7) * time.Millisecond
		name := fmt.Sprintf("%03d-%s", i, kind)
		t.Run(name, func(t *testing.T) {
			dir := filepath.Join(out, name)
			if err := os.Mkdir(dir, 0700); err != nil {
				t.Fatal(err)
			}
			ready := filepath.Join(dir, "ready.pid")
			argv := []string{"-test.run=^TestWindowsSignalChild$", "-test.v", "-test.timeout=10s"}
			if testing.CoverMode() != "" {
				argv = append(argv, "-test.coverprofile="+filepath.Join(dir, "child.cover"))
			}
			cmd := exec.Command(exe, argv...)
			cmd.Env = append(os.Environ(), "ASYNQ_WINDOWS_SIGNAL_CHILD="+kind, "ASYNQ_WINDOWS_SIGNAL_READY="+ready)
			cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP}
			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			job, err := win.CreateJobObject(nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer win.CloseHandle(job)
			info := win.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
			info.BasicLimitInformation.LimitFlags = win.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
			if _, err := win.SetInformationJobObject(job, win.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info))); err != nil {
				t.Fatal(err)
			}
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			pid := cmd.Process.Pid
			waited := make(chan error, 1)
			go func() { waited <- cmd.Wait() }()
			closed := false
			defer func() {
				if !closed {
					_ = win.TerminateJobObject(job, 1)
					_ = cmd.Process.Kill() // covers failclosed assignment failure before membership
					select {
					case <-waited:
						closed = true
					case <-time.After(3 * time.Second):
						t.Error("owned child wait closure missing after termination")
					}
				}
				if err := os.WriteFile(filepath.Join(dir, "terminal.txt"), []byte(fmt.Sprintf("pid=%d closed=%t\n", pid, closed)), 0600); err != nil {
					t.Error(err)
				}
				if closed {
					if err := os.WriteFile(filepath.Join(dir, "stdout.log"), stdout.Bytes(), 0600); err != nil {
						t.Error(err)
					}
					if err := os.WriteFile(filepath.Join(dir, "stderr.log"), stderr.Bytes(), 0600); err != nil {
						t.Error(err)
					}
				}
			}()
			process, err := win.OpenProcess(win.PROCESS_SET_QUOTA|win.PROCESS_TERMINATE|win.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
			if err != nil {
				t.Fatal(err)
			}
			var created, exited, kernelTime, userTime win.Filetime
			identityErr := win.GetProcessTimes(process, &created, &exited, &kernelTime, &userTime)
			assignErr := win.AssignProcessToJobObject(job, process)
			_ = win.CloseHandle(process)
			if identityErr != nil {
				t.Fatal(identityErr)
			}
			if assignErr != nil {
				t.Fatalf("owned job assignment preflight failed: %v", assignErr)
			}
			if err := os.WriteFile(filepath.Join(dir, "identity.txt"), []byte(fmt.Sprintf("pid=%d creation_hi=%d creation_lo=%d kind=%s owned_job=true new_process_group=true\n", pid, created.HighDateTime, created.LowDateTime, kind)), 0600); err != nil {
				t.Fatal(err)
			}
			deadline := time.Now().Add(6 * time.Second)
			for {
				select {
				case err := <-waited:
					closed = true
					t.Fatalf("owned child exited before READY: %v", err)
				default:
				}
				if raw, err := os.ReadFile(ready); err == nil {
					if string(raw) != strconv.Itoa(pid) {
						t.Fatal("owned READY PID mismatch")
					}
					break
				}
				if time.Now().After(deadline) {
					t.Fatal("owned READY deadline")
				}
				time.Sleep(10 * time.Millisecond)
			}
			time.Sleep(delay)
			proc := syscall.NewLazyDLL("kernel32.dll").NewProc("GenerateConsoleCtrlEvent")
			attempts := 0
			for {
				attempts++
				r, _, apiErr := proc.Call(syscall.CTRL_BREAK_EVENT, uintptr(pid))
				if r == 0 {
					t.Fatalf("owned console preflight/CTRL_BREAK failed (not SKIP): %v", apiErr)
				}
				select {
				case err := <-waited:
					closed = true
					if writeErr := os.WriteFile(filepath.Join(dir, "custody.txt"), []byte(fmt.Sprintf("pid=%d kind=%s attempts=%d wait=%v closed=true\n", pid, kind, attempts, err)), 0600); writeErr != nil {
						t.Fatal(writeErr)
					}
					if err != nil {
						t.Fatalf("owned native signal assertion failed: %v", err)
					}
					if testing.CoverMode() != "" {
						raw, err := os.ReadFile(filepath.Join(dir, "child.cover"))
						if err != nil || !bytes.HasPrefix(raw, []byte("mode:")) {
							t.Fatalf("own child profile incomplete: %v", err)
						}
					} else if requireCover {
						t.Fatal("campaign coverage instrumentation missing")
					}
					return
				case <-time.After(100 * time.Millisecond):
				}
				if time.Now().After(deadline) {
					t.Fatal("owned child completion deadline")
				}
			}
		})
	}
}
