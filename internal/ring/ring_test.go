package ring

import (
	"fmt"
	"testing"
)

func newTestRing(nodes ...string) *ConsistentHashRing {
	r := New(8)
	for _, n := range nodes {
		r.AddNode(n)
	}
	return r
}

func TestRing_GetNode_Deterministic(t *testing.T) {
	r := newTestRing("http://n1", "http://n2", "http://n3")

	for i := 0; i < 100; i++ {
		key := fmt.Sprintf("key-%d", i)
		first := r.GetNode(key)
		if first == "" {
			t.Fatalf("GetNode(%q) returned empty node", key)
		}
		for j := 0; j < 5; j++ {
			if got := r.GetNode(key); got != first {
				t.Fatalf("GetNode(%q) not deterministic: %q vs %q", key, got, first)
			}
		}
	}
}

func TestRing_GetNode_EmptyRing(t *testing.T) {
	r := New(8)
	if got := r.GetNode("anything"); got != "" {
		t.Errorf("empty ring should return \"\", got %q", got)
	}
}

func TestRing_GetReplicas_UniqueNodes(t *testing.T) {
	r := newTestRing("http://n1", "http://n2", "http://n3", "http://n4")

	for i := 0; i < 50; i++ {
		key := fmt.Sprintf("key-%d", i)
		replicas := r.GetReplicas(key, 3)
		if len(replicas) != 3 {
			t.Fatalf("GetReplicas(%q, 3): got %d replicas: %v", key, len(replicas), replicas)
		}
		seen := map[string]bool{}
		for _, n := range replicas {
			if seen[n] {
				t.Fatalf("GetReplicas(%q) returned duplicate node %q: %v", key, n, replicas)
			}
			seen[n] = true
		}
		// The primary must match GetNode.
		if replicas[0] != r.GetNode(key) {
			t.Errorf("GetReplicas(%q)[0] = %q, want GetNode result %q", key, replicas[0], r.GetNode(key))
		}
	}
}

func TestRing_GetReplicas_RFLargerThanCluster(t *testing.T) {
	r := newTestRing("http://n1", "http://n2")
	replicas := r.GetReplicas("k", 5)
	if len(replicas) > 2 {
		t.Errorf("replicas cannot exceed physical node count: %v", replicas)
	}
}

func TestRing_RemoveNode(t *testing.T) {
	r := newTestRing("http://n1", "http://n2", "http://n3")
	r.RemoveNode("http://n2")

	for i := 0; i < 100; i++ {
		key := fmt.Sprintf("key-%d", i)
		if got := r.GetNode(key); got == "http://n2" {
			t.Fatalf("GetNode(%q) routed to removed node", key)
		}
	}

	nodes := r.GetNodes()
	if len(nodes) != 2 {
		t.Errorf("GetNodes after removal: got %v", nodes)
	}

	// Removing an unknown node is a no-op.
	r.RemoveNode("http://ghost")
	if len(r.GetNodes()) != 2 {
		t.Error("removing unknown node should not change the ring")
	}
}

func TestRing_KeysRedistributeOnRemoval(t *testing.T) {
	r := newTestRing("http://n1", "http://n2", "http://n3")

	before := map[string]string{}
	for i := 0; i < 200; i++ {
		key := fmt.Sprintf("key-%d", i)
		before[key] = r.GetNode(key)
	}

	r.RemoveNode("http://n3")

	for key, owner := range before {
		got := r.GetNode(key)
		if owner == "http://n3" {
			if got == "http://n3" || got == "" {
				t.Fatalf("key %q not re-routed after owner removal: %q", key, got)
			}
		} else if got != owner {
			// Consistent hashing: keys not owned by the removed node must not move.
			t.Fatalf("key %q moved from %q to %q despite owner staying", key, owner, got)
		}
	}
}

func TestHashKey_MatchesRingHash(t *testing.T) {
	r := New(1)
	for _, k := range []string{"", "a", "hello world", "key-42"} {
		if HashKey(k) != r.hash(k) {
			t.Errorf("HashKey(%q) diverges from ring hash", k)
		}
	}
}
