package node

import (
	"encoding/json"
	"fmt"
	"limedb-go/internal/ring"
	"time"

	"github.com/valyala/fasthttp"
)

// GetResponse represents the JSON response for a GET request.
type GetResponse struct {
	Value  string `json:"value"`
	NodeID int    `json:"nodeId"`
}

// SetRequest represents the JSON body for a SET request.
type SetRequest struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// NodeService manages the node's operations and routing.
type NodeService struct {
	nodeID         int
	currentNodeUrl string
	ring           *ring.ConsistentHashRing
	store          *Store
	client         *fasthttp.Client
}

// New creates a new NodeService.
func New(nodeID int, port int, virtualNodes int, peers []string) *NodeService {
	currentNodeUrl := fmt.Sprintf("http://localhost:%d", port)
	
	r := ring.New(virtualNodes)
	for _, peer := range peers {
		r.AddNode(peer)
	}

	return &NodeService{
		nodeID:         nodeID,
		currentNodeUrl: currentNodeUrl,
		ring:           r,
		store:          NewStore(),
		client:         &fasthttp.Client{MaxConnsPerHost: 1000},
	}
}

// GetRing returns the underlying hash ring.
func (s *NodeService) GetRing() *ring.ConsistentHashRing {
	return s.ring
}

// GetNodeID returns the current node ID.
func (s *NodeService) GetNodeID() int {
	return s.nodeID
}

// GetPeers returns the list of peers in the ring.
func (s *NodeService) GetPeers() []string {
	return s.ring.GetNodes()
}

// GetCurrentNodeUrl returns the current node's URL.
func (s *NodeService) GetCurrentNodeUrl() string {
	return s.currentNodeUrl
}

// HandleGet handles a GET request, routing if necessary.
func (s *NodeService) HandleGet(key string) (*GetResponse, error) {
	targetUrl := s.ring.GetNode(key)
	if targetUrl == s.currentNodeUrl {
		val, ok := s.store.Get(key)
		if !ok {
			return nil, fmt.Errorf("key not found")
		}
		return &GetResponse{Value: val, NodeID: s.nodeID}, nil
	}
	return s.forwardGet(targetUrl, key)
}

// HandleSet handles a SET request, routing if necessary.
func (s *NodeService) HandleSet(key, value string) error {
	targetUrl := s.ring.GetNode(key)
	if targetUrl == s.currentNodeUrl {
		s.store.Set(key, value)
		return nil
	}
	return s.forwardSet(targetUrl, key, value)
}

// HandleDelete handles a DELETE request, routing if necessary.
func (s *NodeService) HandleDelete(key string) (bool, error) {
	targetUrl := s.ring.GetNode(key)
	if targetUrl == s.currentNodeUrl {
		return s.store.Delete(key), nil
	}
	return s.forwardDelete(targetUrl, key)
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
