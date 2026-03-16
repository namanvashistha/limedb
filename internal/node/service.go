package node

import (
	"encoding/json"
	"fmt"
	"limedb/internal/logger"
	"limedb/internal/ring"
	"limedb/internal/store"
	"time"

	"github.com/valyala/fasthttp"
)

// GetResponse represents the JSON response for a GET request.
type GetResponse struct {
	Value   string `json:"value"`
	NodeUrl string `json:"nodeUrl"`
}

// SetRequest represents the JSON body for a SET request.
type SetRequest struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// WriteResponse tracks the result of a write to a single replica
type WriteResponse struct {
	NodeUrl string
	Success bool
	Error   string
}

// NodeService manages the node's operations and routing.
type NodeService struct {
	currentNodeUrl    string
	ring              *ring.ConsistentHashRing
	store             store.Backend
	client            *fasthttp.Client
	replicationFactor int
}

// NewService creates a new NodeService with an injected store backend.
func NewService(nodeUrl string, virtualNodes int, peers []string, s store.Backend, replicationFactor int) *NodeService {
	currentNodeUrl := nodeUrl

	r := ring.New(virtualNodes)
	// IMPORTANT: Include current node itself in the ring so single-node clusters work
	r.AddNode(currentNodeUrl)
	for _, peer := range peers {
		if peer == currentNodeUrl { // Skip duplicate self reference if present in peer list
			continue
		}
		r.AddNode(peer)
	}

	return &NodeService{
		currentNodeUrl:    currentNodeUrl,
		ring:              r,
		store:             s,
		client:            &fasthttp.Client{MaxConnsPerHost: 1000},
		replicationFactor: replicationFactor,
	}
}

// GetRing returns the underlying hash ring.
func (s *NodeService) GetRing() *ring.ConsistentHashRing {
	return s.ring
}

// GetNodeUrl returns the current node URL.
func (s *NodeService) GetNodeUrl() string {
	return s.currentNodeUrl
}

// GetPeers returns the list of peers in the ring.
func (s *NodeService) GetPeers() []string {
	return s.ring.GetNodes()
}

// GetCurrentNodeUrl returns the current node's URL.
func (s *NodeService) GetCurrentNodeUrl() string {
	return s.currentNodeUrl
}

// GetStore returns the underlying store backend.
func (s *NodeService) GetStore() store.Backend {
	return s.store
}

// HandleGet handles a GET request with ONE consistency (read from any available replica).
func (s *NodeService) HandleGet(key string) (*GetResponse, error) {
	replicas := s.ring.GetReplicas(key, s.replicationFactor)

	// Try each replica in order
	for _, replica := range replicas {
		if replica == s.currentNodeUrl {
			// Local read
			val, ok := s.store.Get(key)
			if ok {
				logger.Info("Get ONE (local)", "key", key)
				return &GetResponse{Value: val, NodeUrl: s.currentNodeUrl}, nil
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
// Writes to all replicas in parallel and returns success once QUORUM acks received.
func (s *NodeService) HandleSet(key, value string) error {
	replicas := s.ring.GetReplicas(key, s.replicationFactor)

	// Calculate QUORUM: ceil(RF/2) + 1
	quorum := (s.replicationFactor / 2) + 1

	// Write to all replicas concurrently
	responsesChan := make(chan WriteResponse, len(replicas))

	for _, replica := range replicas {
		go func(nodeUrl string) {
			if nodeUrl == s.currentNodeUrl {
				// Local write
				s.store.Set(key, value)
				responsesChan <- WriteResponse{NodeUrl: nodeUrl, Success: true}
			} else {
				// Remote write via HTTP
				err := s.forwardSet(nodeUrl, key, value)
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
			logger.Info("Write QUORUM reached", "key", key, "rf", s.replicationFactor, "quorum", quorum, "acked", successCount)
			// Note: Remaining replicas will continue writing asynchronously in background
			return nil
		}
	}

	// All replicas responded but QUORUM not reached
	if successCount >= quorum {
		return nil
	}

	errMsg := fmt.Sprintf("write failed: only %d/%d replicas acked (quorum=%d)", successCount, len(replicas), quorum)
	if len(collectedErrors) > 0 {
		errMsg += " - errors: " + fmt.Sprint(collectedErrors)
	}
	logger.Warn("Write QUORUM failed", "key", key, "acked", successCount, "required", quorum, "rf", s.replicationFactor)
	return fmt.Errorf(errMsg)
}

// HandleDelete handles a DELETE request with QUORUM consistency.
func (s *NodeService) HandleDelete(key string) (bool, error) {
	replicas := s.ring.GetReplicas(key, s.replicationFactor)

	// Calculate QUORUM: ceil(RF/2) + 1
	quorum := (s.replicationFactor / 2) + 1

	// Delete from all replicas concurrently
	responsesChan := make(chan WriteResponse, len(replicas))

	for _, replica := range replicas {
		go func(nodeUrl string) {
			if nodeUrl == s.currentNodeUrl {
				// Local delete
				success := s.store.Delete(key)
				responsesChan <- WriteResponse{NodeUrl: nodeUrl, Success: success}
			} else {
				// Remote delete via HTTP
				success, err := s.forwardDelete(nodeUrl, key)
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

	// Collect responses until QUORUM reached
	successCount := 0

	for i := 0; i < len(replicas); i++ {
		resp := <-responsesChan
		if resp.Success {
			successCount++
		}

		// Early return once QUORUM reached
		if successCount >= quorum {
			logger.Info("Delete QUORUM reached", "key", key, "quorum", quorum, "acked", successCount)
			return true, nil
		}
	}

	if successCount >= quorum {
		return true, nil
	}

	return false, fmt.Errorf("delete failed: only %d/%d replicas acked (quorum=%d)", successCount, len(replicas), quorum)
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

// forwardSet forwards a SET request to a peer.
func (s *NodeService) forwardSet(targetUrl, key, value string) error {
	url := fmt.Sprintf("%s/api/v1/set", targetUrl)

	req := fasthttp.AcquireRequest()
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(resp)

	req.SetRequestURI(url)
	req.Header.SetMethod(fasthttp.MethodPost)
	req.Header.SetContentType("application/json")

	body := SetRequest{Key: key, Value: value}
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

// forwardDelete forwards a DELETE request to a peer.
func (s *NodeService) forwardDelete(targetUrl, key string) (bool, error) {
	url := fmt.Sprintf("%s/api/v1/del/%s", targetUrl, key)

	req := fasthttp.AcquireRequest()
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(resp)

	req.SetRequestURI(url)
	req.Header.SetMethod(fasthttp.MethodDelete)

	if err := s.client.DoTimeout(req, resp, 2*time.Second); err != nil {
		return false, err
	}

	if resp.StatusCode() != fasthttp.StatusOK {
		return false, fmt.Errorf("peer returned status %d", resp.StatusCode())
	}

	// Assuming peer returns "1" for true, "0" for false like Java implementation
	return string(resp.Body()) == "1", nil
}
