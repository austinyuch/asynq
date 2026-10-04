package rdb

import (
	"bytes"
	"context"
	stderrors "errors"
	"github.com/austinyuch/asynq/internal/base"
	canonical "github.com/austinyuch/asynq/internal/errors"
	h "github.com/austinyuch/asynq/internal/testutil"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"math/rand"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TB supports ordinary tests and native fuzz callbacks without changing sealed contracts.
func aclPayloadSnapshot(tb testing.TB, ctx context.Context, c *redis.Client, q string) map[string]interface{} {
	tb.Helper()
	keys := []string{base.AllQueues, base.AllSchedulers}
	var cursor uint64
	for {
		page, next, err := c.Scan(ctx, cursor, base.QueueKeyPrefix(q)+"*", 100).Result()
		if err != nil {
			tb.Fatal(err)
		}
		keys = append(keys, page...)
		if next == 0 {
			break
		}
		cursor = next
	}
	sort.Strings(keys)
	out := map[string]interface{}{}
	for _, k := range keys {
		kind, err := c.Type(ctx, k).Result()
		if err != nil {
			tb.Fatal(err)
		}
		var v interface{}
		switch kind {
		case "none":
			v = "<absent>"
		case "string":
			v, err = c.Get(ctx, k).Result()
		case "hash":
			v, err = c.HGetAll(ctx, k).Result()
		case "list":
			v, err = c.LRange(ctx, k, 0, -1).Result()
		case "set":
			var a []string
			a, err = c.SMembers(ctx, k).Result()
			sort.Strings(a)
			v = a
		case "zset":
			v, err = c.ZRangeWithScores(ctx, k, 0, -1).Result()
		default:
			tb.Fatalf("unexpected owned type %s", kind)
		}
		if err != nil {
			tb.Fatal(err)
		}
		out[k] = struct {
			Type  string
			Value interface{}
		}{kind, v}
	}
	return out
}
func aclPayloadProperty(tb testing.TB, payload []byte) {
	tb.Helper()
	if useRedisCluster {
		tb.Skip("standalone owned ACL fixture")
	}
	if len(payload) > 4096 {
		tb.Fatal("caller exceeded bounded payload")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	admin := redis.NewClient(&redis.Options{Addr: redisAddr, DB: redisDB})
	q := "acl-pbt-" + uuid.NewString()
	user := "acl-pbt-" + uuid.NewString()
	password := uuid.NewString()
	client := redis.NewClient(&redis.Options{Addr: redisAddr, DB: redisDB, Username: user, Password: password})
	foreign := aclPayloadSnapshot(tb, ctx, admin, q)
	if len(foreign) != 2 {
		tb.Fatal("UUID collision")
	}
	tb.Cleanup(func() {
		if err := client.Close(); err != nil {
			tb.Error(err)
		}
		cleanup, done := context.WithTimeout(context.Background(), 10*time.Second)
		defer done()
		var cursor uint64
		var keys []string
		for {
			page, next, err := admin.Scan(cleanup, cursor, base.QueueKeyPrefix(q)+"*", 100).Result()
			if err != nil {
				tb.Fatal(err)
			}
			keys = append(keys, page...)
			if next == 0 {
				break
			}
			cursor = next
		}
		for _, key := range keys {
			if !strings.HasPrefix(key, base.QueueKeyPrefix(q)) {
				tb.Fatal("foreign cleanup key")
			}
			if err := admin.Del(cleanup, key).Err(); err != nil {
				tb.Error(err)
			}
		}
		if err := admin.SRem(cleanup, base.AllQueues, q).Err(); err != nil {
			tb.Error(err)
		}
		if err := admin.Do(cleanup, "ACL", "DELUSER", user).Err(); err != nil {
			tb.Error(err)
		}
		if got := aclPayloadSnapshot(tb, cleanup, admin, q); !reflect.DeepEqual(got, foreign) {
			tb.Error("foreign registry baseline or own cleanup changed")
		}
		if err := admin.Close(); err != nil {
			tb.Error(err)
		}
	})
	writer := NewRDB(admin)
	msg := h.NewTaskMessageBuilder().SetQueue(q).Build()
	if err := writer.Schedule(ctx, msg, time.Unix(2000000000, 0)); err != nil {
		tb.Fatal(err)
	}
	if err := admin.Do(ctx, "ACL", "SETUSER", user, "reset", "on", ">"+password, "~"+base.QueueKeyPrefix(q)+"*", "~"+base.AllQueues, "+@all").Err(); err != nil {
		tb.Fatal(err)
	}
	subject := NewRDB(client)
	// Native successful control must round-trip opaque bytes, including invalid UTF8 and NUL.
	if err := subject.UpdateTaskPayload(q, msg.ID, payload); err != nil {
		tb.Fatal("control update", err)
	}
	info, err := subject.GetTaskInfo(q, msg.ID)
	if err != nil || !bytes.Equal(info.Message.Payload, payload) {
		tb.Fatal("opaque control roundtrip failed", err)
	}
	before := aclPayloadSnapshot(tb, ctx, admin, q)
	if err := admin.Do(ctx, "ACL", "SETUSER", user, "-hset").Err(); err != nil {
		tb.Fatal(err)
	}
	attempted := append(append([]byte{}, payload...), 0x7f)
	err = subject.UpdateTaskPayload(q, msg.ID, attempted)
	var ce *canonical.Error
	var server redis.Error
	if err == nil || !stderrors.As(err, &ce) || ce.Op != "rdb.UpdateTask" || ce.Code != canonical.Unknown || !stderrors.As(err, &server) {
		tb.Fatalf("denied HSET lost typed cause/op: %v", err)
	}
	if !strings.Contains(server.Error(), "can't run this command") && !strings.Contains(server.Error(), "no permissions to run the 'hset' command") {
		tb.Errorf("expected script Redis permission denial: %v", server)
	}
	if !reflect.DeepEqual(before, aclPayloadSnapshot(tb, ctx, admin, q)) {
		tb.Fatal("denied opaque update mutated Redis state")
	}
	info, err = subject.GetTaskInfo(q, msg.ID)
	if err != nil || !bytes.Equal(info.Message.Payload, payload) {
		tb.Fatal("denial changed stored opaque control payload", err)
	}
}
func TestOwnedACLPayloadProperties(t *testing.T) {
	rng := rand.New(rand.NewSource(20261004))
	for n := 0; n < 100; n++ {
		size := rng.Intn(4097)
		payload := make([]byte, size)
		if _, err := rng.Read(payload); err != nil {
			t.Fatal(err)
		}
		t.Run(strconv.Itoa(n), func(t *testing.T) { aclPayloadProperty(t, payload) })
	}
}
func FuzzOwnedACLPayloadDenied(f *testing.F) {
	for _, seed := range [][]byte{nil, {}, []byte("plain"), {0, 0xff, 0xc0, 0x80}, bytes.Repeat([]byte{0x80}, 4096)} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, payload []byte) {
		if len(payload) > 4096 {
			t.Skip("outside bounded opaque-payload domain")
		}
		aclPayloadProperty(t, payload)
	})
}
