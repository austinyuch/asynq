package log

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
)

type contractBase struct{ calls []string }

func (b *contractBase) Debug(v ...interface{}) { b.calls = append(b.calls, "debug:"+fmt.Sprint(v...)) }
func (b *contractBase) Info(v ...interface{})  { b.calls = append(b.calls, "info:"+fmt.Sprint(v...)) }
func (b *contractBase) Warn(v ...interface{})  { b.calls = append(b.calls, "warning:"+fmt.Sprint(v...)) }
func (b *contractBase) Error(v ...interface{}) { b.calls = append(b.calls, "error:"+fmt.Sprint(v...)) }
func (b *contractBase) Fatal(v ...interface{}) { b.calls = append(b.calls, "fatal:"+fmt.Sprint(v...)) }

func TestLevelAndFatalContracts(t *testing.T) {
	for i, want := range []string{"debug", "info", "warning", "error", "fatal"} {
		if got := Level(i).String(); got != want {
			t.Errorf("level %d=%q,want %q", i, got, want)
		}
	}
	for _, v := range []Level{-1, 5} {
		if v.String() != "unknown" {
			t.Error("invalid level must format as unknown")
		}
		l := NewLogger(&contractBase{})
		func() {
			defer func() {
				if recover() == nil {
					t.Error("invalid level must panic")
				}
			}()
			l.SetLevel(v)
		}()
		// Panic must release the mutex; otherwise this subsequent call deadlocks.
		l.SetLevel(InfoLevel)
	}
	if l := NewLogger(nil); l.base == nil || l.level != DebugLevel {
		t.Error("default logger constructor contract failed")
	}
	b := &contractBase{}
	l := NewLogger(b)
	l.Fatal("one")
	l.Fatalf("%s:%d", "two", 2)
	if strings.Join(b.calls, ",") != "fatal:one,fatal:two:2" {
		t.Fatalf("fatal forwarding: %v", b.calls)
	}
	// An out-of-range threshold is reachable only internally; verify Fatal uses
	// the same gate as other levels without terminating this test process.
	l.level = FatalLevel + 1
	l.Fatal("suppressed")
	if len(b.calls) != 2 {
		t.Error("Fatal bypassed level gate")
	}
}

func TestLoggingThresholdMatrix(t *testing.T) {
	for threshold := DebugLevel; threshold <= FatalLevel; threshold++ {
		b := &contractBase{}
		l := NewLogger(b)
		l.SetLevel(threshold)
		for level, emit := range []func(){func() { l.Debug("x") }, func() { l.Info("x") }, func() { l.Warn("x") }, func() { l.Error("x") }, func() { l.Fatal("x") }} {
			before := len(b.calls)
			emit()
			want := 0
			if Level(level) >= threshold {
				want = 1
			}
			if len(b.calls)-before != want {
				t.Fatalf("threshold %d, level %d incorrectly gated", threshold, level)
			}
		}
	}
}

func TestBaseFatalProcessContract(t *testing.T) {
	if os.Getenv("ASYNQ_LOG_FATAL_CHILD") == "1" {
		newBase(os.Stdout).Fatal("contract-marker")
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestBaseFatalProcessContract$")
	cmd.Env = append(os.Environ(), "ASYNQ_LOG_FATAL_CHILD=1")
	output, err := cmd.CombinedOutput()
	exit, ok := err.(*exec.ExitError)
	if !ok || exit.ExitCode() != 1 || !strings.Contains(string(output), "FATAL: contract-marker") {
		t.Fatalf("fatal exit/log contract: %v, %q", err, output)
	}
}
