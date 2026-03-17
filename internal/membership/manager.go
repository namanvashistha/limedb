package membership

import (
	"fmt"
	"limedb/internal/ring"
	"sort"
	"sync"
)

// Node describes one node's membership state.
type Node struct {
	NodeURL string `json:"node_url"`
	State   string `json:"state"`
	IsLocal bool   `json:"is_local"`
}

// State is the admin-facing view of observed vs active membership.
type State struct {
	CurrentNodeURL string `json:"current_node_url"`
	ObservedNodes  []Node `json:"observed_nodes"`
	ActiveNodes    []Node `json:"active_nodes"`
}

// Manager owns cluster membership and the active routing ring.
type Manager struct {
	currentNodeURL string

	mu            sync.RWMutex
	ring          *ring.ConsistentHashRing
	observedNodes map[string]bool
	activeNodes   map[string]bool
}

// NewManager creates membership state and seeds active membership from startup peers.
func NewManager(currentNodeURL string, virtualNodes int, peers []string) *Manager {
	activeRing := ring.New(virtualNodes)
	observed := map[string]bool{currentNodeURL: true}
	active := map[string]bool{currentNodeURL: true}

	activeRing.AddNode(currentNodeURL)
	for _, peer := range peers {
		if peer == "" || peer == currentNodeURL {
			continue
		}
		observed[peer] = true
		active[peer] = true
		activeRing.AddNode(peer)
	}

	return &Manager{
		currentNodeURL: currentNodeURL,
		ring:           activeRing,
		observedNodes:  observed,
		activeNodes:    active,
	}
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

	if m.observedNodes[nodeURL] {
		return false
	}

	m.observedNodes[nodeURL] = true
	return true
}

// ActivateNode promotes an observed node into active membership and the routing ring.
func (m *Manager) ActivateNode(nodeURL string) error {
	if nodeURL == "" {
		return fmt.Errorf("node_url is required")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.observedNodes[nodeURL] {
		return fmt.Errorf("node %s has not been discovered", nodeURL)
	}
	if m.activeNodes[nodeURL] {
		return fmt.Errorf("node %s is already active", nodeURL)
	}

	m.activeNodes[nodeURL] = true
	m.ring.AddNode(nodeURL)
	return nil
}

// GetPeers returns the active membership set in stable order.
func (m *Manager) GetPeers() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	peers := make([]string, 0, len(m.activeNodes))
	for nodeURL := range m.activeNodes {
		peers = append(peers, nodeURL)
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

// GetState returns observed and active membership as separate sets.
func (m *Manager) GetState() *State {
	m.mu.RLock()
	defer m.mu.RUnlock()

	observed := make([]Node, 0, len(m.observedNodes))
	for nodeURL := range m.observedNodes {
		state := "DISCOVERED"
		if m.activeNodes[nodeURL] {
			state = "ACTIVE"
		}
		observed = append(observed, Node{
			NodeURL: nodeURL,
			State:   state,
			IsLocal: nodeURL == m.currentNodeURL,
		})
	}

	active := make([]Node, 0, len(m.activeNodes))
	for nodeURL := range m.activeNodes {
		active = append(active, Node{
			NodeURL: nodeURL,
			State:   "ACTIVE",
			IsLocal: nodeURL == m.currentNodeURL,
		})
	}

	sort.Slice(observed, func(i, j int) bool {
		return observed[i].NodeURL < observed[j].NodeURL
	})
	sort.Slice(active, func(i, j int) bool {
		return active[i].NodeURL < active[j].NodeURL
	})

	return &State{
		CurrentNodeURL: m.currentNodeURL,
		ObservedNodes:  observed,
		ActiveNodes:    active,
	}
}
