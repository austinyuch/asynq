package cmd

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/austinyuch/asynq"
	"github.com/austinyuch/asynq/internal/base"
	"github.com/fatih/color"
	"github.com/redis/go-redis/v9"
	"github.com/spf13/cobra"
)

type clusterHistoricalKey struct {
	DumpBase64 string
	PTTLMillis int64
}
type clusterHistoricalSnapshot struct {
	Keys         map[string]clusterHistoricalKey
	QueueMembers []string
}

func clusterHistoricalRead(t *testing.T, c *redis.ClusterClient) clusterHistoricalSnapshot {
	t.Helper()
	ctx := context.Background()
	result := clusterHistoricalSnapshot{Keys: map[string]clusterHistoricalKey{}}
	var mu sync.Mutex
	if e := c.ForEachMaster(ctx, func(ctx context.Context, node *redis.Client) error {
		keys, e := node.Keys(ctx, "*").Result()
		if e != nil {
			return e
		}
		for _, key := range keys {
			dump, e := node.Dump(ctx, key).Result()
			if e != nil {
				return e
			}
			ttl, e := node.PTTL(ctx, key).Result()
			if e != nil {
				return e
			}
			mu.Lock()
			result.Keys[key] = clusterHistoricalKey{base64.StdEncoding.EncodeToString([]byte(dump)), func() int64 {
				if ttl < 0 {
					return int64(ttl)
				}
				return ttl.Milliseconds()
			}()}
			mu.Unlock()
		}
		return nil
	}); e != nil {
		t.Fatal(e)
	}
	members, e := c.SMembers(ctx, base.AllQueues).Result()
	if e != nil {
		t.Fatal(e)
	}
	sort.Strings(members)
	result.QueueMembers = members
	return result
}
func clusterEvidence(t *testing.T, name string, v any) {
	t.Helper()
	dir := os.Getenv("ASYNQ_CLUSTER_CONTRACT_EVIDENCE_DIR")
	if dir == "" {
		return
	}
	if e := os.MkdirAll(dir, 0700); e != nil {
		t.Fatal(e)
	}
	b, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(filepath.Join(dir, name), append(b, '\n'), 0600); e != nil {
		t.Fatal(e)
	}
}
func clusterHistoricalAssert(t *testing.T, before, after clusterHistoricalSnapshot) {
	t.Helper()
	if !reflect.DeepEqual(before.QueueMembers, after.QueueMembers) {
		t.Error("owned cleanup changed historical queue members")
	}
	if len(before.Keys) != len(after.Keys) {
		t.Errorf("historical inventory leaked/lost keys: before%d after%d", len(before.Keys), len(after.Keys))
	}
	for key, want := range before.Keys {
		got, ok := after.Keys[key]
		if !ok {
			t.Errorf("historical key disappeared: %s", key)
			continue
		}
		if key != base.AllQueues && got != want {
			t.Errorf("historical DUMP/PTTL changed: %s", key)
		}
		if key == base.AllQueues && got.PTTLMillis != want.PTTLMillis {
			t.Errorf("historical AllQueues PTTL changed")
		}
		if key == base.AllQueues && got.DumpBase64 != want.DumpBase64 {
			t.Log("AllQueues set serialization changed after authorized owned SADD/SREM; exact members checked")
		}
	}
}
func clusterCleanupOwned(t *testing.T, c *redis.ClusterClient, queues []string) {
	t.Helper()
	ctx := context.Background()
	for _, q := range queues {
		var keys []string
		var mu sync.Mutex
		if e := c.ForEachMaster(ctx, func(ctx context.Context, node *redis.Client) error {
			owned, e := node.Keys(ctx, base.QueueKeyPrefix(q)+"*").Result()
			mu.Lock()
			keys = append(keys, owned...)
			mu.Unlock()
			return e
		}); e != nil {
			t.Error(e)
		}
		for _, key := range keys {
			if e := c.Del(ctx, key).Err(); e != nil {
				t.Error(e)
			}
		}
		if e := c.SRem(ctx, base.AllQueues, q).Err(); e != nil {
			t.Error(e)
		}
	}
}

func TestClusterQueueStatsConfigAndFlagContracts(t *testing.T) {
	addrs := os.Getenv("ASYNQ_CLI_CLUSTER_CONTRACT_ADDRS")
	if addrs == "" {
		t.Skip("requires exclusively claimed three-master cluster")
	}
	for _, mode := range []string{"config-only", "flag"} {
		t.Run(mode, func(t *testing.T) {
			isolatedCLI(t)
			oldErr := configLoadErr
			t.Cleanup(func() { configLoadErr = oldErr })
			// Both variants load a real explicit file and never consult HOME.
			cfgFile = filepath.Join(t.TempDir(), "cluster.yaml")
			body := "cluster: true\ncluster_addrs: " + strconv.Quote(addrs) + "\n"
			if mode == "flag" {
				body = "{}\n"
				if e := rootCmd.PersistentFlags().Set("cluster", "true"); e != nil {
					t.Fatal(e)
				}
				if e := rootCmd.PersistentFlags().Set("cluster_addrs", addrs); e != nil {
					t.Fatal(e)
				}
			}
			if e := os.WriteFile(cfgFile, []byte(body), 0600); e != nil {
				t.Fatal(e)
			}
			captureCLI(t, initConfig)
			if configLoadErr != nil {
				t.Fatal(configLoadErr)
			}
			opt, ok := getRedisConnOpt().(asynq.RedisClusterClientOpt)
			if !ok || !reflect.DeepEqual(opt.Addrs, strings.Split(addrs, ",")) {
				t.Fatalf("configuration factory not cluster: %#v", getRedisConnOpt())
			}
			if mode == "config-only" && useRedisCluster {
				t.Fatal("config-only fixture accidentally set raw CLI flag")
			}
			if mode == "flag" && !useRedisCluster {
				t.Fatal("flag fixture did not set real flag")
			}
			c := redis.NewClusterClient(&redis.ClusterOptions{Addrs: strings.Split(addrs, ",")})
			t.Cleanup(func() { _ = c.Close() })
			ctx := context.Background()
			if e := c.Ping(ctx).Err(); e != nil {
				t.Fatal(e)
			}
			before := clusterHistoricalRead(t, c)
			if len(before.Keys) != 11 {
				t.Fatalf("custody baseline expected 11 keys, got %d; no fixture writes admitted", len(before.Keys))
			}
			clusterEvidence(t, mode+"-baseline-before.json", before)
			t.Cleanup(func() {
				after := clusterHistoricalRead(t, c)
				clusterEvidence(t, mode+"-baseline-after.json", after)
				clusterHistoricalAssert(t, before, after)
			})
			rng := rand.New(rand.NewSource(20261017))
			prefix := fmt.Sprintf("cli-cluster-owned-%d", time.Now().UnixNano())
			var queues []string
			t.Cleanup(func() { clusterCleanupOwned(t, c, queues) })
			client := asynq.NewClient(opt)
			t.Cleanup(func() { _ = client.Close() })
			for n := 0; n < 4; n++ {
				q := fmt.Sprintf("%s-%08x", prefix, rng.Uint32())
				queues = append(queues, q)
				for k := 0; k < 1+n; k++ {
					id := fmt.Sprintf("owned-%d-%d", n, k)
					if _, e := client.Enqueue(asynq.NewTask("cluster-contract", []byte(id)), asynq.Queue(q), asynq.TaskID(id), asynq.Unique(time.Minute)); e != nil {
						t.Fatal(e)
					}
				}
			}
			output, e := invokeAdministration(t, queueListCmd, queueList, nil, nil)
			if e != nil {
				t.Fatal(e)
			}
			if !strings.Contains(output, "Cluster KeySlot") || !strings.Contains(output, "Cluster Nodes") {
				t.Fatalf("cluster output mode lost: %s", output)
			}
			slots, e := c.ClusterSlots(ctx).Result()
			if e != nil {
				t.Fatal(e)
			}
			for _, q := range queues {
				slot, e := c.ClusterKeySlot(ctx, base.PendingKey(q)).Result()
				if e != nil {
					t.Fatal(e)
				}
				var fields []string
				for _, candidate := range strings.Split(output, "\n") {
					row := strings.Fields(candidate)
					if len(row) > 0 && row[0] == q {
						fields = row
						break
					}
				}
				if len(fields) != 3 {
					t.Fatalf("queue row requires exactly three columns: %s %#v", q, fields)
				}
				gotSlot, parseErr := strconv.ParseInt(fields[1], 10, 64)
				if parseErr != nil || gotSlot != slot {
					t.Fatalf("queue slot token mismatch: %s got%s want%d", q, fields[1], slot)
				}
				var expectedNodes []string
				found := false
				for _, rangeInfo := range slots {
					if slot >= int64(rangeInfo.Start) && slot <= int64(rangeInfo.End) {
						found = true
						for _, node := range rangeInfo.Nodes {
							expectedNodes = append(expectedNodes, node.ID+"@"+node.Addr)
						}
					}
				}
				if !found {
					t.Fatal("independent CLUSTER SLOTS missing keyslot")
				}
				sort.Strings(expectedNodes)
				actualNodes := strings.Split(fields[2], ",")
				if !reflect.DeepEqual(actualNodes, expectedNodes) {
					t.Fatalf("cluster nodes exact canonical set mismatch: %s got%v want%v", q, actualNodes, expectedNodes)
				}
			}
			output, e = invokeAdministration(t, statsCmd, stats, nil, map[string]string{"json": "true"})
			if e != nil {
				t.Fatal(e)
			}
			var decoded FullStats
			if e := json.Unmarshal([]byte(strings.TrimSpace(output)), &decoded); e != nil {
				t.Fatalf("stats JSON: %v %s", e, output)
			}
			actualInfo, e := c.ClusterInfo(ctx).Result()
			if e != nil {
				t.Fatal(e)
			}
			for _, key := range []string{"cluster_state", "cluster_known_nodes", "cluster_size"} {
				var expected string
				for _, line := range strings.Split(actualInfo, "\n") {
					if strings.HasPrefix(line, key+":") {
						expected = strings.TrimSpace(strings.TrimPrefix(line, key+":"))
					}
				}
				if expected == "" || decoded.RedisInfo[key] != expected {
					t.Fatalf("cluster metadata %s=%q want%q", key, decoded.RedisInfo[key], expected)
				}
			}
			members, e := c.SMembers(ctx, base.AllQueues).Result()
			if e != nil {
				t.Fatal(e)
			}
			expected := AggregateStats{}
			for _, q := range members {
				for key, dst := range map[string]*int{base.PendingKey(q): &expected.Pending, base.ActiveKey(q): &expected.Active} {
					n, e := c.LLen(ctx, key).Result()
					if e != nil {
						t.Fatal(e)
					}
					*dst += int(n)
				}
				for key, dst := range map[string]*int{base.ScheduledKey(q): &expected.Scheduled, base.RetryKey(q): &expected.Retry, base.ArchivedKey(q): &expected.Archived, base.CompletedKey(q): &expected.Completed} {
					n, e := c.ZCard(ctx, key).Result()
					if e != nil {
						t.Fatal(e)
					}
					*dst += int(n)
				}
				groups, e := c.SMembers(ctx, base.AllGroups(q)).Result()
				if e != nil {
					t.Fatal(e)
				}
				for _, g := range groups {
					n, e := c.ZCard(ctx, base.GroupKey(q, g)).Result()
					if e != nil {
						t.Fatal(e)
					}
					expected.Aggregating += int(n)
				}
			}
			got := decoded.Aggregate
			if got.Active != expected.Active || got.Pending != expected.Pending || got.Scheduled != expected.Scheduled || got.Retry != expected.Retry || got.Archived != expected.Archived || got.Completed != expected.Completed || got.Aggregating != expected.Aggregating {
				t.Fatalf("aggregate differs from independently counted Redis indexes: got%+v want%+v", got, expected)
			}
			for n, q := range queues {
				seen := false
				for _, s := range decoded.QueueStats {
					if s.Queue == q {
						seen = true
						if s.Pending != n+1 || s.Size != n+1 {
							t.Fatalf("owned queue stats counts %+v", s)
						}
					}
				}
				if !seen {
					t.Fatalf("owned queue missing JSON %s", q)
				}
			}
			output, e = invokeAdministration(t, statsCmd, func(cmd *cobra.Command, args []string) error {
				old := color.Output
				color.Output = os.Stdout
				defer func() { color.Output = old }()
				return stats(cmd, args)
			}, nil, map[string]string{"json": "false"})
			if e != nil {
				t.Fatal(e)
			}
			if !strings.Contains(output, "Redis Cluster Info") || !strings.Contains(output, "Known Nodes") || !strings.Contains(output, strings.ToUpper(decoded.RedisInfo["cluster_state"])) {
				t.Fatalf("human stats cluster mode lost: %s", output)
			}
		})
	}
}
