package gossiper

import (
	"encoding/json"
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
	peerHeartbeats := make(map[string]int)
	for _, peer := range peers {
		peerHeartbeats[peer] = 0
	}

	return &Gossiper{
		currentNodeUrl: currentNodeUrl,
		peers:          peers,
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
		logger.Error("Invalid gossip message JSON", "error", err.Error())
		return map[string]interface{}{"error": "Invalid JSON"}
	}

	logger.Info("Received gossip message", "type", gossipMsg.Type)

	switch gossipMsg.Type {
	case "GOSSIP_SYN":
		var synPayload SynPayload
		payloadBytes, _ := json.Marshal(gossipMsg.Payload)
		if err := json.Unmarshal(payloadBytes, &synPayload); err != nil {
			logger.Error("Invalid SYN payload", "error", err.Error())
			return map[string]interface{}{"error": "Invalid SYN payload"}
		}

		// Handle SYN and return ACK response
		ackPayload := g.handleSyn(synPayload)
		return map[string]interface{}{
			"digestsUpdate":  ackPayload.DigestsUpdate,
			"digestsRequest": ackPayload.DigestsRequest,
		}

	case "GOSSIP_ACK2":
		var ack2Payload Ack2Payload
		payloadBytes, _ := json.Marshal(gossipMsg.Payload)
		if err := json.Unmarshal(payloadBytes, &ack2Payload); err != nil {
			logger.Error("Invalid ACK2 payload", "error", err.Error())
			return map[string]interface{}{"error": "Invalid ACK2 payload"}
		}

		// Handle ACK2
		g.handleAck2(ack2Payload)
		logger.Info("Processed ACK2", "updates_count", len(ack2Payload.DigestsUpdate))
		return map[string]interface{}{"status": "OK"}

	default:
		logger.Warn("Unknown gossip message type", "type", gossipMsg.Type)
		return map[string]interface{}{"error": "Unknown message type"}
	}
}
func (g *Gossiper) StartGossiping() {
	ticker := time.NewTicker(5 * time.Second)
	go func() {
		for range ticker.C {
			g.gossipRound()
		}
	}()
}

func (g *Gossiper) gossipRound() {
	g.mu.Lock()
	g.heartbeat++ // ← increase here ONCE per round
	g.mu.Unlock()
	if len(g.peers) == 0 {
		logger.Error("No peers configured for gossiping. Gossiper will not start.")
		return
	}

	peer := g.peers[rand.Intn(len(g.peers))]
	logger.Info("Starting gossip with peer", "peer", peer)

	SynPayload := SynPayload{
		Digests: g.buildDigests(),
	}
	payloadBytes, err := json.Marshal(SynPayload)
	if err != nil {
		logger.Error("Failed to marshal SynPayload", "error", err)
		return
	}
	message := messenger.NewMessage("GOSSIP_SYN", payloadBytes, g.currentNodeUrl, peer)
	g.messenger.SendMessage(message)
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

func (g *Gossiper) handleSyn(payload SynPayload) AckPayload {
	g.mu.Lock()
	defer g.mu.Unlock()

	DigestsUpdate := make([]Digests, 0)
	DigestsRequest := make([]Digests, 0)

	for _, digest := range payload.Digests {
		if digest.NodeURL == g.currentNodeUrl {
			// Ignore self
			continue
		}

		localHeartbeat, exists := g.peerHeartbeats[digest.NodeURL]
		if !exists || digest.Heartbeat > localHeartbeat {
			// Update our record if the received heartbeat is newer
			g.peerHeartbeats[digest.NodeURL] = digest.Heartbeat
			DigestsUpdate = append(DigestsUpdate, digest)
		} else if digest.Heartbeat < localHeartbeat {
			// Request update from peer if our heartbeat is newer
			DigestsRequest = append(DigestsRequest, Digests{
				NodeURL:   digest.NodeURL,
				Heartbeat: localHeartbeat,
			})
		}
	}
	return AckPayload{
		DigestsUpdate:  DigestsUpdate,
		DigestsRequest: DigestsRequest,
	}
}

func (g *Gossiper) handleAck2(payload Ack2Payload) {
	g.mu.Lock()
	defer g.mu.Unlock()

	for _, digest := range payload.DigestsUpdate {
		if digest.NodeURL != g.currentNodeUrl {
			g.peerHeartbeats[digest.NodeURL] = digest.Heartbeat
			logger.Info("Updated peer heartbeat via ACK2", "peer", digest.NodeURL, "heartbeat", digest.Heartbeat)
		}
	}
}
