package node

import (
	"encoding/json"
	"fmt"
	"limedb/internal/logger"
	"limedb/internal/placement"
	"limedb/internal/store"
	"time"

	"github.com/valyala/fasthttp"
)

// GetResponse represents the JSON response for a GET request.
type GetResponse struct {
	Value           string `json:"value"`
	TimestampMicros int64  `json:"timestamp_micros"`
	NodeUrl         string `json:"nodeUrl"`
}

// SetRequest represents the JSON body for a SET request. The same struct is
// the body of /internal/replicate, where the coordinator-assigned timestamp
// and tombstone flag travel with the value (clients never set those fields).
type SetRequest struct {
	Key             string `json:"key"`
	Value           string `json:"value"`
	TimestampMicros int64  `json:"timestamp_micros,omitempty"`
	Tombstone       bool   `json:"tombstone,omitempty"`
}

// WriteResponse tracks the result of a write to a single replica
type WriteResponse struct {
	NodeUrl string
	Success bool
	Error   string
}

// ReplicaInfo describes the replication state for a single key.
type ReplicaInfo struct {
	Key               string        `json:"key"`
	ReplicationFactor int           `json:"replication_factor"`
	Quorum            int           `json:"quorum"`
	Replicas          []ReplicaNode `json:"replicas"`
}

// ReplicaNode describes one node's responsibility for a key.
type ReplicaNode struct {
	NodeUrl   string `json:"node_url"`
	IsPrimary bool   `json:"is_primary"` // First node in the ring walk
	IsLocal   bool   `json:"is_local"`   // This coordinator node
	HasValue  bool   `json:"has_value"`  // Whether the node actually holds the key
}

// NodeService manages the node's operations and routing.
type NodeService struct {
	currentNodeUrl    string
	placement         *placement.Manager
	store             store.Backend
	client            *fasthttp.Client
	replicationFactor int
}

// NewService creates a new NodeService with an injected store backend.
func NewService(nodeUrl string, placementManager *placement.Manager, s store.Backend, replicationFactor int) *NodeService {
	return &NodeService{
		currentNodeUrl:    nodeUrl,
		placement:         placementManager,
		store:             s,
		client:            &fasthttp.Client{MaxConnsPerHost: 1000},
		replicationFactor: replicationFactor,
	}
}

// GetPlacement returns the placement manager used for routing.
func (s *NodeService) GetPlacement() *placement.Manager {
	return s.placement
}

// GetNodeUrl returns the current node URL.
func (s *NodeService) GetNodeUrl() string {
	return s.currentNodeUrl
}

// GetPeers returns the list of peers in the ring.
func (s *NodeService) GetPeers() []string {
	snapshot := s.placement.Snapshot()
	if snapshot == nil {
		return nil
	}
	peers := make([]string, 0, len(snapshot.Members))
	for _, member := range snapshot.Members {
		peers = append(peers, member.NodeURL)
	}
	return peers
}

// GetCurrentNodeUrl returns the current node's URL.
func (s *NodeService) GetCurrentNodeUrl() string {
	return s.currentNodeUrl
}

// GetStore returns the underlying store backend.
func (s *NodeService) GetStore() store.Backend {
	return s.store
}

// GetReplicationFactor returns the configured replication factor.
func (s *NodeService) GetReplicationFactor() int {
	return s.replicationFactor
}

// GetReplicaInfo returns the replica assignment and live status for a key.
// It probes each replica to check if the key actually exists there.
func (s *NodeService) GetReplicaInfo(key string) *ReplicaInfo {
	replicaSet := s.placement.ResolveReplicas(key)
	replicas := make([]string, 0, len(replicaSet.Replicas))
	for _, replica := range replicaSet.Replicas {
		replicas = append(replicas, replica.NodeURL)
	}
	quorum := (s.replicationFactor / 2) + 1

	type probeResult struct {
		idx      int
		hasValue bool
	}
	probeChan := make(chan probeResult, len(replicas))

	for i, replica := range replicas {
		go func(idx int, nodeUrl string) {
			has := false
			if nodeUrl == s.currentNodeUrl {
				v, ok := s.store.Get(key)
				has = ok && !v.Tombstone
			} else {
				// HEAD-style: try to GET and check if it returns a value
				result, err := s.forwardGet(nodeUrl, key)
				has = err == nil && result != nil
			}
			probeChan <- probeResult{idx: idx, hasValue: has}
		}(i, replica)
	}

	results := make([]bool, len(replicas))
	for range replicas {
		r := <-probeChan
		results[r.idx] = r.hasValue
	}

	nodes := make([]ReplicaNode, len(replicas))
	for i, nodeUrl := range replicas {
		nodes[i] = ReplicaNode{
			NodeUrl:   nodeUrl,
			IsPrimary: i == 0,
			IsLocal:   nodeUrl == s.currentNodeUrl,
			HasValue:  results[i],
		}
	}

	return &ReplicaInfo{
		Key:               key,
		ReplicationFactor: s.replicationFactor,
		Quorum:            quorum,
		Replicas:          nodes,
	}
}

// HandleGet handles a GET request with ONE consistency (read from any available replica).
func (s *NodeService) HandleGet(key string) (*GetResponse, error) {
	replicaSet := s.placement.ResolveReplicas(key)

	// Try each replica in order
	for _, replicaTarget := range replicaSet.Replicas {
		replica := replicaTarget.NodeURL
		if replica == s.currentNodeUrl {
			// Local read
			v, ok := s.store.Get(key)
			if ok {
				if v.Tombstone {
					// The key was deleted; the tombstone is authoritative here.
					return nil, fmt.Errorf("key not found on any replica")
				}
				logger.Info("Get ONE (local)", "key", key)
				return &GetResponse{Value: v.Value, TimestampMicros: v.TimestampMicros, NodeUrl: s.currentNodeUrl}, nil
			}
		} else {
			// Remote read
			result, err := s.forwardGet(replica, key)
			if err == nil {
				logger.Info("Get ONE (remote)", "key", key, "from", replica)
				return result, nil
			}
			// Try next replica if this one fails
		}
	}

	return nil, fmt.Errorf("key not found on any replica")
}

// HandleSet handles a SET request with QUORUM consistency.
// The coordinator assigns the LWW timestamp exactly once; every replica
// (including the local store) applies the same versioned value.
func (s *NodeService) HandleSet(key, value string) error {
	return s.writeQuorum(key, store.VersionedValue{
		Value:           value,
		TimestampMicros: time.Now().UnixMicro(),
	})
}

// HandleDelete handles a DELETE request with QUORUM consistency.
// A delete is a replicated tombstone write with a coordinator timestamp, so
// it wins over any older value that resurfaces from a lagging replica.
func (s *NodeService) HandleDelete(key string) (bool, error) {
	err := s.writeQuorum(key, store.VersionedValue{
		TimestampMicros: time.Now().UnixMicro(),
		Tombstone:       true,
	})
	return err == nil, err
}

// writeQuorum writes v to all replicas in parallel and returns once QUORUM acks.
func (s *NodeService) writeQuorum(key string, v store.VersionedValue) error {
	replicaSet := s.placement.ResolveReplicas(key)
	replicas := make([]string, 0, len(replicaSet.Replicas))
	for _, replica := range replicaSet.Replicas {
		replicas = append(replicas, replica.NodeURL)
	}

	// Calculate QUORUM: ceil(RF/2) + 1
	quorum := (s.replicationFactor / 2) + 1

	// Write to all replicas concurrently
	responsesChan := make(chan WriteResponse, len(replicas))

	for _, replica := range replicas {
		go func(nodeUrl string) {
			if nodeUrl == s.currentNodeUrl {
				// Local write
				s.store.Put(key, v)
				responsesChan <- WriteResponse{NodeUrl: nodeUrl, Success: true}
			} else {
				// Remote write via HTTP
				err := s.forwardSet(nodeUrl, key, v)
				success := err == nil
				errMsg := ""
				if err != nil {
					errMsg = err.Error()
				}
				responsesChan <- WriteResponse{
					NodeUrl: nodeUrl,
					Success: success,
					Error:   errMsg,
				}
			}
		}(replica)
	}

	// Collect responses until QUORUM reached or all responded
	successCount := 0
	var collectedErrors []string

	for i := 0; i < len(replicas); i++ {
		resp := <-responsesChan
		if resp.Success {
			successCount++
		} else {
			collectedErrors = append(collectedErrors, fmt.Sprintf("%s: %s", resp.NodeUrl, resp.Error))
		}

		// Early return once QUORUM reached (durability guarantee)
		if successCount >= quorum {
			logger.Info("Write QUORUM reached", "key", key, "rf", s.replicationFactor, "quorum", quorum, "acked", successCount, "tombstone", v.Tombstone)
			// Note: Remaining replicas will continue writing asynchronously in background
			return nil
		}
	}

	errMsg := fmt.Sprintf("write failed: only %d/%d replicas acked (quorum=%d)", successCount, len(replicas), quorum)
	if len(collectedErrors) > 0 {
		errMsg += " - errors: " + fmt.Sprint(collectedErrors)
	}
	logger.Warn("Write QUORUM failed", "key", key, "acked", successCount, "required", quorum, "rf", s.replicationFactor)
	return fmt.Errorf("%s", errMsg)
}

// forwardGet forwards a GET request to a peer.
func (s *NodeService) forwardGet(targetUrl, key string) (*GetResponse, error) {
	url := fmt.Sprintf("%s/api/v1/get/%s", targetUrl, key)

	req := fasthttp.AcquireRequest()
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(resp)

	req.SetRequestURI(url)
	req.Header.SetMethod(fasthttp.MethodGet)

	if err := s.client.DoTimeout(req, resp, 2*time.Second); err != nil {
		return nil, err
	}

	if resp.StatusCode() != fasthttp.StatusOK {
		return nil, fmt.Errorf("peer returned status %d", resp.StatusCode())
	}

	var result GetResponse
	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// forwardSet forwards a versioned write (value or tombstone) to a peer's
// internal replica endpoint (local write only, no re-replication).
func (s *NodeService) forwardSet(targetUrl, key string, v store.VersionedValue) error {
	url := fmt.Sprintf("%s/internal/replicate", targetUrl)

	req := fasthttp.AcquireRequest()
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(resp)

	req.SetRequestURI(url)
	req.Header.SetMethod(fasthttp.MethodPost)
	req.Header.SetContentType("application/json")

	body := SetRequest{Key: key, Value: v.Value, TimestampMicros: v.TimestampMicros, Tombstone: v.Tombstone}
	bodyBytes, _ := json.Marshal(body)
	req.SetBody(bodyBytes)

	if err := s.client.DoTimeout(req, resp, 2*time.Second); err != nil {
		return err
	}

	if resp.StatusCode() != fasthttp.StatusOK {
		return fmt.Errorf("peer returned status %d", resp.StatusCode())
	}
	return nil
}

// HandleReplicaWrite applies a versioned write directly to local storage
// without triggering replication. This is called by the coordinator node when
// forwarding to replicas — prevents recursive replication. LWW in the store
// makes it safe to apply late or duplicate deliveries.
func (s *NodeService) HandleReplicaWrite(key string, v store.VersionedValue) {
	applied := s.store.Put(key, v)
	logger.Info("Replica write (local)", "key", key, "applied", applied, "tombstone", v.Tombstone)
}
