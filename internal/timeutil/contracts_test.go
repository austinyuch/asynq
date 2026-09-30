package timeutil

import (
	"math/rand"
	"sync"
	"testing"
	"testing/quick"
	"time"
)

func TestRealClockContract(t *testing.T) {
	clock := NewRealClock()
	before := time.Now()
	got := clock.Now()
	after := time.Now()
	if got.Before(before) || got.After(after) {
		t.Fatalf("real clock outside observation window: %v <= %v <= %v", before, got, after)
	}
}

func TestSimulatedClockSetAndConcurrentAdvance(t *testing.T) {
	start := time.Date(2026, 10, 1, 0, 0, 0, 17, time.FixedZone("test", 3600))
	clock := NewSimulatedClock(time.Time{})
	clock.SetTime(start)
	if got := clock.Now(); got != start {
		t.Fatalf("SetTime lost exact time/location: %v", got)
	}
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				clock.AdvanceTime(time.Nanosecond)
				_ = clock.Now()
			}
		}()
	}
	wg.Wait()
	if got, want := clock.Now(), start.Add(3200*time.Nanosecond); got != want {
		t.Fatalf("concurrent updates lost: %v, want %v", got, want)
	}
	clock.SetTime(time.Time{})
	if !clock.Now().IsZero() {
		t.Error("zero SetTime not retained")
	}
}

func checkClockReversal(a, b int64) bool {
	start := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	c := NewSimulatedClock(start)
	// Bound durations so summation and reversal cannot overflow int64.
	da, db := time.Duration(a%1e12), time.Duration(b%1e12)
	c.AdvanceTime(da)
	c.AdvanceTime(db)
	if c.Now() != start.Add(da+db) {
		return false
	}
	c.AdvanceTime(-db)
	c.AdvanceTime(-da)
	return c.Now() == start
}

func TestClockReversalProperty(t *testing.T) {
	if err := quick.Check(checkClockReversal, &quick.Config{MaxCount: 10000, Rand: rand.New(rand.NewSource(20261002))}); err != nil {
		t.Fatal(err)
	}
}

func FuzzClockReversal(f *testing.F) {
	f.Add(int64(0), int64(0))
	f.Add(int64(-1), int64(1))
	f.Add(int64(1<<62), int64(-1<<62))
	f.Fuzz(func(t *testing.T, a, b int64) {
		if !checkClockReversal(a, b) {
			t.Fatal("clock addition/reversal contract failed")
		}
	})
}
