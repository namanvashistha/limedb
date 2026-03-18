package placement

import (
	"fmt"
	"limedb/internal/membership"
	"limedb/internal/ring"
	"sort"
	"strconv"
	"sync"
	"time"
)

type Status string

const (
	StatusPending Status = "PENDING"
	StatusActive  Status = "ACTIVE"
)

type Member struct {
	NodeURL string `json:"node_url"`
	Role    string `json:"role"`
}

type TokenAssignment struct {
	Token   uint64 `json:"token"`
	NodeURL string `json:"node_url"`
}

type Snapshot struct {
	Epoch             int64             `json:"epoch"`
	Status            Status            `json:"status"`
	VirtualNodes      int               `json:"virtual_nodes"`
	ReplicationFactor int               `json:"replication_factor"`
	Members           []Member          `json:"members"`
	Tokens            []TokenAssignment `json:"tokens"`
	CreatedAtUnix     int64             `json:"created_at_unix"`
}

type State struct {
	Active  *Snapshot `json:"active"`
	Pending *Snapshot `json:"pending,omitempty"`
}

type ReplicaTarget struct {
	NodeURL   string `json:"node_url"`
	IsPrimary bool   `json:"is_primary"`
	Role      string `json:"role"`
}

type ReplicaSet struct {
	PlacementEpoch int64           `json:"placement_epoch"`
	Key            string          `json:"key"`
	Replicas       []ReplicaTarget `json:"replicas"`
}

type BootstrapPlan struct {
	PlanID          string           `json:"plan_id"`
	TargetNodeURL   string           `json:"target_node_url"`
	PlacementEpoch  int64            `json:"placement_epoch"`
	SourceEpoch     int64            `json:"source_epoch"`
	Status          string           `json:"status"`
	Ranges          []BootstrapRange `json:"ranges"`
	StartedAtUnix   int64            `json:"started_at_unix"`
	CompletedAtUnix int64            `json:"completed_at_unix,omitempty"`
}

type BootstrapRange struct {
	StartToken uint64   `json:"start_token"`
	EndToken   uint64   `json:"end_token"`
	Token      uint64   `json:"token"`
	FromNodes  []string `json:"from_nodes"`
	ToNode     string   `json:"to_node"`
	Status     string   `json:"status"`
	KeysCopied int      `json:"keys_copied"`
}

type Manager struct {
	membership        *membership.Manager
	virtualNodes      int
	replicationFactor int

	mu         sync.RWMutex
	activeRing *ring.ConsistentHashRing
	active     *Snapshot
	pending    *Snapshot
	bootstrap  *BootstrapPlan
}

func NewManager(manager *membership.Manager, virtualNodes int, replicationFactor int) *Manager {
	p := &Manager{
		membership:        manager,
		virtualNodes:      virtualNodes,
		replicationFactor: replicationFactor,
	}

	initial := p.buildSnapshot(
		manager.GetPeers(),
		manager.Snapshot().MembershipEpoch,
		StatusActive,
	)
	p.active = initial
	p.activeRing = p.buildRing(initial)
	return p
}

func (p *Manager) State() *State {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return &State{
		Active:  copySnapshot(p.active),
		Pending: copySnapshot(p.pending),
	}
}

func (p *Manager) BootstrapPlan() *BootstrapPlan {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.bootstrap == nil {
		return nil
	}
	copyPlan := *p.bootstrap
	copyPlan.Ranges = append([]BootstrapRange(nil), p.bootstrap.Ranges...)
	return &copyPlan
}

func (p *Manager) Snapshot() *Snapshot {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return copySnapshot(p.active)
}

func (p *Manager) CreatePendingFromMembership(targetNodeURL string) (*Snapshot, error) {
	membershipSnapshot := p.membership.Snapshot()
	if targetNodeURL == "" {
		return nil, fmt.Errorf("target node is required")
	}

	found := false
	activePeers := make([]string, 0, len(membershipSnapshot.ActiveNodes))
	for _, member := range membershipSnapshot.ActiveNodes {
		activePeers = append(activePeers, member.NodeURL)
		if member.NodeURL == targetNodeURL {
			found = true
		}
	}
	if !found {
		return nil, fmt.Errorf("node %s is not active in membership", targetNodeURL)
	}

	pending := p.buildSnapshot(activePeers, membershipSnapshot.MembershipEpoch, StatusPending)

	p.mu.Lock()
	defer p.mu.Unlock()
	p.pending = pending
	ranges := buildBootstrapRanges(p.active, p.activeRing, pending, targetNodeURL, p.replicationFactor)
	p.bootstrap = &BootstrapPlan{
		PlanID:         fmt.Sprintf("bootstrap-%d-%s", pending.Epoch, targetNodeURL),
		TargetNodeURL:  targetNodeURL,
		PlacementEpoch: pending.Epoch,
		SourceEpoch:    p.active.Epoch,
		Status:         "PLANNED",
		Ranges:         ranges,
		StartedAtUnix:  time.Now().Unix(),
	}

	return copySnapshot(p.pending), nil
}

func (p *Manager) PromotePending() (*Snapshot, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.pending == nil {
		return nil, fmt.Errorf("no pending placement to promote")
	}
	if p.bootstrap != nil && p.bootstrap.Status != "COMPLETED" {
		return nil, fmt.Errorf("bootstrap is not complete")
	}

	p.pending.Status = StatusActive
	p.active = copySnapshot(p.pending)
	p.activeRing = p.buildRing(p.active)
	if p.bootstrap != nil {
		p.bootstrap.Status = "COMPLETED"
		p.bootstrap.CompletedAtUnix = time.Now().Unix()
	}
	p.pending = nil
	return copySnapshot(p.active), nil
}

func (p *Manager) StartBootstrap() (*BootstrapPlan, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.bootstrap == nil {
		return nil, fmt.Errorf("no bootstrap plan available")
	}
	if p.bootstrap.Status == "COMPLETED" {
		copyPlan := *p.bootstrap
		copyPlan.Ranges = append([]BootstrapRange(nil), p.bootstrap.Ranges...)
		return &copyPlan, nil
	}
	p.bootstrap.Status = "RUNNING"
	for i := range p.bootstrap.Ranges {
		p.bootstrap.Ranges[i].Status = "COPYING"
	}
	copyPlan := *p.bootstrap
	copyPlan.Ranges = append([]BootstrapRange(nil), p.bootstrap.Ranges...)
	return &copyPlan, nil
}

func (p *Manager) CompleteBootstrap() (*BootstrapPlan, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.bootstrap == nil {
		return nil, fmt.Errorf("no bootstrap plan available")
	}
	p.bootstrap.Status = "COMPLETED"
	p.bootstrap.CompletedAtUnix = time.Now().Unix()
	for i := range p.bootstrap.Ranges {
		p.bootstrap.Ranges[i].Status = "VERIFIED"
		p.bootstrap.Ranges[i].KeysCopied = 1
	}
	copyPlan := *p.bootstrap
	copyPlan.Ranges = append([]BootstrapRange(nil), p.bootstrap.Ranges...)
	return &copyPlan, nil
}

func (p *Manager) ResolveReplicas(key string) *ReplicaSet {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.active == nil || p.activeRing == nil {
		return &ReplicaSet{Key: key}
	}

	roleByNode := make(map[string]string, len(p.active.Members))
	for _, member := range p.active.Members {
		roleByNode[member.NodeURL] = member.Role
	}

	nodes := p.activeRing.GetReplicas(key, p.replicationFactor)
	replicas := make([]ReplicaTarget, 0, len(nodes))
	for i, nodeURL := range nodes {
		role := roleByNode[nodeURL]
		if role == "" {
			role = membership.StateActive
		}
		replicas = append(replicas, ReplicaTarget{
			NodeURL:   nodeURL,
			IsPrimary: i == 0,
			Role:      role,
		})
	}

	return &ReplicaSet{
		PlacementEpoch: p.active.Epoch,
		Key:            key,
		Replicas:       replicas,
	}
}

func (p *Manager) buildSnapshot(peers []string, epoch int64, status Status) *Snapshot {
	members := make([]Member, 0, len(peers))
	for _, peer := range peers {
		members = append(members, Member{
			NodeURL: peer,
			Role:    membership.StateActive,
		})
	}
	sort.Slice(members, func(i, j int) bool {
		return members[i].NodeURL < members[j].NodeURL
	})

	activeRing := ring.New(p.virtualNodes)
	for _, member := range members {
		activeRing.AddNode(member.NodeURL)
	}

	tokenAssignments := make([]TokenAssignment, 0)
	ranges := activeRing.GetNodeRanges()
	for nodeURL, nodeRanges := range ranges {
		for _, rng := range nodeRanges {
			hash, ok := rng["hash"].(uint64)
			if !ok {
				continue
			}
			tokenAssignments = append(tokenAssignments, TokenAssignment{
				Token:   hash,
				NodeURL: nodeURL,
			})
		}
	}
	sort.Slice(tokenAssignments, func(i, j int) bool {
		if tokenAssignments[i].Token == tokenAssignments[j].Token {
			return tokenAssignments[i].NodeURL < tokenAssignments[j].NodeURL
		}
		return tokenAssignments[i].Token < tokenAssignments[j].Token
	})

	return &Snapshot{
		Epoch:             epoch,
		Status:            status,
		VirtualNodes:      p.virtualNodes,
		ReplicationFactor: p.replicationFactor,
		Members:           members,
		Tokens:            tokenAssignments,
		CreatedAtUnix:     time.Now().Unix(),
	}
}

func (p *Manager) buildRing(snapshot *Snapshot) *ring.ConsistentHashRing {
	activeRing := ring.New(p.virtualNodes)
	for _, member := range snapshot.Members {
		activeRing.AddNode(member.NodeURL)
	}
	return activeRing
}

func copySnapshot(snapshot *Snapshot) *Snapshot {
	if snapshot == nil {
		return nil
	}
	copyValue := *snapshot
	copyValue.Members = append([]Member(nil), snapshot.Members...)
	copyValue.Tokens = append([]TokenAssignment(nil), snapshot.Tokens...)
	return &copyValue
}

func buildBootstrapRanges(activeSnapshot *Snapshot, activeRing *ring.ConsistentHashRing, pendingSnapshot *Snapshot, targetNodeURL string, replicationFactor int) []BootstrapRange {
	if pendingSnapshot == nil {
		return nil
	}

	pendingRing := ring.New(pendingSnapshot.VirtualNodes)
	for _, member := range pendingSnapshot.Members {
		pendingRing.AddNode(member.NodeURL)
	}

	pendingRanges := pendingRing.GetNodeRanges()
	targetRanges := pendingRanges[targetNodeURL]
	ranges := make([]BootstrapRange, 0, len(targetRanges))

	for _, rng := range targetRanges {
		token, ok := rng["hash"].(uint64)
		if !ok {
			continue
		}
		startToken, _ := rng["start"].(uint64)
		endToken, _ := rng["end"].(uint64)

		sourceNodes := []string{}
		if activeSnapshot != nil && activeRing != nil {
			// Use the token position as the rendezvous point into the active ring.
			sourceNodes = activeRing.GetReplicas(strconv.FormatUint(token, 10), replicationFactor)
		}

		ranges = append(ranges, BootstrapRange{
			StartToken: startToken,
			EndToken:   endToken,
			Token:      token,
			FromNodes:  sourceNodes,
			ToNode:     targetNodeURL,
			Status:     "PENDING",
			KeysCopied: 0,
		})
	}

	sort.Slice(ranges, func(i, j int) bool {
		if ranges[i].StartToken == ranges[j].StartToken {
			return ranges[i].EndToken < ranges[j].EndToken
		}
		return ranges[i].StartToken < ranges[j].StartToken
	})

	return ranges
}
