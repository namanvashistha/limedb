package gossiper

import (
	"encoding/json"
	"fmt"
	"limedb/internal/logger"
	"limedb/internal/messenger"
	"limedb/internal/node"
	"math/rand"
	"slices"
	"sync"
	"time"
)

// peerState holds the last-known generation and version for one remote endpoint.
type peerState struct {
	generation int64
	version    int
}

// Gossiper implements SYN → ACK → ACK2 scuttlebutt reconciliation.
//
// Each node tracks:
//   - its own (generation, version) — generation is set once at startup, version
//     increments each gossip round.
//   - a (generation, version) pair for every known peer, updated whenever fresher
//     state arrives via gossip.
//
// Lag = peer's current version − our last-known version for that peer.
// This is purely a per-endpoint comparison, independent of the local counter.
type Gossiper struct {
	currentNodeUrl string
	generation     int64 // Unix epoch seconds at this node's startup — never changes
	version        int   // this node's own gossip round counter

	peers      []string
	peerStates map[string]peerState // keyed by peer URL
	messenger  *messenger.Messenger
	mu         sync.Mutex
	node       *node.NodeService
}

func NewGossiper(currentNodeUrl string, peers []string, msngr *messenger.Messenger, svc *node.NodeService) *Gossiper {
	validPeers := make([]string, 0, len(peers))
	for _, p := range peers {
		if p != currentNodeUrl {
			validPeers = append(validPeers, p)
		}
	}
	states := make(map[string]peerState, len(validPeers))
	for _, p := range validPeers {
		states[p] = peerState{}
	}
	return &Gossiper{
		currentNodeUrl: currentNodeUrl,
		generation:     time.Now().Unix(),
		version:        0,
		peers:          validPeers,
		peerStates:     states,
		messenger:      msngr,
		node:           svc,
	}
}

func (g *Gossiper) StartGossiping() {
	ticker := time.NewTicker(2 * time.Second)
	go func() {
		for range ticker.C {
			g.gossipRound()
		}
	}()
}

func (g *Gossiper) HandleGossip(requestBody []byte) map[string]interface{} {
	var msg GossipMessage
	if err := json.Unmarshal(requestBody, &msg); err != nil {
		return map[string]interface{}{"error": "invalid JSON"}
	}
	re, err := json.Marshal(msg.Payload)
	if err != nil {
		return map[string]interface{}{"error": "marshal payload"}
	}
	switch msg.Type {
	case "GOSSIP_SYN":
		var syn SynPayload
		if err := json.Unmarshal(re, &syn); err != nil {
			return map[string]interface{}{"error": "invalid SYN payload"}
		}
		ack := g.handleSyn(syn)
		return map[string]interface{}{"updates": ack.Updates, "requests": ack.Requests}
	case "GOSSIP_ACK2":
		var ack2 Ack2Payload
		if err := json.Unmarshal(re, &ack2); err != nil {
			return map[string]interface{}{"error": "invalid ACK2 payload"}
		}
		g.handleAck2(ack2)
		return map[string]interface{}{"status": "OK"}
	default:
		return map[string]interface{}{"error": "unknown type"}
	}
}

func (g *Gossiper) GetGossipMetrics() map[string]interface{} {
	g.mu.Lock()
	defer g.mu.Unlock()

	totalPeers := len(g.peers)
	if totalPeers == 0 {
		return map[string]interface{}{
			"node_url":         g.currentNodeUrl,
			"generation":       g.generation,
			"node_heartbeat":   g.version,
			"total_peers":      0,
			"active_peers":     0,
			"stale_peers":      0,
			"dead_peers":       0,
			"cluster_health":   "healthy",
			"convergence_rate": 100.0,
			"average_lag":      0.0,
			"max_lag":          0,
			"peer_details":     []interface{}{},
			"timestamp":        time.Now().Unix(),
		}
	}

	activePeers, stalePeers, deadPeers := 0, 0, 0
	totalLag, maxLag := 0, 0
	peerDetails := make([]map[string]interface{}, 0, totalPeers)

	for _, peer := range g.peers {
		ps := g.peerStates[peer]
		lag := 0
		status := "dead"
		if ps.generation > 0 {
			lag = g.version - ps.version
			if lag < 0 {
				lag = 0
			}
			switch {
			case lag <= 2:
				status = "active"
				activePeers++
			case lag <= 5:
				status = "stale"
				stalePeers++
			default:
				status = "dead"
				deadPeers++
			}
			totalLag += lag
			if lag > maxLag {
				maxLag = lag
			}
		} else {
			deadPeers++
		}
		peerDetails = append(peerDetails, map[string]interface{}{
			"url":        peer,
			"generation": ps.generation,
			"heartbeat":  ps.version,
			"lag":        lag,
			"status":     status,
		})
	}

	avgLag := 0.0
	if activePeers+stalePeers > 0 {
		avgLag = float64(totalLag) / float64(activePeers+stalePeers)
	}
	clusterHealth := "healthy"
	if deadPeers > totalPeers/2 {
		clusterHealth = "critical"
	} else if stalePeers > 0 || deadPeers > 0 {
		clusterHealth = "degraded"
	}

	return map[string]interface{}{
		"node_url":         g.currentNodeUrl,
		"generation":       g.generation,
		"node_heartbeat":   g.version,
		"total_peers":      totalPeers,
		"active_peers":     activePeers,
		"stale_peers":      stalePeers,
		"dead_peers":       deadPeers,
		"cluster_health":   clusterHealth,
		"convergence_rate": float64(activePeers) / float64(totalPeers) * 100,
		"average_lag":      avgLag,
		"max_lag":          maxLag,
		"peer_details":     peerDetails,
		"timestamp":        time.Now().Unix(),
	}
}

func (g *Gossiper) gossipRound() {
	g.mu.Lock()
	g.version++
	target := g.randomPeer()
	syn := SynPayload{Digests: g.buildDigests()}
	g.mu.Unlock()

	if target == "" {
		return
	}

	logger.Info("Gossip SYN", "to", target, "gen", g.generation, "ver", g.version)

	msg := GossipMessage{Type: "GOSSIP_SYN", Payload: syn}
	raw, _ := json.Marshal(msg)
	m := messenger.NewMessage("GOSSIP_SYN", raw, g.currentNodeUrl, fmt.Sprintf("%s/gossip", target))
	if err := g.messenger.SendMessage(m); err != nil {
		logger.Warn("Gossip SYN failed", "peer", target, "error", err)
	}
}

func (g *Gossiper) buildDigests() []EndpointDigest {
	digests := make([]EndpointDigest, 0, len(g.peers)+1)
	digests = append(digests, EndpointDigest{
		NodeURL:    g.currentNodeUrl,
		Generation: g.generation,
		Version:    g.version,
	})
	for _, peer := range g.peers {
		ps := g.peerStates[peer]
		digests = append(digests, EndpointDigest{
			NodeURL:    peer,
			Generation: ps.generation,
			Version:    ps.version,
		})
	}
	return digests
}

func (g *Gossiper) handleSyn(payload SynPayload) AckPayload {
	g.mu.Lock()
	defer g.mu.Unlock()

	updates := make([]EndpointDigest, 0)
	requests := make([]EndpointDigest, 0)

	for _, d := range payload.Digests {
		if d.NodeURL == g.currentNodeUrl {
			continue
		}
		if _, known := g.peerStates[d.NodeURL]; !known {
			g.peers = append(g.peers, d.NodeURL)
			g.peerStates[d.NodeURL] = peerState{}
			logger.Info("New peer discovered via gossip", "peer", d.NodeURL)
		}
		if !slices.Contains(g.node.GetRing().GetNodes(), d.NodeURL) {
			g.node.GetRing().AddNode(d.NodeURL)
		}

		local := g.peerStates[d.NodeURL]
		switch comparePeer(d, local) {
		case newer:
			g.peerStates[d.NodeURL] = peerState{generation: d.Generation, version: d.Version}
			updates = append(updates, d)
			logger.Info("Peer state updated", "peer", d.NodeURL, "gen", d.Generation, "ver", d.Version)
		case older:
			requests = append(requests, EndpointDigest{
				NodeURL:    d.NodeURL,
				Generation: local.generation,
				Version:    local.version,
			})
		}
	}
	return AckPayload{Updates: updates, Requests: requests}
}

func (g *Gossiper) handleAck2(payload Ack2Payload) {
	g.mu.Lock()
	defer g.mu.Unlock()

	for _, d := range payload.Updates {
		if d.NodeURL == g.currentNodeUrl {
			continue
		}
		local := g.peerStates[d.NodeURL]
		if comparePeer(d, local) == newer {
			g.peerStates[d.NodeURL] = peerState{generation: d.Generation, version: d.Version}
			logger.Info("ACK2 state applied", "peer", d.NodeURL, "gen", d.Generation, "ver", d.Version)
		}
	}
}

func (g *Gossiper) randomPeer() string {
	if len(g.peers) == 0 {
		return ""
	}
	return g.peers[rand.Intn(len(g.peers))]
}

type cmp int

const (
	older cmp = iota - 1
	equal
	newer
)

// comparePeer: higher generation always wins (handles node restarts).
// Within the same generation, higher version wins.
func comparePeer(d EndpointDigest, local peerState) cmp {
	switch {
	case d.Generation > local.generation:
		return newer
	case d.Generation < local.generation:
		return older
	case d.Version > local.version:
		return newer
	case d.Version < local.version:
		return older
	default:
		return equal
	}
}
