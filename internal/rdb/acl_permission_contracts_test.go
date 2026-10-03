package rdb

import (
	"context"
	stderrors "errors"
	"github.com/austinyuch/asynq/internal/base"
	errors "github.com/austinyuch/asynq/internal/errors"
	h "github.com/austinyuch/asynq/internal/testutil"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"reflect"
	"strings"
	"testing"
	"time"
)

// Only a root-admitted exclusive standalone fixture may execute these ACL writes.
func newOwnedACLSubject(t *testing.T, ctx context.Context, admin *redis.Client, queue, denied string) (*redis.Client, func()) {
	t.Helper()
	user := "asynq-test-" + uuid.NewString()
	password := "task-local-nonsecret-" + uuid.NewString()
	args := []interface{}{"ACL", "SETUSER", user, "reset", "on", ">" + password, "~" + base.QueueKeyPrefix(queue) + "*", "~" + base.AllQueues, "+@all"}
	if err := admin.Do(ctx, args...).Err(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := admin.Do(cleanup, "ACL", "DELUSER", user).Err(); err != nil {
			t.Errorf("delete owned ACL user: %v", err)
		}
	})
	client := redis.NewClient(&redis.Options{Addr: redisAddr, DB: redisDB, Username: user, Password: password})
	t.Cleanup(func() { _ = client.Close() })
	if err := client.Ping(ctx).Err(); err != nil {
		t.Fatal(err)
	}
	// Store a private closure instead of exposing any password in error messages.
	deny := func() {
		if err := admin.Do(ctx, "ACL", "SETUSER", user, "-"+denied).Err(); err != nil {
			t.Fatal(err)
		}
	}
	return client, deny
}

func ownedACLQueue(t *testing.T, ctx context.Context) (*redis.Client, *RDB, string) {
	t.Helper()
	if useRedisCluster {
		t.Skip("exclusive standalone ACL fixture")
	}
	admin := redis.NewClient(&redis.Options{Addr: redisAddr, DB: redisDB})
	writer := NewRDB(admin)
	foreignBefore := aclOwnSnapshot(t, admin, []string{base.AllQueues, base.AllSchedulers})
	q := "acl-contract-" + uuid.NewString()
	t.Cleanup(func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		for _, key := range publishedTransportKeys(t, cleanup, admin, q) {
			if !strings.HasPrefix(key, base.QueueKeyPrefix(q)) {
				t.Errorf("foreign cleanup key")
				continue
			}
			if err := admin.Del(cleanup, key).Err(); err != nil {
				t.Errorf("owned cleanup: %v", err)
			}
		}
		if err := admin.SRem(cleanup, base.AllQueues, q).Err(); err != nil {
			t.Errorf("membership cleanup: %v", err)
		}
		if len(publishedTransportKeys(t, cleanup, admin, q)) != 0 {
			t.Error("owned keys retained")
		}
		if got := aclOwnSnapshot(t, admin, []string{base.AllQueues, base.AllSchedulers}); !reflect.DeepEqual(got, foreignBefore) {
			t.Error("foreign registry baseline changed")
		}
		_ = admin.Close()
	})
	return admin, writer, q
}

func TestOwnedACLMemoryDeniedAfterStatistics(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	t.Setenv("DISABLE_MEMORY_USAGE_PROFILING", "false")
	admin, writer, q := ownedACLQueue(t, ctx)
	msg := h.NewTaskMessageBuilder().SetQueue(q).Build()
	if err := writer.Enqueue(ctx, msg); err != nil {
		t.Fatal(err)
	}
	client, deny := newOwnedACLSubject(t, ctx, admin, q, "memory")
	subject := NewRDB(client)
	control, err := subject.CurrentStats(q)
	if err != nil || control.Pending != 1 || control.MemoryUsage <= 0 {
		t.Fatalf("control statistics failed: %v", err)
	}
	before := publishedTransportSnapshot(t, ctx, admin, q)
	memberBefore, err := admin.SIsMember(ctx, base.AllQueues, q).Result()
	if err != nil {
		t.Fatal(err)
	}
	deny()
	_, err = subject.CurrentStats(q)
	var canonical *errors.Error
	if err == nil || !stderrors.As(err, &canonical) || canonical.Op != errors.Op("rdb.CurrentStats") || canonical.Code != errors.Unknown {
		t.Fatalf("memory denial canonical contract: %v", err)
	}
	// Preserve the actual server cause through memoryUsage and its public wrapper.
	var cause redis.Error
	if !stderrors.As(err, &cause) {
		t.Errorf("memory denial lost typed server cause: %v", err)
	}
	directErr := func() error { _, e := subject.memoryUsage(q); return e }()
	var directCause redis.Error
	if !stderrors.As(directErr, &directCause) {
		t.Errorf("direct memoryUsage lost typed server cause: %v", directErr)
	}
	if !strings.Contains(strings.ToLower(err.Error()), "permission") && !strings.Contains(err.Error(), "NOPERM") && !strings.Contains(err.Error(), "can't run this command") {
		t.Fatalf("expected actual Redis permission diagnostic: %v", err)
	}
	t.Run("DisabledProfilingControl", func(t *testing.T) {
		t.Setenv("DISABLE_MEMORY_USAGE_PROFILING", "true")
		got, e := subject.CurrentStats(q)
		if e != nil {
			t.Fatalf("disabled profiling must bypass denied MEMORY: %v", e)
		}
		if got == nil || got.Pending != 1 || got.MemoryUsage != 0 {
			t.Errorf("disabled control statistics: %#v", got)
		}
	})
	if !reflect.DeepEqual(before, publishedTransportSnapshot(t, ctx, admin, q)) {
		t.Fatal("memory denial changed owned state")
	}
	memberAfter, err := admin.SIsMember(ctx, base.AllQueues, q).Result()
	if err != nil || memberBefore != memberAfter {
		t.Fatalf("owned membership changed: %v", err)
	}
}

func TestOwnedACLUpdatePayloadDeniedAfterRead(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	admin, writer, q := ownedACLQueue(t, ctx)
	msg := h.NewTaskMessageBuilder().SetQueue(q).Build()
	if err := writer.Schedule(ctx, msg, time.Unix(2000000000, 0)); err != nil {
		t.Fatal(err)
	}
	// Deny HSET inside update Lua only: denying EVALSHA statically also denies GetTaskInfo.
	client, deny := newOwnedACLSubject(t, ctx, admin, q, "hset")
	subject := NewRDB(client)
	if _, err := subject.GetTaskInfo(q, msg.ID); err != nil {
		t.Fatalf("control read: %v", err)
	}
	if err := subject.UpdateTaskPayload(q, msg.ID, []byte("control")); err != nil {
		t.Fatalf("control update: %v", err)
	}
	before := publishedTransportSnapshot(t, ctx, admin, q)
	deny()
	info, err := subject.GetTaskInfo(q, msg.ID)
	if err != nil {
		t.Fatalf("read must still succeed after denial: %v", err)
	}
	if string(info.Message.Payload) != "control" {
		t.Fatal("decoded control payload changed before attempted update")
	}
	err = subject.UpdateTaskPayload(q, msg.ID, []byte("must-not-write"))
	var canonical *errors.Error
	if err == nil || !stderrors.As(err, &canonical) || canonical.Op != errors.Op("rdb.UpdateTask") || canonical.Code != errors.Unknown {
		t.Fatalf("update denial canonical contract: %v", err)
	}
	// inspect.go:1499 passes the original server error to errors.E; require reachable redis.Error.
	var cause redis.Error
	if !stderrors.As(err, &cause) {
		t.Fatalf("update denial lost server cause: %v", err)
	}
	if !strings.Contains(strings.ToLower(cause.Error()), "permission") && !strings.Contains(cause.Error(), "NOPERM") && !strings.Contains(cause.Error(), "can't run this command") {
		t.Fatalf("expected permission cause: %v", err)
	}
	if !reflect.DeepEqual(before, publishedTransportSnapshot(t, ctx, admin, q)) {
		t.Fatal("denied update changed owned state")
	}
}
