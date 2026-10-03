package rdb

import (
	"context"
	stderrors "errors"
	"math/rand"
	"testing"
	"testing/quick"

	errors "github.com/austinyuch/asynq/internal/errors"
	"github.com/redis/go-redis/v9"
)

// Opaque keys and arguments must not hide a closed transport or change its
// canonical classification. No Redis endpoint is contacted after Close.
func checkScriptClosedTransport(t *testing.T, key, argument string) {
	t.Helper()
	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"})
	if err := client.Close(); err != nil {
		t.Fatal(err)
	}
	r := NewRDB(client)
	script := redis.NewScript("return 1")
	const op errors.Op = "rdb.closed-transport-property"
	cases := []struct {
		name string
		code errors.Code
		call func() error
	}{
		{"script", errors.Internal, func() error { return r.runScript(context.Background(), op, script, []string{key}, argument) }},
		{"integer-script", errors.Unknown, func() error {
			n, err := r.runScriptWithErrorCode(context.Background(), op, script, []string{key}, argument)
			if n != 0 {
				t.Errorf("closed script returned %d", n)
			}
			return err
		}},
	}
	for _, tc := range cases {
		err := tc.call()
		var canonical *errors.Error
		if !stderrors.As(err, &canonical) || canonical.Code != tc.code || canonical.Op != op || !stderrors.Is(err, redis.ErrClosed) {
			t.Fatalf("%s changed transport contract: %v", tc.name, err)
		}
		if got, want := err.Error(), tc.code.String()+": redis eval error: "+redis.ErrClosed.Error(); got != want {
			t.Fatalf("%s diagnostic=%q want=%q", tc.name, got, want)
		}
	}
}

func TestScriptClosedTransportProperty(t *testing.T) {
	property := func(key, argument string) bool { checkScriptClosedTransport(t, key, argument); return true }
	if err := quick.Check(property, &quick.Config{MaxCount: 100, Rand: rand.New(rand.NewSource(20261003))}); err != nil {
		t.Fatal(err)
	}
}

func FuzzScriptClosedTransportCause(f *testing.F) {
	f.Add("", "")
	f.Add("{queue}:task", "\x00\xffpayload")
	f.Add("佇列", "任務")
	f.Fuzz(func(t *testing.T, key, argument string) {
		if len(key)+len(argument) > 4096 {
			t.Skip("bounded argument domain")
		}
		checkScriptClosedTransport(t, key, argument)
	})
}
