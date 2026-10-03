package rdb

import (
	"context"
	stderrors "errors"
	"github.com/austinyuch/asynq/internal/base"
	canonical "github.com/austinyuch/asynq/internal/errors"
	h "github.com/austinyuch/asynq/internal/testutil"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"net"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"
)

type aclRemovalHook struct{ scriptReturnedOne bool }

func (h *aclRemovalHook) DialHook(next redis.DialHook) redis.DialHook {
	return func(ctx context.Context, network, addr string) (net.Conn, error) { return next(ctx, network, addr) }
}
func (h *aclRemovalHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return next
}
func (h *aclRemovalHook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		err := next(ctx, cmd)
		if err == nil && (cmd.Name() == "eval" || cmd.Name() == "evalsha") {
			if c, ok := cmd.(*redis.Cmd); ok {
				n, e := c.Int64()
				if e == nil && n == 1 {
					h.scriptReturnedOne = true
				}
			}
		}
		return err
	}
}

func aclOwnSnapshot(t *testing.T, c *redis.Client, keys []string) map[string]interface{} {
	t.Helper()
	ctx := context.Background()
	out := map[string]interface{}{}
	for _, key := range keys {
		kind, e := c.Type(ctx, key).Result()
		if e != nil {
			t.Fatal(e)
		}
		switch kind {
		case "none":
			out[key] = "<absent>"
		case "set":
			v, e := c.SMembers(ctx, key).Result()
			if e != nil {
				t.Fatal(e)
			}
			sort.Strings(v)
			out[key] = v
		case "list":
			v, e := c.LRange(ctx, key, 0, -1).Result()
			if e != nil {
				t.Fatal(e)
			}
			out[key] = v
		case "zset":
			v, e := c.ZRangeWithScores(ctx, key, 0, -1).Result()
			if e != nil {
				t.Fatal(e)
			}
			out[key] = v
		default:
			t.Fatalf("unexpected fixture type %s %s", key, kind)
		}
	}
	return out
}
func TestACLMetadataRemovalPartialEffects(t *testing.T) {
	if useRedisCluster {
		t.Skip("standalone owned ACL contract")
	}
	cases := []struct {
		name, deny       string
		scheduler, force bool
	}{{"scheduler_zrem_denied", "zrem", true, false}, {"scheduler_del_denied", "del", true, false}, {"empty_queue_srem_denied", "srem", false, false}, {"force_pending_srem_denied", "srem", false, true}}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			admin := redis.NewClient(&redis.Options{Addr: redisAddr, DB: redisDB})
			user := "acl-" + uuid.NewString()
			password := uuid.NewString()
			q := "acl-" + uuid.NewString()
			scheduler := uuid.NewString()
			key := base.SchedulerEntriesKey(scheduler)
			globals := []string{base.AllQueues, base.AllSchedulers}
			foreignBefore := aclOwnSnapshot(t, admin, globals)
			client := redis.NewClient(&redis.Options{Addr: redisAddr, DB: redisDB, Username: user, Password: password})
			t.Cleanup(func() {
				if err := client.Close(); err != nil {
					t.Error(err)
				}
				clean, done := context.WithTimeout(context.Background(), 10*time.Second)
				defer done()
				for _, k := range publishedTransportKeys(t, clean, admin, q) {
					if !strings.HasPrefix(k, base.QueueKeyPrefix(q)) {
						t.Fatalf("foreign cleanup key")
					}
					if err := admin.Del(clean, k).Err(); err != nil {
						t.Error(err)
					}
				}
				if err := admin.Del(clean, key).Err(); err != nil {
					t.Error(err)
				}
				if err := admin.SRem(clean, base.AllQueues, q).Err(); err != nil {
					t.Error(err)
				}
				if err := admin.ZRem(clean, base.AllSchedulers, key).Err(); err != nil {
					t.Error(err)
				}
				if err := admin.Do(clean, "ACL", "DELUSER", user).Err(); err != nil {
					t.Error(err)
				}
				if len(publishedTransportKeys(t, clean, admin, q)) != 0 {
					t.Error("owned keys retained")
				}
				if got := aclOwnSnapshot(t, admin, globals); !reflect.DeepEqual(got, foreignBefore) {
					t.Error("foreign registry baseline changed")
				}
				if err := admin.Close(); err != nil {
					t.Error(err)
				}
			})
			writer := NewRDB(admin)
			if tc.scheduler {
				msg := h.NewTaskMessageWithQueue("acl-scheduled", []byte("owned payload"), q)
				entry := &base.SchedulerEntry{ID: msg.ID, Spec: "@hourly", Type: msg.Type, Payload: msg.Payload, Next: time.Unix(1900000000, 0).UTC()}
				if err := writer.WriteSchedulerEntries(scheduler, []*base.SchedulerEntry{entry}, time.Hour); err != nil {
					t.Fatal(err)
				}
			} else if tc.force {
				msg := h.NewTaskMessageWithQueue("acl-pending", []byte("owned payload"), q)
				if err := writer.Enqueue(ctx, msg); err != nil {
					t.Fatal(err)
				}
				if exists, err := admin.Exists(ctx, base.TaskKey(q, msg.ID)).Result(); err != nil || exists != 1 {
					t.Fatal("real task hash absent", err)
				}
			} else {
				if err := admin.SAdd(ctx, base.AllQueues, q).Err(); err != nil {
					t.Fatal(err)
				}
			}
			watched := append(append([]string{}, globals...), key)
			before := aclOwnSnapshot(t, admin, watched)
			queueBefore := publishedTransportSnapshot(t, ctx, admin, q)
			args := []interface{}{"ACL", "SETUSER", user, "reset", "on", ">" + password, "+@all", "-" + tc.deny, "~" + base.QueueKeyPrefix(q) + "*", "~" + key, "~" + base.AllSchedulers, "~" + base.AllQueues}
			if err := admin.Do(ctx, args...).Err(); err != nil {
				t.Fatal(err)
			}
			hook := &aclRemovalHook{}
			client.AddHook(hook)
			r := NewRDB(client)
			var err error
			op := canonical.Op("rdb.RemoveQueue")
			if tc.scheduler {
				op = "rdb.ClearSchedulerEntries"
				err = r.ClearSchedulerEntries(scheduler)
			} else {
				err = r.RemoveQueue(q, tc.force)
			}
			var ce *canonical.Error
			var server redis.Error
			if err == nil || !stderrors.As(err, &ce) || ce.Code != canonical.Unknown || ce.Op != op || !stderrors.As(err, &server) {
				t.Fatalf("expected typed Redis server cause and exact op Unknown: %v", err)
			}
			if !strings.HasPrefix(server.Error(), "NOPERM") {
				t.Errorf("expected actual denied command permission cause: %v", server)
			}
			if tc.scheduler {
				var command *canonical.RedisCommandError
				if !stderrors.As(err, &command) || command.Command != tc.deny {
					t.Fatalf("exact denied command wrapper missing: %v", err)
				}
			}
			after := aclOwnSnapshot(t, admin, watched)
			queueAfter := publishedTransportSnapshot(t, ctx, admin, q)
			want := map[string]interface{}{}
			for k, v := range before {
				want[k] = v
			}
			if tc.scheduler && tc.deny == "del" {
				z := before[base.AllSchedulers].([]redis.Z)
				retained := []redis.Z{}
				for _, v := range z {
					if v.Member != key {
						retained = append(retained, v)
					}
				}
				if len(retained) == 0 {
					want[base.AllSchedulers] = "<absent>"
				} else {
					want[base.AllSchedulers] = retained
				}
				if after[key] == "<absent>" {
					t.Fatal("payload lost after denied DEL")
				}
			}
			if !reflect.DeepEqual(after, want) {
				t.Fatal("global/payload state mismatch", after, want)
			}
			if tc.scheduler || !tc.force {
				if !reflect.DeepEqual(queueAfter, queueBefore) {
					t.Fatal("unexpected queue mutation")
				}
			} else {
				if len(queueBefore) == 0 || len(queueAfter) != 0 {
					t.Fatal("force Lua did not delete actual task/index state")
				}
			}
			if !tc.scheduler && !hook.scriptReturnedOne {
				t.Fatal("Lua return1 not independently observed before SREM denial")
			}
		})
	}
}
