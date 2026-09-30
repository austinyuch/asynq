package errors

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"
	"testing/quick"
)

func TestCodeAndBuilderContracts(t *testing.T) {
	for i, want := range []string{"ERROR_CODE_UNSPECIFIED", "NOT_FOUND", "FAILED_PRECONDITION", "INTERNAL_ERROR", "ALREADY_EXISTS", "UNKNOWN"} {
		if got := Code(i).String(); got != want {
			t.Errorf("code %d = %q, want %q", i, got, want)
		}
	}
	for _, fn := range []func(){func() { _ = Code(255).String() }, func() { _ = E() }} {
		func() {
			defer func() {
				if recover() == nil {
					t.Error("invalid call did not panic")
				}
			}()
			fn()
		}()
	}
	if err := E(42); !strings.Contains(err.Error(), "unknown type int") {
		t.Fatalf("unexpected bad-argument error: %v", err)
	}
	sentinel := New("sentinel")
	e := E(Op("first"), NotFound, "old", Op("last"), Internal, sentinel).(*Error)
	if e.Op != "last" || e.Code != Internal || Unwrap(e) != sentinel {
		t.Fatalf("last typed arguments must win: %#v", e)
	}
	if Unwrap(New("plain")) != nil {
		t.Error("plain error must not unwrap")
	}
	if got := CanonicalCode(E(Op("outer"), E(NotFound))); got != NotFound {
		t.Fatalf("nested unspecified code: %v", got)
	}
}

func TestDomainErrorContracts(t *testing.T) {
	sentinel := New("offline")
	tests := []struct {
		err       error
		want      string
		predicate func(error) bool
	}{
		{&TaskNotFoundError{Queue: "q", ID: "id"}, `cannot find task with id=id in queue "q"`, IsTaskNotFound},
		{&QueueNotFoundError{Queue: "q"}, `queue "q" does not exist`, IsQueueNotFound},
		{&QueueNotEmptyError{Queue: "q"}, `queue "q" is not empty`, IsQueueNotEmpty},
		{&TaskAlreadyArchivedError{Queue: "q", ID: "id"}, `task is already archived: id=id, queue=q`, IsTaskAlreadyArchived},
		{&RedisCommandError{Command: "lrange", Err: sentinel}, `redis command error: LRANGE failed: offline`, IsRedisCommandError},
		{&PanicError{ErrMsg: "boom"}, `panic error cause by: boom`, IsPanicError},
	}
	for _, tc := range tests {
		if got := tc.err.Error(); got != tc.want {
			t.Errorf("%T: %q, want %q", tc.err, got, tc.want)
		}
		wrapped := fmt.Errorf("outer: %w", E(Op("inner"), tc.err))
		if !tc.predicate(wrapped) || tc.predicate(nil) || tc.predicate(New("unrelated")) {
			t.Errorf("%T chain predicate failed", tc.err)
		}
	}
	redisErr := &RedisCommandError{Command: "GET", Err: sentinel}
	if Unwrap(redisErr) != sentinel || !Is(fmt.Errorf("outer: %w", redisErr), sentinel) {
		t.Error("redis error lost cause identity")
	}
}

func TestErrorFormattingZeroFields(t *testing.T) {
	for _, tc := range []struct {
		e           *Error
		user, debug string
	}{
		{&Error{}, "", ""}, {&Error{Op: "op"}, "", "op"}, {&Error{Code: NotFound}, "NOT_FOUND", "NOT_FOUND"},
		{&Error{Err: New("cause")}, "cause", "cause"}, {&Error{Op: "op", Err: New("cause")}, "cause", "op: cause"},
	} {
		if tc.e.Error() != tc.user || tc.e.DebugString() != tc.debug {
			t.Errorf("%#v formatting mismatch", tc.e)
		}
	}
}

func checkErrorChain(message string, codeByte, depthByte uint8) bool {
	code := Code(codeByte % 6)
	sentinel := New(message)
	err := E(code, sentinel)
	for i := 0; i < int(depthByte%16); i++ {
		err = E(Op("outer"), err)
	}
	return CanonicalCode(err) == code && Is(err, sentinel) && strings.HasSuffix(err.Error(), message)
}

func TestErrorChainProperty(t *testing.T) {
	if err := quick.Check(checkErrorChain, &quick.Config{MaxCount: 1000, Rand: rand.New(rand.NewSource(20261002))}); err != nil {
		t.Fatal(err)
	}
}

func FuzzErrorChain(f *testing.F) {
	f.Add("cause", uint8(NotFound), uint8(3))
	f.Add("", uint8(Unspecified), uint8(0))
	f.Fuzz(func(t *testing.T, message string, code, depth uint8) {
		if !checkErrorChain(message, code, depth) {
			t.Fatal("wrapping changed canonical code or cause identity")
		}
	})
}
