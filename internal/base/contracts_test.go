package base

import (
	"bytes"
	"math"
	"math/rand"
	"testing"
	"testing/quick"
	"unicode/utf8"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestTaskStateContract(t *testing.T) {
	for i, name := range []string{"active", "pending", "scheduled", "retry", "archived", "completed", "aggregating"} {
		state := TaskState(i + 1)
		if state.String() != name {
			t.Fatalf("state %d: got %q, want %q", state, state.String(), name)
		}
		got, err := TaskStateFromString(name)
		if err != nil || got != state {
			t.Fatalf("parse %q: %v, %v", name, got, err)
		}
	}
	for _, name := range []string{"", "Active", " active", "pending ", "unknown"} {
		if got, err := TaskStateFromString(name); err == nil || got != 0 {
			t.Errorf("invalid state %q accepted: %v", name, got)
		}
	}
	for _, state := range []TaskState{-1, 0, 8} {
		t.Run(string(rune(state+65)), func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Error("invalid state must panic")
				}
			}()
			state.String()
		})
	}
}

func TestQueueNameContract(t *testing.T) {
	for _, name := range []string{"", " ", "\t\n", "\u00a0\u2003"} {
		if ValidateQueueName(name) == nil {
			t.Errorf("blank queue %q accepted", name)
		}
	}
	for _, name := range []string{"default", " queue ", "佇列", "\x00", "a:b"} {
		if err := ValidateQueueName(name); err != nil {
			t.Errorf("queue %q rejected: %v", name, err)
		}
	}
}

func TestEncodingRejectsNilAndMalformed(t *testing.T) {
	encoders := []struct {
		name   string
		encode func() ([]byte, error)
	}{
		{"message", func() ([]byte, error) { return EncodeMessage(nil) }},
		{"server", func() ([]byte, error) { return EncodeServerInfo(nil) }},
		{"worker", func() ([]byte, error) { return EncodeWorkerInfo(nil) }},
		{"entry", func() ([]byte, error) { return EncodeSchedulerEntry(nil) }},
		{"event", func() ([]byte, error) { return EncodeSchedulerEnqueueEvent(nil) }},
	}
	for _, tc := range encoders {
		t.Run(tc.name, func(t *testing.T) {
			if b, err := tc.encode(); err == nil || b != nil {
				t.Fatalf("nil input: bytes=%x error=%v", b, err)
			}
		})
	}
	decoders := []struct {
		name   string
		decode func([]byte) error
	}{
		{"message", func(b []byte) error { _, err := DecodeMessage(b); return err }},
		{"server", func(b []byte) error { _, err := DecodeServerInfo(b); return err }},
		{"worker", func(b []byte) error { _, err := DecodeWorkerInfo(b); return err }},
		{"entry", func(b []byte) error { _, err := DecodeSchedulerEntry(b); return err }},
		{"event", func(b []byte) error { _, err := DecodeSchedulerEnqueueEvent(b); return err }},
	}
	for _, tc := range decoders {
		t.Run(tc.name, func(t *testing.T) {
			for _, b := range [][]byte{{0xff}, {0x0a, 0x80}, {0}} {
				if tc.decode(b) == nil {
					t.Errorf("malformed protobuf %x accepted", b)
				}
			}
		})
	}
}

// Independent range oracle protects saturation across the host int domain.
func TestInt32SaturationProperty(t *testing.T) {
	err := quick.Check(func(n int) bool {
		got := int64(toInt32(n))
		if int64(n) > math.MaxInt32 {
			return got == math.MaxInt32
		}
		if int64(n) < math.MinInt32 {
			return got == math.MinInt32
		}
		return got == int64(n)
	}, &quick.Config{MaxCount: 10000, Rand: rand.New(rand.NewSource(20261001))})
	if err != nil {
		t.Fatal(err)
	}
}

// Arbitrary wire bytes must either fail cleanly or survive a semantic roundtrip.
func FuzzMessageWireRoundTrip(f *testing.F) {
	for _, b := range [][]byte{nil, {0xff}, {0x0a, 0x01, 'a'}, {0x10, 0x80}} {
		f.Add(b)
	}
	f.Fuzz(func(t *testing.T, b []byte) {
		msg, err := DecodeMessage(b)
		if err != nil {
			return
		}
		encoded, err := EncodeMessage(msg)
		if err != nil {
			t.Fatalf("decoded message cannot encode: %v", err)
		}
		got, err := DecodeMessage(encoded)
		if err != nil {
			t.Fatalf("encoded message cannot decode: %v", err)
		}
		if diff := cmp.Diff(msg, got, cmpopts.EquateEmpty()); diff != "" {
			t.Fatalf("wire roundtrip changed task fields (-want +got):\n%s", diff)
		}
	})
}

func FuzzMessageRetryRoundTrip(f *testing.F) {
	f.Add("task", []byte{0, 255}, int64(0), int64(math.MaxInt64))
	f.Add("佇列", []byte{}, int64(math.MinInt64), int64(math.MaxInt32))
	f.Fuzz(func(t *testing.T, typ string, payload []byte, retry, retried int64) {
		if !utf8.ValidString(typ) {
			return
		}
		msg := &TaskMessage{Type: typ, Payload: payload, Retry: int(retry), Retried: int(retried)}
		b, err := EncodeMessage(msg)
		if err != nil {
			t.Fatal(err)
		}
		got, err := DecodeMessage(b)
		if err != nil {
			t.Fatal(err)
		}
		clamp := func(n int64) int {
			if n > math.MaxInt32 {
				return math.MaxInt32
			}
			if n < math.MinInt32 {
				return math.MinInt32
			}
			return int(n)
		}
		if got.Type != typ || !bytes.Equal(got.Payload, payload) || got.Retry != clamp(int64(msg.Retry)) || got.Retried != clamp(int64(msg.Retried)) {
			t.Fatal("task roundtrip lost payload/type or failed retry saturation")
		}
	})
}
