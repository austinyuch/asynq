package main

import (
	"context"
	"errors"
	"math/rand"
	"testing"
	"testing/quick"

	"github.com/redis/go-redis/v9"
)

type admissionSpy struct {
	wantContext        context.Context
	size               int64
	queryErr, closeErr error
	queries, closes    int
	contextOK          bool
}

func (s *admissionSpy) DBSize(ctx context.Context) *redis.IntCmd {
	s.queries++
	s.contextOK = ctx == s.wantContext
	return redis.NewIntResult(s.size, s.queryErr)
}
func (s *admissionSpy) Close() error { s.closes++; return s.closeErr }

func TestDemoDatabaseAdmission(t *testing.T) {
	queryFailure := errors.New("DBSIZE transport failure")
	closeFailure := errors.New("connection close failure")
	for _, tc := range []struct {
		name                        string
		size                        int64
		queryErr, closeErr, wantErr error
	}{
		{name: "empty"},
		{name: "existing user data", size: 1, wantErr: errDemoDBNotEmpty},
		{name: "invalid size", size: -1, wantErr: errDemoDBNotEmpty},
		{name: "query failure", queryErr: queryFailure, wantErr: queryFailure},
		{name: "cancelled request", queryErr: context.Canceled, wantErr: context.Canceled},
		{name: "close failure", closeErr: closeFailure, wantErr: closeFailure},
		{name: "query and close failure", queryErr: queryFailure, closeErr: closeFailure, wantErr: queryFailure},
		{name: "existing data and close failure", size: 42, closeErr: closeFailure, wantErr: errDemoDBNotEmpty},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.WithValue(context.Background(), struct{}{}, tc.name)
			spy := &admissionSpy{wantContext: ctx, size: tc.size, queryErr: tc.queryErr, closeErr: tc.closeErr}
			err := requireEmptyDemoDB(ctx, spy)
			if tc.wantErr == nil && err != nil || tc.wantErr != nil && !errors.Is(err, tc.wantErr) {
				t.Fatalf("admission error=%v, want cause %v", err, tc.wantErr)
			}
			if tc.closeErr != nil && !errors.Is(err, tc.closeErr) {
				t.Fatalf("close failure lost: %v", err)
			}
			if spy.queries != 1 || spy.closes != 1 || !spy.contextOK {
				t.Fatalf("resource ownership/context violated: %+v", spy)
			}
		})
	}
}

func admissionProperty(size int64) bool {
	ctx := context.Background()
	spy := &admissionSpy{wantContext: ctx, size: size}
	err := requireEmptyDemoDB(ctx, spy)
	return (size == 0 && err == nil || size != 0 && errors.Is(err, errDemoDBNotEmpty)) && spy.queries == 1 && spy.closes == 1 && spy.contextOK
}
func TestDemoAdmissionProperty(t *testing.T) {
	if err := quick.Check(admissionProperty, &quick.Config{MaxCount: 10000, Rand: rand.New(rand.NewSource(20261001))}); err != nil {
		t.Fatal(err)
	}
}
func FuzzDemoDatabaseAdmission(f *testing.F) {
	f.Add(int64(0))
	f.Add(int64(1))
	f.Add(int64(-1))
	f.Add(int64(9223372036854775807))
	f.Fuzz(func(t *testing.T, size int64) {
		if !admissionProperty(size) {
			t.Fatal("nonempty admission or connection ownership regressed")
		}
	})
}
