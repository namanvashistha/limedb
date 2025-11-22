package gossiper

import (
	"encoding/json"
	"fmt"
	"limedb/internal/logger"
	"limedb/internal/messenger"
	"sync"
	"time"

	"math/rand"
)

type Gossiper struct {
	currentNodeUrl string
	peers          []string
	heartbeat      int
	peerHeartbeats map[string]int
	messenger      *messenger.Messenger
	mu             sync.Mutex
}

func NewGossiper(currentNodeUrl string, peers []string, messenger *messenger.Messenger) *Gossiper {
	// Filter out self from peers
	validPeers := make([]string, 0)
	for _, peer := range peers {
		if peer != currentNodeUrl {
			validPeers = append(validPeers, peer)
		}
	}

	peerHeartbeats := make(map[string]int)
	for _, peer := range validPeers {
		peerHeartbeats[peer] = 0
	}

	return &Gossiper{
		currentNodeUrl: currentNodeUrl,
		peers:          validPeers,
		heartbeat:      0,
		peerHeartbeats: peerHeartbeats,
		messenger:      messenger,
		mu:             sync.Mutex{},
	}
}

func (g *Gossiper) HandleGossip(requestBody []byte) map[string]interface{} {
	// Parse the incoming gossip message
	var gossipMsg GossipMessage
	if err := json.Unmarshal(requestBody, &gossipMsg); err != nil {
		logger.Error("Invalid gossip message JSON",
			"error", err.Error(),
			"request_size", len(requestBody),
		)
		return map[string]interface{}{"error": "Invalid JSON"}
	}

	logger.Info("Received gossip message",
		"type", gossipMsg.Type,
		"request_size", len(requestBody),
	)

	switch gossipMsg.Type {
	case "GOSSIP_SYN":
		var synPayload SynPayload
		payloadBytes, err := json.Marshal(gossipMsg.Payload)
		if err != nil {
			logger.Error("Failed to marshal SYN payload", "error", err.Error())
			return map[string]interface{}{"error": "Failed to marshal payload"}
		}
		if err := json.Unmarshal(payloadBytes, &synPayload); err != nil {
			logger.Error("Invalid SYN payload", "error", err.Error(), "payload", string(payloadBytes))
			return map[string]interface{}{"error": "Invalid SYN payload"}
		}

		// Handle SYN and return ACK response
		ackPayload := g.handleSyn(synPayload)
		logger.Info("Sending ACK response",
			"updates_to_send", len(ackPayload.DigestsUpdate),
			"requests_to_send", len(ackPayload.DigestsRequest),
			"received_digests", len(synPayload.Digests),
		)
		return map[string]interface{}{
			"digestsUpdate":  ackPayload.DigestsUpdate,
			"digestsRequest": ackPayload.DigestsRequest,
		}

	case "GOSSIP_ACK2":
		var ack2Payload Ack2Payload
		payloadBytes, err := json.Marshal(gossipMsg.Payload)
		if err != nil {
			logger.Error("Failed to marshal ACK2 payload", "error", err.Error())
			return map[string]interface{}{"error": "Failed to marshal payload"}
		}
		if err := json.Unmarshal(payloadBytes, &ack2Payload); err != nil {
			logger.Error("Invalid ACK2 payload", "error", err.Error(), "payload", string(payloadBytes))
			return map[string]interface{}{"error": "Invalid ACK2 payload"}
		}

		// Handle ACK2
		g.handleAck2(ack2Payload)
		logger.Info("Processed ACK2",
			"updates_applied", len(ack2Payload.DigestsUpdate),
			"gossip_cycle_complete", true,
		)
		return map[string]interface{}{"status": "OK"}

	default:
		logger.Warn("Unknown gossip message type", "type", gossipMsg.Type)
		return map[string]interface{}{"error": "Unknown message type"}
	}
}
func (g *Gossiper) StartGossiping() {
	gossipTicker := time.NewTicker(30 * time.Second)
	summaryTicker := time.NewTicker(60 * time.Second)

	go func() {
		for {
			select {
			case <-gossipTicker.C:
				g.gossipRound()
			case <-summaryTicker.C:
				g.logClusterHealth()
			}
		}
	}()
}

func (g *Gossiper) gossipRound() {
	g.mu.Lock()
	g.heartbeat++ // ← increase here ONCE per round
	currentHeartbeat := g.heartbeat
	peersSnapshot := make(map[string]int)
	for k, v := range g.peerHeartbeats {
		peersSnapshot[k] = v
	}
	g.mu.Unlock()

	// Enhanced gossip summary with metrics
	totalPeers := len(g.peers)
	activePeers := 0
	stalePeers := 0

	for _, heartbeat := range peersSnapshot {
		if heartbeat > 0 {
			activePeers++
			if currentHeartbeat-heartbeat > 10 { // Consider stale if >10 heartbeats behind
				stalePeers++
			}
		}
	}

	logger.Info("Gossip Round Summary",
		"round", currentHeartbeat,
		"node", g.currentNodeUrl,
		"total_peers", totalPeers,
		"active_peers", activePeers,
		"stale_peers", stalePeers,
		"peer_heartbeats", peersSnapshot,
	)

	if len(g.peers) == 0 {
		logger.Error("No peers configured for gossiping - skipping round")
		return
	}

	peer := g.peers[rand.Intn(len(g.peers))]
	logger.Info("Initiating gossip exchange",
		"target_peer", peer,
		"my_heartbeat", currentHeartbeat,
		"target_heartbeat", peersSnapshot[peer],
	)

	synPayload := SynPayload{
		Digests: g.buildDigests(),
	}

	// Create gossip message and send via messenger
	gossipMsg := GossipMessage{
		Type:    "GOSSIP_SYN",
		Payload: synPayload,
	}

	payloadBytes, err := json.Marshal(gossipMsg)
	if err != nil {
		logger.Error("Failed to marshal gossip message", "error", err)
		return
	}
	peerUrl := fmt.Sprintf("%s/gossip", peer)
	message := messenger.NewMessage("GOSSIP_SYN", payloadBytes, g.currentNodeUrl, peerUrl)
	if err := g.messenger.SendMessage(message); err != nil {
		logger.Error("Failed to send gossip SYN",
			"peer", peer,
			"error", err.Error(),
			"retry_in_next_round", true,
		)
	} else {
		logger.Info("Gossip SYN sent successfully",
			"peer", peer,
			"digest_count", len(synPayload.Digests),
		)
	}
}

func (g *Gossiper) buildDigests() []Digests {
	digests := make([]Digests, 0, len(g.peers)+1)

	// Add current node's digest
	digests = append(digests, Digests{
		NodeURL:   g.currentNodeUrl,
		Heartbeat: g.heartbeat,
	})

	// Add peers' digests
	for _, peer := range g.peers {
		digests = append(digests, Digests{
			NodeURL:   peer,
			Heartbeat: g.peerHeartbeats[peer],
		})
	}

	return digests
}

// logClusterHealth provides periodic cluster health metrics and insights
func (g *Gossiper) logClusterHealth() {
	g.mu.Lock()
	defer g.mu.Unlock()

	totalPeers := len(g.peers)
	if totalPeers == 0 {
		logger.Info("Cluster Health Summary",
			"status", "STANDALONE",
			"peers", 0,
			"mode", "single_node",
		)
		return
	}

	activePeers := 0
	stalePeers := 0
	deadPeers := 0
	avgLag := 0
	maxLag := 0
	minHeartbeat := g.heartbeat
	maxHeartbeat := 0

	for peer, heartbeat := range g.peerHeartbeats {
		lag := g.heartbeat - heartbeat

		if heartbeat == 0 {
			deadPeers++
		} else if lag > 10 {
			stalePeers++
		} else {
			activePeers++
		}

		if heartbeat > 0 {
			avgLag += lag
			if lag > maxLag {
				maxLag = lag
			}
			if heartbeat < minHeartbeat {
				minHeartbeat = heartbeat
			}
			if heartbeat > maxHeartbeat {
				maxHeartbeat = heartbeat
			}
		}

		// Log individual peer status if concerning
		if heartbeat == 0 {
			logger.Warn("Peer appears dead",
				"peer", peer,
				"last_seen", "never",
				"action", "monitoring",
			)
		} else if lag > 20 {
			logger.Warn("Peer significantly lagging",
				"peer", peer,
				"heartbeat", heartbeat,
				"lag", lag,
				"health", "degraded",
			)
		}
	}

	clusterHealth := "HEALTHY"
	if deadPeers > totalPeers/2 {
		clusterHealth = "CRITICAL"
	} else if stalePeers > 0 || deadPeers > 0 {
		clusterHealth = "DEGRADED"
	}

	if activePeers > 0 {
		avgLag = avgLag / activePeers
	}

	convergenceRate := float64(activePeers) / float64(totalPeers) * 100

	logger.Info("Cluster Health Summary",
		"status", clusterHealth,
		"node_heartbeat", g.heartbeat,
		"total_peers", totalPeers,
		"active_peers", activePeers,
		"stale_peers", stalePeers,
		"dead_peers", deadPeers,
		"convergence_rate", fmt.Sprintf("%.1f%%", convergenceRate),
		"avg_lag", avgLag,
		"max_lag", maxLag,
		"heartbeat_range", fmt.Sprintf("%d-%d", minHeartbeat, maxHeartbeat),
		"cluster_sync", func() string {
			if maxLag <= 3 {
				return "EXCELLENT"
			} else if maxLag <= 10 {
				return "GOOD"
			} else {
				return "POOR"
			}
		}(),
	)

	// Additional insights
	if totalPeers > 0 {
		if convergenceRate == 100 && maxLag <= 3 {
			logger.Info("Cluster Performance", "insight", "optimal_convergence", "recommendation", "none")
		} else if deadPeers > 0 {
			logger.Warn("Cluster Performance", "insight", "peer_connectivity_issues", "recommendation", "check_network_and_peer_health")
		} else if maxLag > 10 {
			logger.Warn("Cluster Performance", "insight", "high_gossip_lag", "recommendation", "investigate_slow_peers")
		}
	}
}

// GetGossipMetrics returns current gossip protocol metrics for monitoring/API
func (g *Gossiper) GetGossipMetrics() map[string]interface{} {
	g.mu.Lock()
	defer g.mu.Unlock()

	totalPeers := len(g.peers)
	if totalPeers == 0 {
		return map[string]interface{}{
			"status":         "standalone",
			"node_heartbeat": g.heartbeat,
			"total_peers":    0,
			"cluster_health": "N/A",
		}
	}

	activePeers := 0
	stalePeers := 0
	deadPeers := 0
	totalLag := 0
	maxLag := 0

	peerDetails := make([]map[string]interface{}, 0)

	for peer, heartbeat := range g.peerHeartbeats {
		lag := g.heartbeat - heartbeat
		status := "active"

		if heartbeat == 0 {
			deadPeers++
			status = "dead"
		} else if lag > 10 {
			stalePeers++
			status = "stale"
		} else {
			activePeers++
		}

		if heartbeat > 0 {
			totalLag += lag
			if lag > maxLag {
				maxLag = lag
			}
		}

		peerDetails = append(peerDetails, map[string]interface{}{
			"url":       peer,
			"heartbeat": heartbeat,
			"lag":       lag,
			"status":    status,
		})
	}

	avgLag := 0.0
	if activePeers > 0 {
		avgLag = float64(totalLag) / float64(activePeers)
	}

	clusterHealth := "healthy"
	if deadPeers > totalPeers/2 {
		clusterHealth = "critical"
	} else if stalePeers > 0 || deadPeers > 0 {
		clusterHealth = "degraded"
	}

	convergenceRate := float64(activePeers) / float64(totalPeers) * 100

	return map[string]interface{}{
		"node_heartbeat":   g.heartbeat,
		"cluster_health":   clusterHealth,
		"total_peers":      totalPeers,
		"active_peers":     activePeers,
		"stale_peers":      stalePeers,
		"dead_peers":       deadPeers,
		"convergence_rate": convergenceRate,
		"average_lag":      avgLag,
		"max_lag":          maxLag,
		"peer_details":     peerDetails,
		"timestamp":        time.Now().Unix(),
	}
}

func (g *Gossiper) handleSyn(payload SynPayload) AckPayload {
	g.mu.Lock()
	defer g.mu.Unlock()

	DigestsUpdate := make([]Digests, 0)
	DigestsRequest := make([]Digests, 0)
	updatesApplied := 0
	requestsGenerated := 0

	for _, digest := range payload.Digests {
		if digest.NodeURL == g.currentNodeUrl {
			// Ignore self
			continue
		}

		localHeartbeat, exists := g.peerHeartbeats[digest.NodeURL]
		
		// Add new peer if not already known
		if !exists {
			g.peers = append(g.peers, digest.NodeURL)
			logger.Info("New peer discovered",
				"peer", digest.NodeURL,
				"heartbeat", digest.Heartbeat,
				"total_peers", len(g.peers),
			)
		}
		
		if !exists || digest.Heartbeat > localHeartbeat {
			// Update our record if the received heartbeat is newer
			oldHeartbeat := g.peerHeartbeats[digest.NodeURL]
			g.peerHeartbeats[digest.NodeURL] = digest.Heartbeat
			DigestsUpdate = append(DigestsUpdate, digest)
			updatesApplied++

			logger.Info("Heartbeat updated",
				"peer", digest.NodeURL,
				"old_heartbeat", oldHeartbeat,
				"new_heartbeat", digest.Heartbeat,
				"lag_reduced", digest.Heartbeat-oldHeartbeat,
			)
		} else if digest.Heartbeat < localHeartbeat {
			// Request update from peer if our heartbeat is newer
			DigestsRequest = append(DigestsRequest, Digests{
				NodeURL:   digest.NodeURL,
				Heartbeat: localHeartbeat,
			})
			requestsGenerated++

			logger.Info("Requesting update",
				"peer", digest.NodeURL,
				"peer_heartbeat", digest.Heartbeat,
				"my_heartbeat", localHeartbeat,
				"peer_lag", localHeartbeat-digest.Heartbeat,
			)
		}
	}

	logger.Info("SYN processing complete",
		"digests_received", len(payload.Digests),
		"updates_applied", updatesApplied,
		"requests_generated", requestsGenerated,
		"no_change", len(payload.Digests)-updatesApplied-requestsGenerated-1, // -1 for self
	)

	return AckPayload{
		DigestsUpdate:  DigestsUpdate,
		DigestsRequest: DigestsRequest,
	}
}

func (g *Gossiper) handleAck2(payload Ack2Payload) {
	g.mu.Lock()
	defer g.mu.Unlock()

	updatesApplied := 0
	for _, digest := range payload.DigestsUpdate {
		if digest.NodeURL != g.currentNodeUrl {
			oldHeartbeat := g.peerHeartbeats[digest.NodeURL]
			g.peerHeartbeats[digest.NodeURL] = digest.Heartbeat
			updatesApplied++

			logger.Info("ACK2 update applied",
				"peer", digest.NodeURL,
				"old_heartbeat", oldHeartbeat,
				"new_heartbeat", digest.Heartbeat,
				"improvement", digest.Heartbeat-oldHeartbeat,
			)
		}
	}

	if updatesApplied > 0 {
		logger.Info("ACK2 processing complete",
			"total_updates", len(payload.DigestsUpdate),
			"updates_applied", updatesApplied,
			"gossip_convergence", "improved",
		)
	}
}
