package membership

import (
	"fmt"
	"limedb/internal/ring"
	"sort"
	"sync"
	"time"
)

const (
	StateDiscovered    = "DISCOVERED"
	StateBootstrapping = "BOOTSTRAPPING"
	StateActive        = "ACTIVE"
	StateLeaving       = "LEAVING"
	StateLeft          = "LEFT"
)

const (
	LivenessUnknown = "unknown"
	LivenessAlive   = "alive"
	LivenessStale   = "stale"
	LivenessDead    = "dead"
)

// Member is the canonical membership record for one node.
type Member struct {
	NodeURL             string `json:"node_url"`
	State               string `json:"state"`
	IsLocal             bool   `json:"is_local"`
	IsActiveForRouting  bool   `json:"is_active_for_routing"`
	Generation          int64  `json:"generation"`
	Version             int    `json:"version"`
	LastSeenUnix        int64  `json:"last_seen_unix"`
	Liveness            string `json:"liveness"`
	ActivationEpoch     int64  `json:"activation_epoch,omitempty"`
	TransitionRequested bool   `json:"transition_requested"`
}

// Snapshot is the admin-facing immutable membership view.
type Snapshot struct {
	CurrentNodeURL  string   `json:"current_node_url"`
	MembershipEpoch int64    `json:"membership_epoch"`
	ActiveNodes     []Member `json:"active_nodes"`
	ObservedNodes   []Member `json:"observed_nodes"`
	DesiredNodes    []Member `json:"desired_nodes"`
}

// Manager owns cluster membership state and the active routing ring.
type Manager struct {
	currentNodeURL string

	mu              sync.RWMutex
	ring            *ring.ConsistentHashRing
	membershipEpoch int64
	members         map[string]*Member
	desiredActive   map[string]bool
}

// NewManager creates membership state and seeds active membership from startup peers.
func NewManager(currentNodeURL string, virtualNodes int, peers []string) *Manager {
	activeRing := ring.New(virtualNodes)
	manager := &Manager{
		currentNodeURL:  currentNodeURL,
		ring:            activeRing,
		membershipEpoch: 1,
		members:         make(map[string]*Member),
		desiredActive:   make(map[string]bool),
	}

	now := time.Now().Unix()
	manager.members[currentNodeURL] = &Member{
		NodeURL:            currentNodeURL,
		State:              StateActive,
		IsLocal:            true,
		IsActiveForRouting: true,
		LastSeenUnix:       now,
		Liveness:           LivenessAlive,
		ActivationEpoch:    manager.membershipEpoch,
	}
	manager.desiredActive[currentNodeURL] = true
	activeRing.AddNode(currentNodeURL)

	for _, peer := range peers {
		if peer == "" || peer == currentNodeURL {
			continue
		}
		manager.members[peer] = &Member{
			NodeURL:            peer,
			State:              StateActive,
			IsLocal:            false,
			IsActiveForRouting: true,
			LastSeenUnix:       now,
			Liveness:           LivenessUnknown,
			ActivationEpoch:    manager.membershipEpoch,
		}
		manager.desiredActive[peer] = true
		activeRing.AddNode(peer)
	}

	return manager
}

func (m *Manager) CurrentNodeURL() string {
	return m.currentNodeURL
}

// ObserveNode records a node learned through gossip without activating it for routing.
func (m *Manager) ObserveNode(nodeURL string) bool {
	if nodeURL == "" {
		return false
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.members[nodeURL]; ok {
		return false
	}

	m.members[nodeURL] = &Member{
		NodeURL:            nodeURL,
		State:              StateDiscovered,
		IsLocal:            nodeURL == m.currentNodeURL,
		IsActiveForRouting: false,
		LastSeenUnix:       time.Now().Unix(),
		Liveness:           LivenessUnknown,
	}
	return true
}

// RecordObservation updates the latest gossip observation for a node.
func (m *Manager) RecordObservation(nodeURL string, generation int64, version int) {
	if nodeURL == "" {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	member, ok := m.members[nodeURL]
	if !ok {
		member = &Member{
			NodeURL:            nodeURL,
			State:              StateDiscovered,
			IsLocal:            nodeURL == m.currentNodeURL,
			IsActiveForRouting: false,
		}
		m.members[nodeURL] = member
	}

	member.Generation = generation
	member.Version = version
	member.LastSeenUnix = time.Now().Unix()
	member.Liveness = LivenessAlive
}

// RequestActivation marks operator intent to move a discovered node into routing.
func (m *Manager) RequestActivation(nodeURL string) error {
	if nodeURL == "" {
		return fmt.Errorf("node_url is required")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	member, ok := m.members[nodeURL]
	if !ok {
		return fmt.Errorf("node %s has not been discovered", nodeURL)
	}
	if member.IsActiveForRouting {
		return fmt.Errorf("node %s is already active", nodeURL)
	}
	if member.State == StateLeft {
		return fmt.Errorf("node %s has already left the cluster", nodeURL)
	}

	member.TransitionRequested = true
	member.State = StateBootstrapping
	m.desiredActive[nodeURL] = true
	return nil
}

// MarkNodeActive finalizes activation into the current routing epoch.
func (m *Manager) MarkNodeActive(nodeURL string) error {
	if nodeURL == "" {
		return fmt.Errorf("node_url is required")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	member, ok := m.members[nodeURL]
	if !ok {
		return fmt.Errorf("node %s has not been discovered", nodeURL)
	}
	if member.IsActiveForRouting {
		return fmt.Errorf("node %s is already active", nodeURL)
	}
	if !member.TransitionRequested && member.State != StateBootstrapping {
		return fmt.Errorf("node %s has not been requested for activation", nodeURL)
	}

	m.membershipEpoch++
	member.IsActiveForRouting = true
	member.State = StateActive
	member.TransitionRequested = false
	member.ActivationEpoch = m.membershipEpoch
	m.desiredActive[nodeURL] = true
	m.ring.AddNode(nodeURL)
	return nil
}

// ActivateNode preserves the current admin API while routing through explicit transitions.
func (m *Manager) ActivateNode(nodeURL string) error {
	if err := m.RequestActivation(nodeURL); err != nil {
		return err
	}
	return m.MarkNodeActive(nodeURL)
}

// GetPeers returns the active membership set in stable order.
func (m *Manager) GetPeers() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	peers := make([]string, 0)
	for nodeURL, member := range m.members {
		if member.IsActiveForRouting {
			peers = append(peers, nodeURL)
		}
	}
	sort.Strings(peers)
	return peers
}

// GetRing returns the active routing ring.
func (m *Manager) GetRing() *ring.ConsistentHashRing {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.ring
}

// Snapshot returns an immutable membership view.
func (m *Manager) Snapshot() *Snapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()

	observed := make([]Member, 0, len(m.members))
	active := make([]Member, 0, len(m.members))
	desired := make([]Member, 0, len(m.members))

	for _, member := range m.members {
		copyMember := *member
		observed = append(observed, copyMember)
		if member.IsActiveForRouting {
			active = append(active, copyMember)
		}
		if m.desiredActive[member.NodeURL] {
			desired = append(desired, copyMember)
		}
	}

	sortMembers := func(members []Member) {
		sort.Slice(members, func(i, j int) bool {
			return members[i].NodeURL < members[j].NodeURL
		})
	}
	sortMembers(observed)
	sortMembers(active)
	sortMembers(desired)

	return &Snapshot{
		CurrentNodeURL:  m.currentNodeURL,
		MembershipEpoch: m.membershipEpoch,
		ObservedNodes:   observed,
		ActiveNodes:     active,
		DesiredNodes:    desired,
	}
}

// GetState is a backward-compatible alias retained for existing callers.
func (m *Manager) GetState() *Snapshot {
	return m.Snapshot()
}
