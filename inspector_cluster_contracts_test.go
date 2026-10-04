package asynq

import (
	"context"
	"errors"
	"fmt"
	"github.com/google/go-cmp/cmp"
	"github.com/redis/go-redis/v9"
	"os"
	"sort"
	"strings"
	"testing"
)

func inspectorContractSlot(key string) int64 {
	start, end := -1, -1
	for j := range key {
		if key[j] == '{' {
			start = j
			break
		}
	}
	if start >= 0 {
		for j := start + 1; j < len(key); j++ {
			if key[j] == '}' {
				end = j
				break
			}
		}
		if end > start+1 {
			key = key[start+1 : end]
		}
	}
	var crc uint16
	for j := 0; j < len(key); j++ {
		crc ^= uint16(key[j]) << 8
		for b := 0; b < 8; b++ {
			if crc&0x8000 != 0 {
				crc = (crc << 1) ^ 0x1021
			} else {
				crc <<= 1
			}
		}
	}
	return int64(crc % 16384)
}
func TestInspectorClusterReadOnlyContract(t *testing.T) {
	rawAddrs := os.Getenv("ASYNQ_INSPECTOR_CLUSTER_CONTRACT_ADDRS")
	if rawAddrs == "" {
		t.Skip("explicit registered cluster addresses/custody required")
	}
	addrs := strings.Split(rawAddrs, ",")
	for _, addr := range addrs {
		if strings.TrimSpace(addr) == "" {
			t.Fatal("cluster addresses must be nonempty")
		}
	}
	ctx := context.Background()
	oracle := redis.NewClusterClient(&redis.ClusterOptions{Addrs: addrs})
	t.Cleanup(func() {
		if e := oracle.Close(); e != nil {
			t.Error(e)
		}
	})
	slots, e := oracle.ClusterSlots(ctx).Result()
	if e != nil {
		t.Fatal(e)
	}
	inspector := NewInspector(RedisClusterClientOpt{Addrs: addrs})
	t.Cleanup(func() {
		if e := inspector.Close(); e != nil {
			t.Error(e)
		}
	})
	ownerSeen := map[string]bool{}
	seed := uint64(20261001)
	for index := 0; index < 32; index++ {
		seed = seed*6364136223846793005 + 1
		queue := fmt.Sprintf("readonly-inspector-%d-%016x", index, seed)
		wantSlot := inspectorContractSlot("asynq:{" + queue + "}:pending")
		got, e := inspector.ClusterKeySlot(queue)
		if e != nil || got != wantSlot {
			t.Fatalf("independent queue-slot assertion: got %d/%v want %d", got, e, wantSlot)
		}
		var want []string
		for _, slot := range slots {
			if int64(slot.Start) <= wantSlot && wantSlot <= int64(slot.End) {
				for _, n := range slot.Nodes {
					want = append(want, n.ID+"@"+n.Addr)
				}
				if len(slot.Nodes) > 0 {
					ownerSeen[slot.Nodes[0].ID] = true
				}
				break
			}
		}
		if len(want) == 0 {
			t.Fatalf("independent slots oracle lacks owner for %d", wantSlot)
		}
		nodes, e := inspector.ClusterNodes(queue)
		if e != nil {
			t.Fatal(e)
		}
		actual := []string{}
		for _, n := range nodes {
			actual = append(actual, n.ID+"@"+n.Addr)
		}
		sort.Strings(want)
		sort.Strings(actual)
		if !cmp.Equal(actual, want) {
			t.Fatalf("independent complete-node-set assertion: got %v want %v", actual, want)
		}
		// Literal brace-tag equivalence is tested against the actual public API.
		tagged := queue + "}suffix"
		tagSlot, e := inspector.ClusterKeySlot(tagged)
		wantTagged := wantSlot
		if e != nil || tagSlot != wantTagged {
			t.Fatalf("independent nested-tag slot assertion: got%d/%v want%d", tagSlot, e, wantTagged)
		}
	}
	if len(ownerSeen) != 3 {
		t.Fatalf("three-master selection assertion: selected %d want3", len(ownerSeen))
	}
}
func TestInspectorClusterClosedTransportContract(t *testing.T) {
	inspector := NewInspector(RedisClusterClientOpt{Addrs: []string{"127.0.0.1:1"}})
	if e := inspector.Close(); e != nil {
		t.Fatal(e)
	}
	slot, e := inspector.ClusterKeySlot("closed-contract")
	if !errors.Is(e, redis.ErrClosed) || slot != 0 {
		t.Fatalf("closed slot cause/no-success assertion: slot%d err%v", slot, e)
	}
	nodes, e := inspector.ClusterNodes("closed-contract")
	if !errors.Is(e, redis.ErrClosed) || nodes != nil {
		t.Fatalf("closed nodes cause/no-success assertion: nodes%v err%v", nodes, e)
	}
}
func FuzzInspectorClusterHashTagModel(f *testing.F) {
	f.Add([]byte("123456789"))
	f.Add([]byte{0, 255})
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 2048 {
			t.Skip()
		}
		tag := fmt.Sprintf("%x", b)
		if tag == "" {
			tag = "00"
		}
		a := inspectorContractSlot("left{" + tag + "}right")
		c := inspectorContractSlot("different{" + tag + "}tail")
		if a != c || a < 0 || a >= 16384 {
			t.Fatalf("hash-tag partition invariant: %d %d", a, c)
		}
	})
}
func TestInspectorCRC16PrimaryVector(t *testing.T) {
	if got := inspectorContractSlot("123456789"); got != 12739 {
		t.Fatalf("CRC16-XMODEM published check 0x31C3 modulo16384: got%d want12739", got)
	}
}

// The closed lazy client cannot perform network I/O; this exercises the actual APIs.
func FuzzInspectorClusterClosedTransport(f *testing.F) {
	inspector := NewInspector(RedisClusterClientOpt{Addrs: []string{"127.0.0.1:1"}})
	if err := inspector.Close(); err != nil {
		f.Fatal(err)
	}
	f.Add([]byte("default"))
	f.Add([]byte{})
	f.Add([]byte{0, 255, '{', '}'})
	f.Fuzz(func(t *testing.T, queue []byte) {
		if len(queue) > 2048 {
			t.Skip()
		}
		slot, err := inspector.ClusterKeySlot(string(queue))
		if !errors.Is(err, redis.ErrClosed) || slot != 0 {
			t.Fatalf("public fuzz closed-slot cause/no-success assertion: slot%d err%v", slot, err)
		}
		nodes, err := inspector.ClusterNodes(string(queue))
		if !errors.Is(err, redis.ErrClosed) || nodes != nil {
			t.Fatalf("public fuzz closed-nodes cause/no-success assertion: nodes%v err%v", nodes, err)
		}
	})
}
